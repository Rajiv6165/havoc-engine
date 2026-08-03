package experiments

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SpikeCPU stresses CPU inside a target pod using a stress-ng ephemeral container for durationSec seconds.
func SpikeCPU(ctx context.Context, client kubernetes.Interface, namespace, podName string, durationSec int) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if podName == "" {
		return fmt.Errorf("podName cannot be empty")
	}
	if durationSec <= 0 {
		return fmt.Errorf("durationSec must be greater than 0")
	}

	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod %s in namespace %s: %w", podName, namespace, err)
	}

	ephemeralContainerName := fmt.Sprintf("chaos-cpu-%d", time.Now().Unix())
	durationStr := fmt.Sprintf("%ds", durationSec)

	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:    ephemeralContainerName,
			Image:   "alpinelinux/stress-ng",
			Command: []string{"stress-ng", "--cpu", "2", "--timeout", durationStr},
		},
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ephemeralContainer)

	_, err = client.CoreV1().Pods(namespace).UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{})
	if err != nil {
		// Fallback to standard Update if subresource fails or isn't mocked
		_, updateErr := client.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("failed to inject ephemeral cpu stress container into pod %s: %w", podName, err)
		}
	}

	return nil
}
