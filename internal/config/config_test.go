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
	if cfg.MaxBlastRadiusPercent != 30.0 {
		t.Errorf("expected default MaxBlastRadiusPercent 30.0, got %f", cfg.MaxBlastRadiusPercent)
	}
	if cfg.AbortOnErrorRatePercent != 5.0 {
		t.Errorf("expected default AbortOnErrorRatePercent 5.0, got %f", cfg.AbortOnErrorRatePercent)
	}
	if cfg.Prometheus.URL != "http://localhost:9092" {
		t.Errorf("expected default Prometheus URL 'http://localhost:9092', got '%s'", cfg.Prometheus.URL)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected default Server.Host '127.0.0.1', got '%s'", cfg.Server.Host)
	}
}

func TestLoadConfig_PrometheusAndEnvOverrides(t *testing.T) {
	os.Setenv("PROMETHEUS_URL", "http://prometheus:9090")
	os.Setenv("HAVOC_API_TOKEN", "test-secret-token")
	os.Setenv("HAVOC_BIND_ADDRESS", "0.0.0.0")
	defer func() {
		os.Unsetenv("PROMETHEUS_URL")
		os.Unsetenv("HAVOC_API_TOKEN")
		os.Unsetenv("HAVOC_BIND_ADDRESS")
	}()

	cfg, err := LoadConfig("non_existent_file.yaml")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Prometheus.URL != "http://prometheus:9090" {
		t.Errorf("expected Prometheus URL override 'http://prometheus:9090', got '%s'", cfg.Prometheus.URL)
	}
	if cfg.API.Token != "test-secret-token" {
		t.Errorf("expected API token override 'test-secret-token', got '%s'", cfg.API.Token)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected Host override '0.0.0.0', got '%s'", cfg.Server.Host)
	}
}
