package scoring

import (
	"testing"
)

func TestCalculateResilienceScore(t *testing.T) {
	cfg := DefaultConfig()

	ptr := func(i int) *int {
		return &i
	}

	tests := []struct {
		name             string
		recoveryTimeMs   *int
		errorRate        float64
		safetyIntervened bool
		expectedScore    int
	}{
		{
			name:             "Perfect run",
			recoveryTimeMs:   ptr(3000), // < 5000
			errorRate:        0.5,       // < 1.0
			safetyIntervened: false,
			expectedScore:    100,
		},
		{
			name:             "Slow recovery max penalty",
			recoveryTimeMs:   ptr(35000), // > 30000 (30pt penalty)
			errorRate:        0.5,
			safetyIntervened: false,
			expectedScore:    70,
		},
		{
			name:             "High error rate max penalty",
			recoveryTimeMs:   ptr(3000),
			errorRate:        12.0, // > 10.0 (30pt penalty)
			safetyIntervened: false,
			expectedScore:    70,
		},
		{
			name:             "Safety intervened",
			recoveryTimeMs:   ptr(3000),
			errorRate:        0.5,
			safetyIntervened: true, // 50pt penalty
			expectedScore:    50,
		},
		{
			name:             "Everything bad (should floor at 0)",
			recoveryTimeMs:   ptr(40000), // 30 penalty
			errorRate:        15.0,       // 30 penalty
			safetyIntervened: true,       // 50 penalty (Total 110 penalty)
			expectedScore:    0,
		},
		{
			name:             "Midway recovery and error rate",
			recoveryTimeMs:   ptr(17500), // exactly halfway between 5s and 30s -> 15 penalty
			errorRate:        5.5,        // exactly halfway between 1% and 10% -> 15 penalty
			safetyIntervened: false,
			expectedScore:    70,
		},
		{
			name:             "No recovery time provided",
			recoveryTimeMs:   nil,
			errorRate:        0.5,
			safetyIntervened: false,
			expectedScore:    100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := CalculateResilienceScore(tc.recoveryTimeMs, tc.errorRate, tc.safetyIntervened, cfg)
			if score != tc.expectedScore {
				t.Errorf("Expected score %d, got %d", tc.expectedScore, score)
			}
		})
	}
}
