package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := []byte("kubeconfig: /custom/kube/config\ndefault_namespace: test-ns\n")
	if err := os.WriteFile(configFile, content, 0644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Kubeconfig != "/custom/kube/config" {
		t.Errorf("expected Kubeconfig '/custom/kube/config', got '%s'", cfg.Kubeconfig)
	}
	if cfg.DefaultNamespace != "test-ns" {
		t.Errorf("expected DefaultNamespace 'test-ns', got '%s'", cfg.DefaultNamespace)
	}
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	cfg, err := LoadConfig("non_existent_file.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing config file, got %v", err)
	}
	if cfg.DefaultNamespace != "default" {
		t.Errorf("expected default namespace 'default', got '%s'", cfg.DefaultNamespace)
	}
}
