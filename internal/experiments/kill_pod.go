package experiments

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// KillPod deletes a random pod in the specified namespace matching the label selector.
// Returns the name of the deleted pod or an error.
func KillPod(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) (string, error) {
	if namespace == "" {
		return "", fmt.Errorf("namespace cannot be empty")
	}
	if labelSelector == "" {
		return "", fmt.Errorf("labelSelector cannot be empty")
	}

	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found matching selector %q in namespace %q", labelSelector, namespace)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	selectedIndex := r.Intn(len(pods.Items))
	targetPod := pods.Items[selectedIndex]

	err = client.CoreV1().Pods(namespace).Delete(ctx, targetPod.Name, metav1.DeleteOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to delete pod %s: %w", targetPod.Name, err)
	}

	return targetPod.Name, nil
}
