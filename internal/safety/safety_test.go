package safety

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"havoc-engine/internal/events"
	"havoc-engine/internal/metrics"
)

func TestSafety_BlastRadiusExceeded(t *testing.T) {
	// 2 matching pods -> killing 1 pod affects 50% of matching pods
	fakeClient := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod-1",
				Namespace: "default",
				Labels:    map[string]string{"app": "demo"},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod-2",
				Namespace: "default",
				Labels:    map[string]string{"app": "demo"},
			},
		},
	)

	ctx := context.Background()
	// Default blast radius threshold is 30%
	runner := NewSafetyRunner(fakeClient, 30.0, 5.0, false, nil)

	_, _, err := runner.ExecuteKillPod(context.Background(), "default", "app=demo")
	if err == nil {
		t.Fatalf("expected error due to blast radius limit, got nil")
	}

	if !strings.Contains(err.Error(), "blast radius limit exceeded") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Verify pods were NOT modified or deleted
	pods, listErr := fakeClient.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})
	if listErr != nil {
		t.Fatalf("failed to list pods: %v", listErr)
	}
	if len(pods.Items) != 2 {
		t.Errorf("expected 2 pods remaining, got %d", len(pods.Items))
	}
}

func TestSafety_BlastRadiusAllowed(t *testing.T) {
	// 4 matching pods -> killing 1 pod affects 25% <= 30%
	fakeClient := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "default", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-3", Namespace: "default", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-4", Namespace: "default", Labels: map[string]string{"app": "demo"}}},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	runner := NewSafetyRunner(fakeClient, 30.0, 5.0, false, nil)

	deletedPod, _, err := runner.ExecuteKillPod(ctx, "default", "app=demo")
	if err != nil {
		t.Fatalf("expected experiment to succeed within blast radius limit, got %v", err)
	}

	if deletedPod == "" {
		t.Errorf("expected a pod name to be returned")
	}

	pods, _ := fakeClient.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})
	if len(pods.Items) != 3 {
		t.Errorf("expected 3 pods remaining after successful kill, got %d", len(pods.Items))
	}
}

func TestSafety_AutoAbortMidRun(t *testing.T) {
	// 4 pods to satisfy blast radius limit
	fakeClient := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-pod-1", Namespace: "default", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-pod-2", Namespace: "default", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-pod-3", Namespace: "default", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-pod-4", Namespace: "default", Labels: map[string]string{"app": "demo"}}},
	)

	var callCount int32
	// Pre-check returns 1.0% error rate; post/mid-run check returns 8.0% (exceeding 5.0% threshold)
	mockChecker := MetricsCheckFunc(func(ctx context.Context, namespace string) (float64, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			return 1.0, nil
		}
		return 8.0, nil
	})

	ctx := context.Background()
	runner := NewSafetyRunner(fakeClient, 30.0, 5.0, false, mockChecker)

	_, err := runner.ExecuteInjectLatency(ctx, "default", "demo-pod-1", 500)
	if err == nil {
		t.Fatalf("expected auto-abort error, got nil")
	}

	if !strings.Contains(err.Error(), "auto-abort triggered") {
		t.Errorf("expected auto-abort error message, got: %v", err)
	}

	// Verify rollback took effect (ephemeral container removed)
	pod, getErr := fakeClient.CoreV1().Pods("default").Get(ctx, "demo-pod-1", metav1.GetOptions{})
	if getErr != nil {
		t.Fatalf("failed to get pod: %v", getErr)
	}

	for _, ec := range pod.Spec.EphemeralContainers {
		if strings.HasPrefix(ec.Name, "chaos-latency-") {
			t.Errorf("expected latency ephemeral container to be removed on rollback, found %s", ec.Name)
		}
	}
}

func TestSafety_DryRunMode(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod-1",
				Namespace: "default",
				Labels:    map[string]string{"app": "demo"},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod-2",
				Namespace: "default",
				Labels:    map[string]string{"app": "demo"},
			},
		},
	)

	ctx := context.Background()
	// Even though 1/2 = 50% > 30% blast radius, dry-run mode should log and return without mutating or invoking API checks
	runner := NewSafetyRunner(fakeClient, 30.0, 5.0, true, nil)

	deletedPod, _, err := runner.ExecuteKillPod(ctx, "default", "app=demo")
	if err != nil {
		t.Fatalf("expected dry-run kill-pod to succeed without error, got %v", err)
	}
	if !strings.Contains(deletedPod, "[DRY-RUN]") {
		t.Errorf("expected dry-run indication in returned pod string, got %s", deletedPod)
	}

	recLatency, err := runner.ExecuteInjectLatency(ctx, "default", "demo-pod-1", 500)
	if err != nil {
		t.Fatalf("expected dry-run inject-latency to succeed without error, got %v", err)
	}
	if recLatency == nil {
		t.Errorf("expected non-nil recovery time in dry-run inject-latency")
	}

	recCPU, err := runner.ExecuteSpikeCPU(ctx, "default", "demo-pod-1", 30)
	if err != nil {
		t.Fatalf("expected dry-run spike-cpu to succeed without error, got %v", err)
	}
	if recCPU == nil {
		t.Errorf("expected non-nil recovery time in dry-run spike-cpu")
	}

	// Verify Kubernetes API objects were NOT modified
	pods, _ := fakeClient.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})
	if len(pods.Items) != 2 {
		t.Errorf("expected 2 pods to remain intact in dry-run mode, got %d", len(pods.Items))
	}

	pod, _ := fakeClient.CoreV1().Pods("default").Get(ctx, "demo-pod-1", metav1.GetOptions{})
	if len(pod.Spec.EphemeralContainers) != 0 {
		t.Errorf("expected 0 ephemeral containers in dry-run mode, got %d", len(pod.Spec.EphemeralContainers))
	}
}

type testEventPublisher struct {
	events []events.Event
}

func (p *testEventPublisher) Publish(event events.Event) {
	p.events = append(p.events, event)
}

func TestSafety_CheckNamespaceAllowed_Unit(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		allowed     []string
		expectError bool
		errMsgPart  string
	}{
		{
			name:        "empty namespace returns error",
			namespace:   "",
			allowed:     []string{"demo"},
			expectError: true,
			errMsgPart:  "namespace cannot be empty",
		},
		{
			name:        "allowed namespace passes",
			namespace:   "demo",
			allowed:     []string{"demo"},
			expectError: false,
		},
		{
			name:        "disallowed namespace is blocked",
			namespace:   "production",
			allowed:     []string{"demo"},
			expectError: true,
			errMsgPart:  "not in the allowed namespaces list",
		},
		{
			name:        "kube-system blocked with empty allowlist",
			namespace:   "kube-system",
			allowed:     nil,
			expectError: true,
			errMsgPart:  "system namespace and is blocked by the hard-coded denylist",
		},
		{
			name:        "kube-system blocked even if explicitly listed in allowlist",
			namespace:   "kube-system",
			allowed:     []string{"demo", "kube-system"},
			expectError: true,
			errMsgPart:  "system namespace and is blocked by the hard-coded denylist",
		},
		{
			name:        "kube-public blocked even if listed",
			namespace:   "kube-public",
			allowed:     []string{"kube-public"},
			expectError: true,
			errMsgPart:  "system namespace and is blocked by the hard-coded denylist",
		},
		{
			name:        "kube-node-lease blocked even if listed",
			namespace:   "kube-node-lease",
			allowed:     []string{"kube-node-lease"},
			expectError: true,
			errMsgPart:  "system namespace and is blocked by the hard-coded denylist",
		},
		{
			name:        "case-insensitive match for allowed namespace",
			namespace:   "DEMO",
			allowed:     []string{"demo"},
			expectError: false,
		},
		{
			name:        "case-insensitive match for kube-system denylist",
			namespace:   "Kube-System",
			allowed:     []string{"kube-system"},
			expectError: true,
			errMsgPart:  "system namespace and is blocked by the hard-coded denylist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckNamespaceAllowed(tt.namespace, tt.allowed)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsgPart) {
					t.Errorf("expected error containing %q, got %q", tt.errMsgPart, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestSafety_NamespaceAllowlist_AllowedPasses(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "demo", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "demo", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-3", Namespace: "demo", Labels: map[string]string{"app": "demo"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-4", Namespace: "demo", Labels: map[string]string{"app": "demo"}}},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	runner := NewSafetyRunner(fakeClient, 30.0, 5.0, false, nil, []string{"demo"})

	deletedPod, _, err := runner.ExecuteKillPod(ctx, "demo", "app=demo")
	if err != nil {
		t.Fatalf("expected allowed namespace experiment to pass, got: %v", err)
	}
	if deletedPod == "" {
		t.Errorf("expected a pod to have been deleted")
	}
}

func TestSafety_NamespaceAllowlist_DisallowedBlocked(t *testing.T) {
	metrics.ExperimentsTotal.Reset()

	fakeClient := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "production", Labels: map[string]string{"app": "prod-app"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "production", Labels: map[string]string{"app": "prod-app"}}},
	)

	pub := &testEventPublisher{}
	runner := NewSafetyRunner(fakeClient, 30.0, 5.0, false, nil, []string{"demo"})
	runner.SetPublisher(pub)

	ctx := context.Background()
	_, _, err := runner.ExecuteKillPod(ctx, "production", "app=prod-app")
	if err == nil {
		t.Fatalf("expected experiment in disallowed namespace to be blocked, got nil")
	}

	if !strings.Contains(err.Error(), "not in the allowed namespaces list") {
		t.Errorf("expected error to mention allowed namespaces list, got: %v", err)
	}

	// Verify metric incremented for blocked
	count := testutil.ToFloat64(metrics.ExperimentsTotal.WithLabelValues("kill_pod", "blocked"))
	if count != 1 {
		t.Errorf("expected 1 blocked metric count, got %v", count)
	}

	// Verify blocked event was emitted
	if len(pub.events) == 0 {
		t.Fatalf("expected at least one event published")
	}
	blockedFound := false
	for _, ev := range pub.events {
		if ev.Type == events.EventBlocked {
			blockedFound = true
			break
		}
	}
	if !blockedFound {
		t.Errorf("expected EventBlocked to be published, but got events: %+v", pub.events)
	}

	// Verify cluster was untouched
	pods, listErr := fakeClient.CoreV1().Pods("production").List(ctx, metav1.ListOptions{})
	if listErr != nil {
		t.Fatalf("failed to list pods: %v", listErr)
	}
	if len(pods.Items) != 2 {
		t.Errorf("expected pods in cluster to remain untouched (2 pods), got %d", len(pods.Items))
	}
}

func TestSafety_NamespaceAllowlist_KubeSystemBlockedEvenIfListed(t *testing.T) {
	metrics.ExperimentsTotal.Reset()

	fakeClient := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "kube-dns", Namespace: "kube-system", Labels: map[string]string{"app": "kube-dns"}}},
	)

	pub := &testEventPublisher{}
	// Even if an administrator mistakenly lists kube-system in allowed namespaces:
	runner := NewSafetyRunner(fakeClient, 30.0, 5.0, false, nil, []string{"demo", "kube-system"})
	runner.SetPublisher(pub)

	ctx := context.Background()
	_, _, err := runner.ExecuteKillPod(ctx, "kube-system", "app=kube-dns")
	if err == nil {
		t.Fatalf("expected kube-system experiment to be blocked even if listed, got nil")
	}

	if !strings.Contains(err.Error(), "system namespace and is blocked by the hard-coded denylist") {
		t.Errorf("expected error to state system namespace denylist, got: %v", err)
	}

	// Verify metric incremented for blocked
	count := testutil.ToFloat64(metrics.ExperimentsTotal.WithLabelValues("kill_pod", "blocked"))
	if count != 1 {
		t.Errorf("expected 1 blocked metric count, got %v", count)
	}

	// Verify blocked event emitted
	blockedFound := false
	for _, ev := range pub.events {
		if ev.Type == events.EventBlocked {
			blockedFound = true
			break
		}
	}
	if !blockedFound {
		t.Errorf("expected EventBlocked to be published, but got events: %+v", pub.events)
	}

	// Verify cluster was untouched
	pods, listErr := fakeClient.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if listErr != nil {
		t.Fatalf("failed to list pods: %v", listErr)
	}
	if len(pods.Items) != 1 {
		t.Errorf("expected kube-system pods in cluster to remain untouched (1 pod), got %d", len(pods.Items))
	}
}
