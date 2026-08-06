package safety

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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

	_, err := runner.ExecuteKillPod(ctx, "default", "app=demo")
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

	ctx := context.Background()
	runner := NewSafetyRunner(fakeClient, 30.0, 5.0, false, nil)

	deletedPod, err := runner.ExecuteKillPod(ctx, "default", "app=demo")
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

	err := runner.ExecuteInjectLatency(ctx, "default", "demo-pod-1", 500)
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

	deletedPod, err := runner.ExecuteKillPod(ctx, "default", "app=demo")
	if err != nil {
		t.Fatalf("expected dry-run kill-pod to succeed without error, got %v", err)
	}
	if !strings.Contains(deletedPod, "[DRY-RUN]") {
		t.Errorf("expected dry-run indication in returned pod string, got %s", deletedPod)
	}

	err = runner.ExecuteInjectLatency(ctx, "default", "demo-pod-1", 500)
	if err != nil {
		t.Fatalf("expected dry-run inject-latency to succeed without error, got %v", err)
	}

	err = runner.ExecuteSpikeCPU(ctx, "default", "demo-pod-1", 30)
	if err != nil {
		t.Fatalf("expected dry-run spike-cpu to succeed without error, got %v", err)
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
