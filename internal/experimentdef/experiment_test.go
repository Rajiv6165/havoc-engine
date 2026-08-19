package experimentdef

import (
	"strings"
	"testing"
)

func TestValidate_Valid(t *testing.T) {
	tests := []struct {
		name string
		exp  Experiment
	}{
		{
			name: "valid kill-pod",
			exp: Experiment{
				Name: "test-kill",
				Target: Target{
					Namespace:     "default",
					LabelSelector: "app=demo",
				},
				Action: ActionKillPod,
			},
		},
		{
			name: "valid inject-latency",
			exp: Experiment{
				Name: "test-latency",
				Target: Target{
					Namespace: "default",
					PodName:   "demo-pod-123",
				},
				Action: ActionInjectLatency,
				Params: Params{
					DelayMs: 500,
				},
			},
		},
		{
			name: "valid spike-cpu",
			exp: Experiment{
				Name: "test-cpu",
				Target: Target{
					Namespace: "default",
					PodName:   "demo-pod-123",
				},
				Action: ActionSpikeCPU,
				Params: Params{
					DurationSec: 30,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.exp.Validate(); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

func TestValidate_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		exp         Experiment
		errContains string
	}{
		{
			name: "missing name",
			exp: Experiment{
				Target: Target{Namespace: "default"},
				Action: ActionKillPod,
			},
			errContains: "missing required field: name",
		},
		{
			name: "missing namespace",
			exp: Experiment{
				Name:   "test",
				Action: ActionKillPod,
			},
			errContains: "missing required field: target.namespace",
		},
		{
			name: "missing action",
			exp: Experiment{
				Name:   "test",
				Target: Target{Namespace: "default"},
			},
			errContains: "missing required field: action",
		},
		{
			name: "unknown action",
			exp: Experiment{
				Name:   "test",
				Target: Target{Namespace: "default"},
				Action: "invalid-action",
			},
			errContains: "unknown action",
		},
		{
			name: "kill-pod missing selector",
			exp: Experiment{
				Name:   "test",
				Target: Target{Namespace: "default"},
				Action: ActionKillPod,
			},
			errContains: "target.labelSelector is required",
		},
		{
			name: "kill-pod with params",
			exp: Experiment{
				Name:   "test",
				Target: Target{Namespace: "default", LabelSelector: "app=demo"},
				Action: ActionKillPod,
				Params: Params{DelayMs: 100},
			},
			errContains: "params are not supported for action",
		},
		{
			name: "inject-latency missing podName",
			exp: Experiment{
				Name:   "test",
				Target: Target{Namespace: "default"},
				Action: ActionInjectLatency,
				Params: Params{DelayMs: 100},
			},
			errContains: "target.podName is required",
		},
		{
			name: "inject-latency missing delayMs",
			exp: Experiment{
				Name:   "test",
				Target: Target{Namespace: "default", PodName: "demo"},
				Action: ActionInjectLatency,
			},
			errContains: "params.delayMs must be greater than 0",
		},
		{
			name: "inject-latency with durationSec",
			exp: Experiment{
				Name:   "test",
				Target: Target{Namespace: "default", PodName: "demo"},
				Action: ActionInjectLatency,
				Params: Params{DelayMs: 100, DurationSec: 30},
			},
			errContains: "params.durationSec is not supported",
		},
		{
			name: "spike-cpu missing podName",
			exp: Experiment{
				Name:   "test",
				Target: Target{Namespace: "default"},
				Action: ActionSpikeCPU,
				Params: Params{DurationSec: 30},
			},
			errContains: "target.podName is required",
		},
		{
			name: "spike-cpu missing durationSec",
			exp: Experiment{
				Name:   "test",
				Target: Target{Namespace: "default", PodName: "demo"},
				Action: ActionSpikeCPU,
			},
			errContains: "params.durationSec must be greater than 0",
		},
		{
			name: "spike-cpu with delayMs",
			exp: Experiment{
				Name:   "test",
				Target: Target{Namespace: "default", PodName: "demo"},
				Action: ActionSpikeCPU,
				Params: Params{DurationSec: 30, DelayMs: 100},
			},
			errContains: "params.delayMs is not supported",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.exp.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("expected error containing %q, got: %v", tc.errContains, err)
			}
		})
	}
}
