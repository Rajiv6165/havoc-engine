package experiments

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RemoveLatency removes chaos latency ephemeral containers from the specified pod.
func RemoveLatency(ctx context.Context, client kubernetes.Interface, namespace, podName string) error {
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod %s for latency rollback: %w", podName, err)
	}

	var updatedEphemeralContainers []corev1.EphemeralContainer
	for _, ec := range pod.Spec.EphemeralContainers {
		if !strings.HasPrefix(ec.Name, "chaos-latency-") {
			updatedEphemeralContainers = append(updatedEphemeralContainers, ec)
		}
	}
	pod.Spec.EphemeralContainers = updatedEphemeralContainers

	_, err = client.CoreV1().Pods(namespace).UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{})
	if err != nil {
		_, updateErr := client.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("failed to rollback latency ephemeral container on pod %s: %w", podName, err)
		}
	}
	return nil
}

// RemoveCPUStress removes chaos CPU stress ephemeral containers from the specified pod.
func RemoveCPUStress(ctx context.Context, client kubernetes.Interface, namespace, podName string) error {
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod %s for CPU stress rollback: %w", podName, err)
	}

	var updatedEphemeralContainers []corev1.EphemeralContainer
	for _, ec := range pod.Spec.EphemeralContainers {
		if !strings.HasPrefix(ec.Name, "chaos-cpu-") {
			updatedEphemeralContainers = append(updatedEphemeralContainers, ec)
		}
	}
	pod.Spec.EphemeralContainers = updatedEphemeralContainers

	_, err = client.CoreV1().Pods(namespace).UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{})
	if err != nil {
		_, updateErr := client.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("failed to rollback CPU stress ephemeral container on pod %s: %w", podName, err)
		}
	}
	return nil
}
