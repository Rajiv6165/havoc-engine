package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	
	"havoc-engine/internal/scoring"
)

type Config struct {
	Kubeconfig              string          `yaml:"kubeconfig"`
	DefaultNamespace        string          `yaml:"default_namespace"`
	TargetNamespaces        []string        `yaml:"target_namespaces"`
	MaxBlastRadiusPercent   float64         `yaml:"maxBlastRadiusPercent"`
	AbortOnErrorRatePercent float64         `yaml:"abortOnErrorRatePercent"`
	DatabaseDSN             string          `yaml:"database_dsn"`
	AnthropicAPIKey         string          `yaml:"anthropic_api_key"`
	Scoring                 scoring.Config  `yaml:"scoring"`
	Scheduler               SchedulerConfig `yaml:"scheduler"`
	Server                  ServerConfig    `yaml:"server"`
	Topology                TopologyConfig  `yaml:"topology"`
}

type ServerConfig struct {
	Port        int      `yaml:"port"`
	WSPort      int      `yaml:"ws_port"`
	MetricsPort int      `yaml:"metrics_port"`
	AllowedCORS []string `yaml:"allowed_cors"`
}

type TopologyConfig struct {
	Nodes []TopologyNode `yaml:"nodes"`
	Edges []TopologyEdge `yaml:"edges"`
}

type TopologyNode struct {
	ID       string `yaml:"id" json:"id"`
	Label    string `yaml:"label" json:"label"`
	PodCount int    `yaml:"podCount" json:"podCount"`
}

type TopologyEdge struct {
	Source string `yaml:"source" json:"source"`
	Target string `yaml:"target" json:"target"`
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
		TargetNamespaces:        []string{"default"},
		MaxBlastRadiusPercent:   30.0,
		AbortOnErrorRatePercent: 5.0,
		DatabaseDSN:             "postgres://postgres:postgres@localhost:5432/havoc?sslmode=disable",
		Scoring:                 scoring.DefaultConfig(),
		Scheduler: SchedulerConfig{
			Enabled: false,
		},
		Server: ServerConfig{
			Port:        8080,
			WSPort:      9091,
			MetricsPort: 9090,
			AllowedCORS: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
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
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.WSPort <= 0 {
		cfg.Server.WSPort = 9091
	}
	if cfg.Server.MetricsPort <= 0 {
		cfg.Server.MetricsPort = 9090
	}
	if len(cfg.TargetNamespaces) == 0 {
		cfg.TargetNamespaces = []string{cfg.DefaultNamespace}
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
