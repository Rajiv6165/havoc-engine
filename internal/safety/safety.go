package safety

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"havoc-engine/internal/events"
	"havoc-engine/internal/experiments"
	"havoc-engine/internal/metrics"
)

type contextKey string

const (
	ExpIDKey   contextKey = "havoc_exp_id"
	ServiceKey contextKey = "havoc_service_name"
)

// WithExperimentContext attaches experiment ID and service name to context.
func WithExperimentContext(ctx context.Context, expID, service string) context.Context {
	ctx = context.WithValue(ctx, ExpIDKey, expID)
	ctx = context.WithValue(ctx, ServiceKey, service)
	return ctx
}

func getExpIDAndService(ctx context.Context, fallbackService string) (string, string) {
	expID := ""
	if v, ok := ctx.Value(ExpIDKey).(string); ok {
		expID = v
	}
	service := fallbackService
	if v, ok := ctx.Value(ServiceKey).(string); ok && v != "" {
		service = v
	}
	return expID, service
}

func extractServiceName(selector, podName string) string {
	if selector != "" {
		parts := strings.Split(selector, ",")
		for _, part := range parts {
			kv := strings.Split(strings.TrimSpace(part), "=")
			if len(kv) == 2 {
				if kv[0] == "app" || kv[0] == "service" || kv[0] == "app.kubernetes.io/name" {
					return kv[1]
				}
			}
		}
		if idx := strings.Index(selector, "="); idx != -1 {
			return selector[idx+1:]
		}
		return selector
	}
	if podName != "" {
		parts := strings.Split(podName, "-")
		if len(parts) >= 3 {
			return strings.Join(parts[:len(parts)-2], "-")
		}
		return podName
	}
	return "unknown"
}

// MetricsChecker defines an interface for fetching real-time metrics (e.g. error rate).
type MetricsChecker interface {
	GetErrorRate(ctx context.Context, namespace string) (float64, error)
}

// MetricsCheckFunc type adapter to convert a plain function into a MetricsChecker.
type MetricsCheckFunc func(ctx context.Context, namespace string) (float64, error)

func (f MetricsCheckFunc) GetErrorRate(ctx context.Context, namespace string) (float64, error) {
	return f(ctx, namespace)
}

// SafetyRunner wraps chaos experiment execution with blast radius limits, auto-abort error checks, and dry-run mode.
type SafetyRunner struct {
	client                  kubernetes.Interface
	maxBlastRadiusPercent   float64
	abortOnErrorRatePercent float64
	dryRun                  bool
	metricsChecker          MetricsChecker
	publisher               events.EventPublisher
}

// NewSafetyRunner initializes a new SafetyRunner.
func NewSafetyRunner(client kubernetes.Interface, maxBlastRadius, abortThreshold float64, dryRun bool, checker MetricsChecker) *SafetyRunner {
	if maxBlastRadius <= 0 {
		maxBlastRadius = 30.0
	}
	if abortThreshold <= 0 {
		abortThreshold = 5.0
	}
	return &SafetyRunner{
		client:                  client,
		maxBlastRadiusPercent:   maxBlastRadius,
		abortOnErrorRatePercent: abortThreshold,
		dryRun:                  dryRun,
		metricsChecker:          checker,
	}
}

// SetPublisher assigns an event publisher to emit lifecycle events.
func (r *SafetyRunner) SetPublisher(pub events.EventPublisher) {
	r.publisher = pub
}

func (r *SafetyRunner) publish(event events.Event) {
	if r.publisher != nil {
		r.publisher.Publish(event)
	}
}

// CheckBlastRadiusBySelector checks if targeting targetCount pods out of total matching pods exceeds maxPercent.
func CheckBlastRadiusBySelector(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string, targetCount int, maxPercent float64) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if labelSelector == "" {
		return fmt.Errorf("labelSelector cannot be empty")
	}

	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
	}

	totalPods := len(pods.Items)
	if totalPods == 0 {
		return fmt.Errorf("no pods found matching selector %q in namespace %q", labelSelector, namespace)
	}

	affectedPercent := (float64(targetCount) / float64(totalPods)) * 100.0
	if affectedPercent > maxPercent {
		return fmt.Errorf("blast radius limit exceeded: affecting %d of %d pods (%.2f%%) exceeds maximum allowed blast radius of %.2f%%", targetCount, totalPods, affectedPercent, maxPercent)
	}

	return nil
}

// CheckBlastRadiusByPodName checks if targeting a specific pod exceeds maxPercent within its workload matching group.
func CheckBlastRadiusByPodName(ctx context.Context, client kubernetes.Interface, namespace, podName string, targetCount int, maxPercent float64) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if podName == "" {
		return fmt.Errorf("podName cannot be empty")
	}

	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod %s in namespace %s: %w", podName, namespace, err)
	}

	var selector string
	if len(pod.Labels) > 0 {
		if appVal, exists := pod.Labels["app"]; exists {
			selector = fmt.Sprintf("app=%s", appVal)
		} else {
			var pairs []string
			for k, v := range pod.Labels {
				pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
			}
			selector = strings.Join(pairs, ",")
		}
	}

	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
	}

	totalPods := len(pods.Items)
	if totalPods == 0 {
		totalPods = 1
	}

	affectedPercent := (float64(targetCount) / float64(totalPods)) * 100.0
	if affectedPercent > maxPercent {
		return fmt.Errorf("blast radius limit exceeded: affecting %d of %d pods (%.2f%%) exceeds maximum allowed blast radius of %.2f%%", targetCount, totalPods, affectedPercent, maxPercent)
	}

	return nil
}

// CheckAutoAbort evaluates whether the current error rate exceeds the threshold.
func CheckAutoAbort(ctx context.Context, checker MetricsChecker, namespace string, threshold float64) error {
	if checker == nil {
		return nil
	}
	errRate, err := checker.GetErrorRate(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to check metrics error rate: %w", err)
	}
	if errRate > threshold {
		return fmt.Errorf("auto-abort triggered: current error rate %.2f%% exceeds threshold of %.2f%%", errRate, threshold)
	}
	return nil
}

// ExecuteKillPod wraps experiments.KillPod with safety checks and dry-run logic.
func (r *SafetyRunner) ExecuteKillPod(ctx context.Context, namespace, labelSelector string) (string, *int, error) {
	expID, service := getExpIDAndService(ctx, extractServiceName(labelSelector, ""))
	if r.dryRun {
		fmt.Printf("[DRY-RUN] Would kill 1 pod matching selector %q in namespace %q\n", labelSelector, namespace)
		r.publish(events.Event{ExperimentID: expID, Type: events.EventTargetSelected, Level: "warn", Message: fmt.Sprintf("[%s] target_selected: targeting simulated pod in %s", expID, service), Service: service})
		r.publish(events.Event{ExperimentID: expID, Type: events.EventPodKilled, Level: "warn", Message: fmt.Sprintf("[%s] pod_killed: simulated kill of pod in %s", expID, service), Service: service})
		simRecovery := 1000
		r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovered, Level: "info", Message: fmt.Sprintf("[%s] recovered: service %q pod recovered in %dms (resolved)", expID, service, simRecovery), Service: service, RecoveryMs: &simRecovery})
		return "[DRY-RUN] simulated-pod", &simRecovery, nil
	}

	if err := CheckBlastRadiusBySelector(ctx, r.client, namespace, labelSelector, 1, r.maxBlastRadiusPercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("kill_pod", "blocked").Inc()
		r.publish(events.Event{ExperimentID: expID, Type: events.EventBlocked, Level: "error", Message: fmt.Sprintf("[%s] blocked: %v", expID, err), Service: service})
		return "", nil, err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("kill_pod", "aborted").Inc()
		r.publish(events.Event{ExperimentID: expID, Type: events.EventAborted, Level: "error", Message: fmt.Sprintf("[%s] aborted: %v", expID, err), Service: service})
		return "", nil, err
	}

	targetPod, err := experiments.SelectTargetPod(ctx, r.client, namespace, labelSelector)
	if err != nil {
		return "", nil, err
	}
	r.publish(events.Event{ExperimentID: expID, Type: events.EventTargetSelected, Level: "warn", Message: fmt.Sprintf("[%s] target_selected: targeting pod %q in service %q", expID, targetPod.Name, service), Service: service})

	killTime := time.Now()
	if err := experiments.DeletePod(ctx, r.client, namespace, targetPod.Name); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("kill_pod", "failed").Inc()
		return "", nil, err
	}
	deletedPod := targetPod.Name
	r.publish(events.Event{ExperimentID: expID, Type: events.EventPodKilled, Level: "warn", Message: fmt.Sprintf("[%s] pod_killed: deleted pod %q in service %q", expID, deletedPod, service), Service: service})

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("kill_pod", "aborted").Inc()
		r.publish(events.Event{ExperimentID: expID, Type: events.EventAborted, Level: "error", Message: fmt.Sprintf("[%s] aborted: auto-abort triggered post-execution: %v", expID, err), Service: service})
		return deletedPod, nil, fmt.Errorf("auto-abort triggered post-execution: %w", err)
	}

	metrics.ExperimentsTotal.WithLabelValues("kill_pod", "success").Inc()

	// Watch for pod recovery synchronously to capture recovery time for scoring
	fmt.Printf("Waiting for pod recovery in namespace %q with selector %q...\n", namespace, labelSelector)
	r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovering, Level: "info", Message: fmt.Sprintf("[%s] recovering: waiting for service %q pod recovery", expID, service), Service: service})
	recoveryTimeMs := watchPodRecovery(ctx, r.client, namespace, labelSelector, killTime)
	if recoveryTimeMs != nil {
		r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovered, Level: "info", Message: fmt.Sprintf("[%s] recovered: service %q pod recovered in %dms (resolved)", expID, service, *recoveryTimeMs), Service: service, RecoveryMs: recoveryTimeMs})
	}

	return deletedPod, recoveryTimeMs, nil
}

// ExecuteInjectLatency wraps experiments.InjectLatency with safety checks, auto-abort, rollback logic, and recovery tracking.
func (r *SafetyRunner) ExecuteInjectLatency(ctx context.Context, namespace, podName string, delayMs int) (*int, error) {
	expID, service := getExpIDAndService(ctx, extractServiceName("", podName))
	if r.dryRun {
		fmt.Printf("[DRY-RUN] Would inject %dms latency into pod %q in namespace %q\n", delayMs, podName, namespace)
		r.publish(events.Event{ExperimentID: expID, Type: events.EventTargetSelected, Level: "warn", Message: fmt.Sprintf("[%s] target_selected: targeting pod %q in service %q", expID, podName, service), Service: service})
		simRecovery := 1000
		r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovered, Level: "info", Message: fmt.Sprintf("[%s] recovered: simulated latency injection complete (resolved)", expID), Service: service, RecoveryMs: &simRecovery})
		return &simRecovery, nil
	}

	if err := CheckBlastRadiusByPodName(ctx, r.client, namespace, podName, 1, r.maxBlastRadiusPercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("inject_latency", "blocked").Inc()
		r.publish(events.Event{ExperimentID: expID, Type: events.EventBlocked, Level: "error", Message: fmt.Sprintf("[%s] blocked: %v", expID, err), Service: service})
		return nil, err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("inject_latency", "aborted").Inc()
		r.publish(events.Event{ExperimentID: expID, Type: events.EventAborted, Level: "error", Message: fmt.Sprintf("[%s] aborted: %v", expID, err), Service: service})
		return nil, err
	}

	r.publish(events.Event{ExperimentID: expID, Type: events.EventTargetSelected, Level: "warn", Message: fmt.Sprintf("[%s] target_selected: targeting pod %q in service %q for latency", expID, podName, service), Service: service})

	startTime := time.Now()
	if err := experiments.InjectLatency(ctx, r.client, namespace, podName, delayMs); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("inject_latency", "failed").Inc()
		return nil, err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		_ = experiments.RemoveLatency(ctx, r.client, namespace, podName)
		metrics.ExperimentsTotal.WithLabelValues("inject_latency", "aborted").Inc()
		r.publish(events.Event{ExperimentID: expID, Type: events.EventAborted, Level: "error", Message: fmt.Sprintf("[%s] aborted: auto-abort triggered and latency rolled back: %v", expID, err), Service: service})
		return nil, fmt.Errorf("auto-abort triggered: %w (in-progress experiment rolled back)", err)
	}

	metrics.ExperimentsTotal.WithLabelValues("inject_latency", "success").Inc()

	r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovering, Level: "info", Message: fmt.Sprintf("[%s] recovering: waiting for service %q pod %q to recover", expID, service, podName), Service: service})
	recoveryTimeMs := watchEphemeralContainerRecovery(ctx, r.client, namespace, podName, startTime)
	if recoveryTimeMs != nil {
		r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovered, Level: "info", Message: fmt.Sprintf("[%s] recovered: service %q pod %q recovered in %dms (resolved)", expID, service, podName, *recoveryTimeMs), Service: service, RecoveryMs: recoveryTimeMs})
	} else {
		r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovered, Level: "info", Message: fmt.Sprintf("[%s] recovered: latency experiment completed on %s (resolved)", expID, service), Service: service})
	}
	return recoveryTimeMs, nil
}

// ExecuteSpikeCPU wraps experiments.SpikeCPU with safety checks, auto-abort, rollback logic, and recovery tracking.
func (r *SafetyRunner) ExecuteSpikeCPU(ctx context.Context, namespace, podName string, durationSec int) (*int, error) {
	expID, service := getExpIDAndService(ctx, extractServiceName("", podName))
	if r.dryRun {
		fmt.Printf("[DRY-RUN] Would spike CPU (%ds) in pod %q in namespace %q\n", durationSec, podName, namespace)
		r.publish(events.Event{ExperimentID: expID, Type: events.EventTargetSelected, Level: "warn", Message: fmt.Sprintf("[%s] target_selected: targeting pod %q in service %q", expID, podName, service), Service: service})
		simRecovery := 1000
		r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovered, Level: "info", Message: fmt.Sprintf("[%s] recovered: simulated CPU spike complete (resolved)", expID), Service: service, RecoveryMs: &simRecovery})
		return &simRecovery, nil
	}

	if err := CheckBlastRadiusByPodName(ctx, r.client, namespace, podName, 1, r.maxBlastRadiusPercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("spike_cpu", "blocked").Inc()
		r.publish(events.Event{ExperimentID: expID, Type: events.EventBlocked, Level: "error", Message: fmt.Sprintf("[%s] blocked: %v", expID, err), Service: service})
		return nil, err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("spike_cpu", "aborted").Inc()
		r.publish(events.Event{ExperimentID: expID, Type: events.EventAborted, Level: "error", Message: fmt.Sprintf("[%s] aborted: %v", expID, err), Service: service})
		return nil, err
	}

	r.publish(events.Event{ExperimentID: expID, Type: events.EventTargetSelected, Level: "warn", Message: fmt.Sprintf("[%s] target_selected: targeting pod %q in service %q for CPU spike", expID, podName, service), Service: service})

	startTime := time.Now()
	if err := experiments.SpikeCPU(ctx, r.client, namespace, podName, durationSec); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("spike_cpu", "failed").Inc()
		return nil, err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		_ = experiments.RemoveCPUStress(ctx, r.client, namespace, podName)
		metrics.ExperimentsTotal.WithLabelValues("spike_cpu", "aborted").Inc()
		r.publish(events.Event{ExperimentID: expID, Type: events.EventAborted, Level: "error", Message: fmt.Sprintf("[%s] aborted: auto-abort triggered and CPU stress rolled back: %v", expID, err), Service: service})
		return nil, fmt.Errorf("auto-abort triggered: %w (in-progress experiment rolled back)", err)
	}

	metrics.ExperimentsTotal.WithLabelValues("spike_cpu", "success").Inc()

	r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovering, Level: "info", Message: fmt.Sprintf("[%s] recovering: waiting for service %q pod %q to recover", expID, service, podName), Service: service})
	recoveryTimeMs := watchEphemeralContainerRecovery(ctx, r.client, namespace, podName, startTime)
	if recoveryTimeMs != nil {
		r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovered, Level: "info", Message: fmt.Sprintf("[%s] recovered: service %q pod %q recovered in %dms (resolved)", expID, service, podName, *recoveryTimeMs), Service: service, RecoveryMs: recoveryTimeMs})
	} else {
		r.publish(events.Event{ExperimentID: expID, Type: events.EventRecovered, Level: "info", Message: fmt.Sprintf("[%s] recovered: CPU spike completed on %s (resolved)", expID, service), Service: service})
	}
	return recoveryTimeMs, nil
}

func watchEphemeralContainerRecovery(ctx context.Context, client kubernetes.Interface, namespace, podName string, startTime time.Time) *int {
	if client == nil {
		return nil
	}

	watcher, err := client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("failed to watch ephemeral container recovery: %v\n", err)
		return nil
	}
	defer watcher.Stop()

	timeout := time.After(5 * time.Minute)

	checkPod := func(pod *corev1.Pod) *int {
		if pod == nil || pod.Name != podName {
			return nil
		}

		ephemeralDone := false
		if len(pod.Status.EphemeralContainerStatuses) > 0 {
			ephemeralDone = true
			for _, cs := range pod.Status.EphemeralContainerStatuses {
				if cs.State.Terminated == nil {
					ephemeralDone = false
					break
				}
			}
		} else if len(pod.Spec.EphemeralContainers) == 0 {
			ephemeralDone = true
		}

		podReady := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				podReady = true
				break
			}
		}

		if ephemeralDone && podReady {
			recoveryTimeSec := time.Since(startTime).Seconds()
			metrics.PodRecoverySeconds.Observe(recoveryTimeSec)
			ms := int(time.Since(startTime).Milliseconds())
			if ms <= 0 {
				ms = 1
			}
			return &ms
		}
		return nil
	}

	if pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{}); err == nil {
		if res := checkPod(pod); res != nil {
			return res
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timeout:
			fmt.Printf("timeout watching for pod %q recovery in namespace %q\n", podName, namespace)
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return nil
			}
			if event.Type == watch.Modified || event.Type == watch.Added {
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					continue
				}
				if res := checkPod(pod); res != nil {
					return res
				}
			}
		}
	}
}

func watchPodRecovery(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string, killTime time.Time) *int {
	watcher, err := client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		fmt.Printf("failed to watch pod recovery: %v\n", err)
		return nil
	}
	defer watcher.Stop()

	// Timeout to avoid leaking goroutines
	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timeout:
			fmt.Printf("timeout watching for pod recovery in namespace %q with selector %q\n", namespace, labelSelector)
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return nil
			}
			if event.Type == watch.Modified || event.Type == watch.Added {
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					continue
				}
				
				// Check if this pod was created after the kill event
				if pod.CreationTimestamp.Time.After(killTime) {
					// Check if pod is Ready
					ready := false
					for _, cond := range pod.Status.Conditions {
						if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
							ready = true
							break
						}
					}

					if ready {
						recoveryTimeSec := time.Since(killTime).Seconds()
						metrics.PodRecoverySeconds.Observe(recoveryTimeSec)
						recoveryTimeMs := int(time.Since(killTime).Milliseconds())
						return &recoveryTimeMs
					}
				}
			}
		}
	}
}
