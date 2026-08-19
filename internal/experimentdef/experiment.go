package experimentdef

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Experiment defines the schema for a Chaos-as-Code YAML file.
type Experiment struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Target      Target `yaml:"target"`
	Action      string `yaml:"action"`
	Params      Params `yaml:"params,omitempty"`
	Safety      Safety `yaml:"safety,omitempty"`
	Schedule    string `yaml:"schedule,omitempty"`
}

// Target defines where the experiment should be applied.
type Target struct {
	Namespace     string `yaml:"namespace"`
	LabelSelector string `yaml:"labelSelector,omitempty"`
	PodName       string `yaml:"podName,omitempty"`
}

// Params defines parameters specific to the chosen action.
type Params struct {
	DelayMs     int `yaml:"delayMs,omitempty"`
	DurationSec int `yaml:"durationSec,omitempty"`
}

// Safety overrides global safety limits for this specific experiment.
type Safety struct {
	MaxBlastRadiusPercent   float64 `yaml:"maxBlastRadiusPercent,omitempty"`
	AbortOnErrorRatePercent float64 `yaml:"abortOnErrorRatePercent,omitempty"`
}

// Valid Actions
const (
	ActionKillPod      = "kill-pod"
	ActionInjectLatency = "inject-latency"
	ActionSpikeCPU      = "spike-cpu"
)

// LoadExperiment reads and parses a YAML file into an Experiment struct.
func LoadExperiment(path string) (*Experiment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	var exp Experiment
	if err := yaml.Unmarshal(data, &exp); err != nil {
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", path, err)
	}

	return &exp, nil
}

// Validate checks if the experiment definition is valid according to the schema rules.
func (e *Experiment) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("missing required field: name")
	}
	if e.Target.Namespace == "" {
		return fmt.Errorf("missing required field: target.namespace")
	}
	if e.Action == "" {
		return fmt.Errorf("missing required field: action")
	}

	switch e.Action {
	case ActionKillPod:
		if e.Target.LabelSelector == "" {
			return fmt.Errorf("target.labelSelector is required for action %q", ActionKillPod)
		}
		if e.Params.DelayMs != 0 || e.Params.DurationSec != 0 {
			return fmt.Errorf("params are not supported for action %q", ActionKillPod)
		}
	case ActionInjectLatency:
		if e.Target.PodName == "" {
			return fmt.Errorf("target.podName is required for action %q", ActionInjectLatency)
		}
		if e.Params.DelayMs <= 0 {
			return fmt.Errorf("params.delayMs must be greater than 0 for action %q", ActionInjectLatency)
		}
		if e.Params.DurationSec != 0 {
			return fmt.Errorf("params.durationSec is not supported for action %q", ActionInjectLatency)
		}
	case ActionSpikeCPU:
		if e.Target.PodName == "" {
			return fmt.Errorf("target.podName is required for action %q", ActionSpikeCPU)
		}
		if e.Params.DurationSec <= 0 {
			return fmt.Errorf("params.durationSec must be greater than 0 for action %q", ActionSpikeCPU)
		}
		if e.Params.DelayMs != 0 {
			return fmt.Errorf("params.delayMs is not supported for action %q", ActionSpikeCPU)
		}
	default:
		return fmt.Errorf("unknown action: %q (supported actions: %s, %s, %s)", e.Action, ActionKillPod, ActionInjectLatency, ActionSpikeCPU)
	}

	return nil
}
