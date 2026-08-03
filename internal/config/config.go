package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Kubeconfig       string `yaml:"kubeconfig"`
	DefaultNamespace string `yaml:"default_namespace"`
}

// LoadConfig reads configuration from the given file path.
// If the file does not exist, default values are returned.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		Kubeconfig:       getDefaultKubeconfigPath(),
		DefaultNamespace: "default",
	}

	if path == "" {
		path = "config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.Kubeconfig = expandHomePath(cfg.Kubeconfig)
	return cfg, nil
}

func getDefaultKubeconfigPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}

func expandHomePath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if len(path) >= 2 && (path[:2] == "~/" || path[:2] == "~\\") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
