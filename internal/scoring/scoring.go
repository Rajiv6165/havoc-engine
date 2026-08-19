package scoring

import "math"

// Config defines the tunable thresholds for resilience scoring.
type Config struct {
	RecoveryTimeOptimalMs   int `yaml:"recoveryTimeOptimalMs"`
	RecoveryTimeMaxMs       int `yaml:"recoveryTimeMaxMs"`
	ErrorRateOptimalPercent float64 `yaml:"errorRateOptimalPercent"`
	ErrorRateMaxPercent     float64 `yaml:"errorRateMaxPercent"`
	SafetyInterventionPenalty int `yaml:"safetyInterventionPenalty"`
}

// DefaultConfig provides sensible defaults if not specified in config.yaml.
func DefaultConfig() Config {
	return Config{
		RecoveryTimeOptimalMs:     5000,   // 5 seconds
		RecoveryTimeMaxMs:         30000,  // 30 seconds
		ErrorRateOptimalPercent:   1.0,    // 1% error rate is fine
		ErrorRateMaxPercent:       10.0,   // 10% is very bad
		SafetyInterventionPenalty: 50,     // Heavy penalty if safety had to intervene
	}
}

// CalculateResilienceScore calculates a score from 0-100 based on experiment metrics.
func CalculateResilienceScore(recoveryTimeMs *int, errorRate float64, safetyIntervened bool, cfg Config) int {
	score := 100.0

	// 1. Safety Intervention Penalty
	if safetyIntervened {
		score -= float64(cfg.SafetyInterventionPenalty)
	}

	// 2. Recovery Time Penalty
	// E.g. Optimal = 5000, Max = 30000. Penalty scales from 0 at Optimal to 30 at Max.
	// We'll use 30 points as the maximum penalty for slow recovery.
	if recoveryTimeMs != nil {
		rt := float64(*recoveryTimeMs)
		opt := float64(cfg.RecoveryTimeOptimalMs)
		maxRt := float64(cfg.RecoveryTimeMaxMs)

		if rt > opt {
			if rt >= maxRt {
				score -= 30.0
			} else {
				// Linear scale between optimal and max
				penalty := 30.0 * ((rt - opt) / (maxRt - opt))
				score -= penalty
			}
		}
	}

	// 3. Error Rate Penalty
	// E.g. Optimal = 1.0, Max = 10.0. Penalty scales from 0 at Optimal to 30 at Max.
	if errorRate > cfg.ErrorRateOptimalPercent {
		if errorRate >= cfg.ErrorRateMaxPercent {
			score -= 30.0
		} else {
			// Linear scale between optimal and max
			opt := cfg.ErrorRateOptimalPercent
			maxErr := cfg.ErrorRateMaxPercent
			penalty := 30.0 * ((errorRate - opt) / (maxErr - opt))
			score -= penalty
		}
	}

	// Floor at 0 and round to nearest int
	finalScore := int(math.Round(score))
	if finalScore < 0 {
		finalScore = 0
	}
	return finalScore
}
