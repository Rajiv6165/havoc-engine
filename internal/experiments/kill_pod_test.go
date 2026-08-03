package experiments

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestKillPod_Success(t *testing.T) {
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
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-pod",
				Namespace: "default",
				Labels:    map[string]string{"app": "other"},
			},
		},
	)

	ctx := context.Background()
	deletedPodName, err := KillPod(ctx, fakeClient, "default", "app=demo")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if deletedPodName != "demo-pod-1" && deletedPodName != "demo-pod-2" {
		t.Errorf("unexpected deleted pod name: %s", deletedPodName)
	}

	pods, err := fakeClient.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list pods: %v", err)
	}

	if len(pods.Items) != 2 {
		t.Errorf("expected 2 remaining pods, got %d", len(pods.Items))
	}
}

func TestKillPod_NoMatchingPods(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	ctx := context.Background()

	_, err := KillPod(ctx, fakeClient, "default", "app=nonexistent")
	if err == nil {
		t.Fatalf("expected error for no matching pods, got nil")
	}
}

func TestKillPod_EmptyInputs(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	ctx := context.Background()

	if _, err := KillPod(ctx, fakeClient, "", "app=demo"); err == nil {
		t.Errorf("expected error for empty namespace")
	}

	if _, err := KillPod(ctx, fakeClient, "default", ""); err == nil {
		t.Errorf("expected error for empty labelSelector")
	}
}
