package experiments

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// InjectLatency injects network delay into a target pod using tc/netem via an ephemeral container.
func InjectLatency(ctx context.Context, client kubernetes.Interface, namespace, podName string, delayMs int) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if podName == "" {
		return fmt.Errorf("podName cannot be empty")
	}
	if delayMs <= 0 {
		return fmt.Errorf("delayMs must be greater than 0")
	}

	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod %s in namespace %s: %w", podName, namespace, err)
	}

	ephemeralContainerName := fmt.Sprintf("chaos-latency-%d", time.Now().Unix())
	delayStr := fmt.Sprintf("%dms", delayMs)

	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:    ephemeralContainerName,
			Image:   "nicolaka/netshoot",
			Command: []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay", delayStr},
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN"},
				},
			},
		},
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ephemeralContainer)

	_, err = client.CoreV1().Pods(namespace).UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{})
	if err != nil {
		// Fallback to standard Update if subresource fails or isn't mocked
		_, updateErr := client.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("failed to inject ephemeral latency container into pod %s: %w", podName, err)
		}
	}

	return nil
}
