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

	"havoc-engine/internal/experiments"
	"havoc-engine/internal/metrics"
)

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
	if r.dryRun {
		fmt.Printf("[DRY-RUN] Would kill 1 pod matching selector %q in namespace %q\n", labelSelector, namespace)
		simRecovery := 1000
		return "[DRY-RUN] simulated-pod", &simRecovery, nil
	}

	if err := CheckBlastRadiusBySelector(ctx, r.client, namespace, labelSelector, 1, r.maxBlastRadiusPercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("kill_pod", "blocked").Inc()
		return "", nil, err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("kill_pod", "aborted").Inc()
		return "", nil, err
	}

	killTime := time.Now()
	deletedPod, err := experiments.KillPod(ctx, r.client, namespace, labelSelector)
	if err != nil {
		metrics.ExperimentsTotal.WithLabelValues("kill_pod", "failed").Inc()
		return "", nil, err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("kill_pod", "aborted").Inc()
		return deletedPod, nil, fmt.Errorf("auto-abort triggered post-execution: %w", err)
	}

	metrics.ExperimentsTotal.WithLabelValues("kill_pod", "success").Inc()

	// Watch for pod recovery synchronously to capture recovery time for scoring
	fmt.Printf("Waiting for pod recovery in namespace %q with selector %q...\n", namespace, labelSelector)
	recoveryTimeMs := watchPodRecovery(ctx, r.client, namespace, labelSelector, killTime)

	return deletedPod, recoveryTimeMs, nil
}

// ExecuteInjectLatency wraps experiments.InjectLatency with safety checks, auto-abort, and rollback logic.
func (r *SafetyRunner) ExecuteInjectLatency(ctx context.Context, namespace, podName string, delayMs int) error {
	if r.dryRun {
		fmt.Printf("[DRY-RUN] Would inject %dms latency into pod %q in namespace %q\n", delayMs, podName, namespace)
		return nil
	}

	if err := CheckBlastRadiusByPodName(ctx, r.client, namespace, podName, 1, r.maxBlastRadiusPercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("inject_latency", "blocked").Inc()
		return err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("inject_latency", "aborted").Inc()
		return err
	}

	if err := experiments.InjectLatency(ctx, r.client, namespace, podName, delayMs); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("inject_latency", "failed").Inc()
		return err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		_ = experiments.RemoveLatency(ctx, r.client, namespace, podName)
		metrics.ExperimentsTotal.WithLabelValues("inject_latency", "aborted").Inc()
		return fmt.Errorf("auto-abort triggered: %w (in-progress experiment rolled back)", err)
	}

	metrics.ExperimentsTotal.WithLabelValues("inject_latency", "success").Inc()
	return nil
}

// ExecuteSpikeCPU wraps experiments.SpikeCPU with safety checks, auto-abort, and rollback logic.
func (r *SafetyRunner) ExecuteSpikeCPU(ctx context.Context, namespace, podName string, durationSec int) error {
	if r.dryRun {
		fmt.Printf("[DRY-RUN] Would spike CPU (%ds) in pod %q in namespace %q\n", durationSec, podName, namespace)
		return nil
	}

	if err := CheckBlastRadiusByPodName(ctx, r.client, namespace, podName, 1, r.maxBlastRadiusPercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("spike_cpu", "blocked").Inc()
		return err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("spike_cpu", "aborted").Inc()
		return err
	}

	if err := experiments.SpikeCPU(ctx, r.client, namespace, podName, durationSec); err != nil {
		metrics.ExperimentsTotal.WithLabelValues("spike_cpu", "failed").Inc()
		return err
	}

	if err := CheckAutoAbort(ctx, r.metricsChecker, namespace, r.abortOnErrorRatePercent); err != nil {
		_ = experiments.RemoveCPUStress(ctx, r.client, namespace, podName)
		metrics.ExperimentsTotal.WithLabelValues("spike_cpu", "aborted").Inc()
		return fmt.Errorf("auto-abort triggered: %w (in-progress experiment rolled back)", err)
	}

	metrics.ExperimentsTotal.WithLabelValues("spike_cpu", "success").Inc()
	return nil
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
