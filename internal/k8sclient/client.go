package k8sclient

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClient creates a new Kubernetes clientset implementing kubernetes.Interface.
// It attempts to build configuration from the provided kubeconfig path first.
// If the file is not found or empty, it falls back to in-cluster config.
func NewClient(kubeconfigPath string) (kubernetes.Interface, error) {
	var restConfig *rest.Config
	var err error

	if kubeconfigPath != "" {
		if _, statErr := os.Stat(kubeconfigPath); statErr == nil {
			restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return nil, fmt.Errorf("failed to build config from kubeconfig %s: %w", kubeconfigPath, err)
			}
		}
	}

	if restConfig == nil {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			if kubeconfigPath != "" {
				return nil, fmt.Errorf("could not load kubeconfig at %s nor in-cluster config: %w", kubeconfigPath, err)
			}
			return nil, fmt.Errorf("could not load in-cluster config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return clientset, nil
}
