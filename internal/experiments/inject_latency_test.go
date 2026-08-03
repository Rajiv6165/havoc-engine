package experiments

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestInjectLatency_Success(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod",
				Namespace: "default",
			},
		},
	)

	ctx := context.Background()
	err := InjectLatency(ctx, fakeClient, "default", "demo-pod", 500)
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
	if ec.Image != "nicolaka/netshoot" {
		t.Errorf("expected image 'nicolaka/netshoot', got %s", ec.Image)
	}

	expectedDelay := "500ms"
	foundDelay := false
	for _, arg := range ec.Command {
		if arg == expectedDelay {
			foundDelay = true
			break
		}
	}
	if !foundDelay {
		t.Errorf("command does not contain expected delay %s: %v", expectedDelay, ec.Command)
	}

	if ec.SecurityContext == nil || ec.SecurityContext.Capabilities == nil {
		t.Fatalf("expected SecurityContext with Capabilities, got nil")
	}

	hasNetAdmin := false
	for _, cap := range ec.SecurityContext.Capabilities.Add {
		if cap == "NET_ADMIN" {
			hasNetAdmin = true
			break
		}
	}
	if !hasNetAdmin {
		t.Errorf("expected NET_ADMIN capability in ephemeral container")
	}
}

func TestInjectLatency_PodNotFound(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	ctx := context.Background()

	err := InjectLatency(ctx, fakeClient, "default", "non-existent-pod", 200)
	if err == nil {
		t.Fatalf("expected error for missing pod, got nil")
	}
}

func TestInjectLatency_InvalidInputs(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	ctx := context.Background()

	if err := InjectLatency(ctx, fakeClient, "", "demo-pod", 100); err == nil {
		t.Errorf("expected error for empty namespace")
	}

	if err := InjectLatency(ctx, fakeClient, "default", "", 100); err == nil {
		t.Errorf("expected error for empty podName")
	}

	if err := InjectLatency(ctx, fakeClient, "default", "demo-pod", 0); err == nil {
		t.Errorf("expected error for zero delayMs")
	}

	if err := InjectLatency(ctx, fakeClient, "default", "demo-pod", -50); err == nil {
		t.Errorf("expected error for negative delayMs")
	}
}
