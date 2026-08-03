package experiments

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSpikeCPU_Success(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod",
				Namespace: "default",
			},
		},
	)

	ctx := context.Background()
	err := SpikeCPU(ctx, fakeClient, "default", "demo-pod", 30)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	pod, err := fakeClient.CoreV1().Pods("default").Get(ctx, "demo-pod", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to fetch updated pod: %v", err)
	}

	if len(pod.Spec.EphemeralContainers) != 1 {
		t.Fatalf("expected 1 ephemeral container, got %d", len(pod.Spec.EphemeralContainers))
	}

	ec := pod.Spec.EphemeralContainers[0]
	if ec.Image != "alpinelinux/stress-ng" {
		t.Errorf("expected image 'alpinelinux/stress-ng', got %s", ec.Image)
	}

	expectedDuration := "30s"
	foundDuration := false
	for _, arg := range ec.Command {
		if arg == expectedDuration {
			foundDuration = true
			break
		}
	}
	if !foundDuration {
		t.Errorf("command does not contain expected duration %s: %v", expectedDuration, ec.Command)
	}
}

func TestSpikeCPU_PodNotFound(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	ctx := context.Background()

	err := SpikeCPU(ctx, fakeClient, "default", "non-existent-pod", 30)
	if err == nil {
		t.Fatalf("expected error for missing pod, got nil")
	}
}

func TestSpikeCPU_InvalidInputs(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	ctx := context.Background()

	if err := SpikeCPU(ctx, fakeClient, "", "demo-pod", 30); err == nil {
		t.Errorf("expected error for empty namespace")
	}

	if err := SpikeCPU(ctx, fakeClient, "default", "", 30); err == nil {
		t.Errorf("expected error for empty podName")
	}

	if err := SpikeCPU(ctx, fakeClient, "default", "demo-pod", 0); err == nil {
		t.Errorf("expected error for zero durationSec")
	}

	if err := SpikeCPU(ctx, fakeClient, "default", "demo-pod", -10); err == nil {
		t.Errorf("expected error for negative durationSec")
	}
}
