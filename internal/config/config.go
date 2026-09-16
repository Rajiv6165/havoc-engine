package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	
	"havoc-engine/internal/scoring"
)

type Config struct {
	Kubeconfig              string         `yaml:"kubeconfig"`
	DefaultNamespace        string         `yaml:"default_namespace"`
	MaxBlastRadiusPercent   float64        `yaml:"maxBlastRadiusPercent"`
	AbortOnErrorRatePercent float64        `yaml:"abortOnErrorRatePercent"`
	DatabaseDSN             string         `yaml:"database_dsn"`
	AnthropicAPIKey         string         `yaml:"anthropic_api_key"`
	Scoring                 scoring.Config `yaml:"scoring"`
	Scheduler               SchedulerConfig `yaml:"scheduler"`
}

type SchedulerConfig struct {
	Enabled        bool         `yaml:"enabled"`
	AllowedWindows []TimeWindow `yaml:"allowedWindows"`
	MinExecutions  int          `yaml:"minExecutions"`
	MaxExecutions  int          `yaml:"maxExecutions"`
	Period         string       `yaml:"period"`
}

type TimeWindow struct {
	Days      []string `yaml:"days"`
	StartTime string   `yaml:"startTime"`
	EndTime   string   `yaml:"endTime"`
}

// LoadConfig reads configuration from the given file path.
// If the file does not exist, default values are returned.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		Kubeconfig:              getDefaultKubeconfigPath(),
		DefaultNamespace:        "default",
		MaxBlastRadiusPercent:   30.0,
		AbortOnErrorRatePercent: 5.0,
		DatabaseDSN:             "postgres://postgres:postgres@localhost:5432/havoc?sslmode=disable",
		Scoring:                 scoring.DefaultConfig(),
		Scheduler: SchedulerConfig{
			Enabled: false,
		},
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

	if cfg.MaxBlastRadiusPercent <= 0 {
		cfg.MaxBlastRadiusPercent = 30.0
	}
	if cfg.AbortOnErrorRatePercent <= 0 {
		cfg.AbortOnErrorRatePercent = 5.0
	}

	cfg.Kubeconfig = expandHomePath(cfg.Kubeconfig)
	
	// Read API key from environment, prioritizing it over the config file
	if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
		cfg.AnthropicAPIKey = envKey
	}
	
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
