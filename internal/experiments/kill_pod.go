package experiments

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SelectTargetPod finds a random pod matching the label selector in the namespace.
func SelectTargetPod(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) (*corev1.Pod, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}
	if labelSelector == "" {
		return nil, fmt.Errorf("labelSelector cannot be empty")
	}

	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found matching selector %q in namespace %q", labelSelector, namespace)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	selectedIndex := r.Intn(len(pods.Items))
	targetPod := pods.Items[selectedIndex]
	return &targetPod, nil
}

// DeletePod deletes a pod by name in the specified namespace.
func DeletePod(ctx context.Context, client kubernetes.Interface, namespace, podName string) error {
	return client.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

// KillPod deletes a random pod in the specified namespace matching the label selector.
// Returns the name of the deleted pod or an error.
func KillPod(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) (string, error) {
	pod, err := SelectTargetPod(ctx, client, namespace, labelSelector)
	if err != nil {
		return "", err
	}

	if err := DeletePod(ctx, client, namespace, pod.Name); err != nil {
		return "", fmt.Errorf("failed to delete pod %s: %w", pod.Name, err)
	}

	return pod.Name, nil
}
