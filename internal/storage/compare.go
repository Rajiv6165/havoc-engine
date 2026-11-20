package storage

import (
	"context"
	"fmt"
	"math"
	"sort"
)

// LabelStats holds aggregated metrics for a specific experiment label.
type LabelStats struct {
	Label               string   `json:"label"`
	RunCount            int      `json:"runCount"`
	AvgScore            float64  `json:"avgScore"`
	AvgRecoveryMs       float64  `json:"avgRecoveryMs"`
	P95RecoveryMs       *float64 `json:"p95RecoveryMs,omitempty"`
	SafetyInterventions int      `json:"safetyInterventions"`
	AvgAvailability     *float64 `json:"avgAvailability,omitempty"`
	AvgDowntimeMs       *float64 `json:"avgDowntimeMs,omitempty"`
}

// ComparisonDelta holds differences between two labels (LabelB - LabelA).
type ComparisonDelta struct {
	ScoreDelta               float64  `json:"scoreDelta"`
	RecoveryMsDelta          float64  `json:"recoveryMsDelta"`
	P95RecoveryMsDelta       *float64 `json:"p95RecoveryMsDelta,omitempty"`
	SafetyInterventionsDelta int      `json:"safetyInterventionsDelta"`
	AvailabilityDelta        *float64 `json:"availabilityDelta,omitempty"`
	DowntimeMsDelta          *float64 `json:"downtimeMsDelta,omitempty"`
}

// ComparisonResult encapsulates the before/after comparison between labelA and labelB.
type ComparisonResult struct {
	LabelA            LabelStats      `json:"labelA"`
	LabelB            LabelStats      `json:"labelB"`
	Difference        ComparisonDelta `json:"difference"`
	Warning           string          `json:"warning,omitempty"`
	HasSufficientData bool            `json:"hasSufficientData"`
}

// GetExperimentsByLabel queries non-seeded experiments matching a given label and optional action.
func (r *PostgresRepository) GetExperimentsByLabel(ctx context.Context, label string, action string) ([]ExperimentResult, error) {
	query := `
		SELECT id, experiment_type, target_namespace, recovery_time_ms, 
		       error_rate_percent, safety_intervened, resilience_score, postmortem_report, created_at, seeded, label,
		       probe_requests, probe_failures, probe_downtime_ms
		FROM experiments
		WHERE seeded = FALSE AND label = $1
	`
	args := []interface{}{label}

	if action != "" {
		query += " AND experiment_type = $2"
		args = append(args, action)
	}

	query += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query experiments by label %q: %w", label, err)
	}
	defer rows.Close()

	var results []ExperimentResult
	for rows.Next() {
		var res ExperimentResult
		if err := rows.Scan(
			&res.ID,
			&res.ExperimentType,
			&res.TargetNamespace,
			&res.RecoveryTimeMs,
			&res.ErrorRatePercent,
			&res.SafetyIntervened,
			&res.ResilienceScore,
			&res.PostmortemReport,
			&res.CreatedAt,
			&res.Seeded,
			&res.Label,
			&res.ProbeRequests,
			&res.ProbeFailures,
			&res.ProbeDowntimeMs,
		); err != nil {
			return nil, fmt.Errorf("failed to scan experiment row: %w", err)
		}
		results = append(results, res)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return results, nil
}

// CalculateLabelStats aggregates statistics from a slice of experiment results.
func CalculateLabelStats(label string, experiments []ExperimentResult) LabelStats {
	stats := LabelStats{
		Label:    label,
		RunCount: len(experiments),
	}

	if len(experiments) == 0 {
		return stats
	}

	totalScore := 0
	var recoveryTimes []int
	safetyCount := 0
	var (
		totalAvail    float64
		totalDowntime int
		probeCount    int
	)

	for _, exp := range experiments {
		totalScore += exp.ResilienceScore
		if exp.RecoveryTimeMs != nil {
			recoveryTimes = append(recoveryTimes, *exp.RecoveryTimeMs)
		}
		if exp.SafetyIntervened {
			safetyCount++
		}
		if exp.ProbeRequests != nil && *exp.ProbeRequests > 0 {
			probeCount++
			failed := 0
			if exp.ProbeFailures != nil {
				failed = *exp.ProbeFailures
			}
			avail := (float64(*exp.ProbeRequests-failed) / float64(*exp.ProbeRequests)) * 100.0
			totalAvail += avail
			if exp.ProbeDowntimeMs != nil {
				totalDowntime += *exp.ProbeDowntimeMs
			}
		}
	}

	stats.AvgScore = math.Round((float64(totalScore)/float64(len(experiments)))*10) / 10
	stats.SafetyInterventions = safetyCount

	if probeCount > 0 {
		avgAvail := math.Round((totalAvail/float64(probeCount))*10) / 10
		avgDown := math.Round((float64(totalDowntime)/float64(probeCount))*10) / 10
		stats.AvgAvailability = &avgAvail
		stats.AvgDowntimeMs = &avgDown
	}

	if len(recoveryTimes) > 0 {
		sumRecovery := 0
		for _, rt := range recoveryTimes {
			sumRecovery += rt
		}
		stats.AvgRecoveryMs = math.Round((float64(sumRecovery)/float64(len(recoveryTimes)))*10) / 10

		// p95 recovery time only if 5+ runs with recovery times
		if len(recoveryTimes) >= 5 {
			sorted := make([]int, len(recoveryTimes))
			copy(sorted, recoveryTimes)
			sort.Ints(sorted)

			rank := int(math.Ceil(0.95*float64(len(sorted)))) - 1
			if rank < 0 {
				rank = 0
			}
			if rank >= len(sorted) {
				rank = len(sorted) - 1
			}
			p95 := float64(sorted[rank])
			stats.P95RecoveryMs = &p95
		}
	}

	return stats
}

// CompareLabels executes a before/after comparison between labelA and labelB.
func (r *PostgresRepository) CompareLabels(ctx context.Context, labelA, labelB, action string) (*ComparisonResult, error) {
	runsA, err := r.GetExperimentsByLabel(ctx, labelA, action)
	if err != nil {
		return nil, err
	}
	if len(runsA) == 0 {
		return nil, fmt.Errorf("no real experiment data found for label %q", labelA)
	}

	runsB, err := r.GetExperimentsByLabel(ctx, labelB, action)
	if err != nil {
		return nil, err
	}
	if len(runsB) == 0 {
		return nil, fmt.Errorf("no real experiment data found for label %q", labelB)
	}

	statsA := CalculateLabelStats(labelA, runsA)
	statsB := CalculateLabelStats(labelB, runsB)

	delta := ComparisonDelta{
		ScoreDelta:               math.Round((statsB.AvgScore-statsA.AvgScore)*10) / 10,
		RecoveryMsDelta:          math.Round((statsB.AvgRecoveryMs-statsA.AvgRecoveryMs)*10) / 10,
		SafetyInterventionsDelta: statsB.SafetyInterventions - statsA.SafetyInterventions,
	}

	if statsA.P95RecoveryMs != nil && statsB.P95RecoveryMs != nil {
		p95Delta := math.Round((*statsB.P95RecoveryMs-*statsA.P95RecoveryMs)*10) / 10
		delta.P95RecoveryMsDelta = &p95Delta
	}

	if statsA.AvgAvailability != nil && statsB.AvgAvailability != nil {
		availDelta := math.Round((*statsB.AvgAvailability-*statsA.AvgAvailability)*10) / 10
		delta.AvailabilityDelta = &availDelta
	}

	if statsA.AvgDowntimeMs != nil && statsB.AvgDowntimeMs != nil {
		dtDelta := math.Round((*statsB.AvgDowntimeMs-*statsA.AvgDowntimeMs)*10) / 10
		delta.DowntimeMsDelta = &dtDelta
	}

	var warning string
	hasSufficient := true
	if statsA.RunCount < 3 || statsB.RunCount < 3 {
		hasSufficient = false
		warning = fmt.Sprintf("sample size is too small for reliable comparison (label %q has %d runs, label %q has %d runs; minimum recommended is 3 runs per label)", labelA, statsA.RunCount, labelB, statsB.RunCount)
	}

	return &ComparisonResult{
		LabelA:            statsA,
		LabelB:            statsB,
		Difference:        delta,
		Warning:           warning,
		HasSufficientData: hasSufficient,
	}, nil
}
