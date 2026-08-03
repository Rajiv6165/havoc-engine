package k8sclient

import (
	"testing"
)

func TestNewClient_InvalidPath(t *testing.T) {
	_, err := NewClient("/non/existent/kubeconfig/path")
	if err == nil {
		t.Fatalf("expected error when loading invalid kubeconfig path outside cluster, got nil")
	}
}
