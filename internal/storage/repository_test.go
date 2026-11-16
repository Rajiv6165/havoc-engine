package storage

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func setupTestDB(t *testing.T, runMigrations bool) (*PostgresRepository, func()) {
	return SetupIsolatedTestDB(t, runMigrations)
}

func TestMigrations_AppliedOnceAndTracked(t *testing.T) {
	repo, cleanup := setupTestDB(t, false)
	defer cleanup()

	ctx := context.Background()

	// 1. Run migrations
	if err := repo.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// 2. Discover expected migration files in embedded FS
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		t.Fatalf("failed to read migrations dir: %v", err)
	}

	var expectedFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			expectedFiles = append(expectedFiles, entry.Name())
		}
	}
	sort.Strings(expectedFiles)

	// 3. Query schema_migrations table
	rows, err := repo.db.QueryContext(ctx, "SELECT version, applied_at FROM schema_migrations ORDER BY version ASC")
	if err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}
	defer rows.Close()

	appliedMigrations := make(map[string]time.Time)
	var migrationCount int
	for rows.Next() {
		var version string
		var appliedAt time.Time
		if err := rows.Scan(&version, &appliedAt); err != nil {
			t.Fatalf("failed to scan schema_migrations row: %v", err)
		}
		if _, exists := appliedMigrations[version]; exists {
			t.Errorf("migration %s was applied more than once", version)
		}
		appliedMigrations[version] = appliedAt
		migrationCount++
	}

	// Verify every expected file was applied
	for _, f := range expectedFiles {
		appliedAt, found := appliedMigrations[f]
		if !found {
			t.Errorf("expected migration %s was not recorded in schema_migrations", f)
		}
		if appliedAt.IsZero() {
			t.Errorf("migration %s has zero applied_at timestamp", f)
		}
	}

	// 4. Test idempotency: calling RunMigrations again should not fail or duplicate entries
	if err := repo.RunMigrations(ctx); err != nil {
		t.Fatalf("subsequent RunMigrations failed: %v", err)
	}

	var newCount int
	err = repo.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&newCount)
	if err != nil {
		t.Fatalf("failed to count schema_migrations: %v", err)
	}
	if newCount != migrationCount {
		t.Errorf("expected migration count to remain %d, got %d", migrationCount, newCount)
	}
}

func TestSaveAndFetchExperimentResults(t *testing.T) {
	repo, cleanup := setupTestDB(t, true)
	defer cleanup()

	ctx := context.Background()
	if err := repo.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)

	recTime1 := 2500
	errRate1 := 1.25
	report1 := "Sample postmortem report for live run"
	label1 := "baseline"
	liveExp := &ExperimentResult{
		ID:               uuid.New(),
		ExperimentType:   "kill-pod",
		TargetNamespace:  "test-live-ns",
		RecoveryTimeMs:   &recTime1,
		ErrorRatePercent: &errRate1,
		SafetyIntervened: false,
		ResilienceScore:  92,
		PostmortemReport: &report1,
		CreatedAt:        now.Add(-2 * time.Minute),
		Seeded:           false,
		Label:            &label1,
	}

	recTime2 := 4500
	errRate2 := 4.50
	report2 := "Sample postmortem report for seeded run"
	seededExp := &ExperimentResult{
		ID:               uuid.New(),
		ExperimentType:   "spike-cpu",
		TargetNamespace:  "test-seeded-ns",
		RecoveryTimeMs:   &recTime2,
		ErrorRatePercent: &errRate2,
		SafetyIntervened: true,
		ResilienceScore:  65,
		PostmortemReport: &report2,
		CreatedAt:        now.Add(-1 * time.Minute),
		Seeded:           true,
	}

	// 1. Save results
	if err := repo.SaveExperimentResult(ctx, liveExp); err != nil {
		t.Fatalf("SaveExperimentResult(liveExp) failed: %v", err)
	}
	if err := repo.SaveExperimentResult(ctx, seededExp); err != nil {
		t.Fatalf("SaveExperimentResult(seededExp) failed: %v", err)
	}

	// 2. Fetch results
	recent, err := repo.GetRecentExperiments(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentExperiments failed: %v", err)
	}

	var foundLive, foundSeeded *ExperimentResult
	for i := range recent {
		if recent[i].ID == liveExp.ID {
			foundLive = &recent[i]
		}
		if recent[i].ID == seededExp.ID {
			foundSeeded = &recent[i]
		}
	}

	if foundLive == nil {
		t.Fatalf("live experiment with ID %s not found in recent experiments", liveExp.ID)
	}
	if foundLive.Seeded != false {
		t.Errorf("expected liveExp.Seeded to be false, got %v", foundLive.Seeded)
	}
	if foundLive.ExperimentType != "kill-pod" {
		t.Errorf("expected ExperimentType 'kill-pod', got %q", foundLive.ExperimentType)
	}
	if foundLive.TargetNamespace != "test-live-ns" {
		t.Errorf("expected TargetNamespace 'test-live-ns', got %q", foundLive.TargetNamespace)
	}
	if foundLive.ResilienceScore != 92 {
		t.Errorf("expected ResilienceScore 92, got %d", foundLive.ResilienceScore)
	}
	if foundLive.PostmortemReport == nil || *foundLive.PostmortemReport != report1 {
		t.Errorf("postmortem report mismatch: %v", foundLive.PostmortemReport)
	}
	if foundLive.Label == nil || *foundLive.Label != label1 {
		t.Errorf("expected liveExp.Label to be %q, got %v", label1, foundLive.Label)
	}

	if foundSeeded == nil {
		t.Fatalf("seeded experiment with ID %s not found in recent experiments", seededExp.ID)
	}
	if foundSeeded.Label != nil {
		t.Errorf("expected seededExp.Label to be nil, got %v", *foundSeeded.Label)
	}
	if foundSeeded.Seeded != true {
		t.Errorf("expected seededExp.Seeded to be true, got %v", foundSeeded.Seeded)
	}
	if foundSeeded.SafetyIntervened != true {
		t.Errorf("expected SafetyIntervened true, got %v", foundSeeded.SafetyIntervened)
	}
	if foundSeeded.ResilienceScore != 65 {
		t.Errorf("expected ResilienceScore 65, got %d", foundSeeded.ResilienceScore)
	}

	// 3. Test GetScoreHistory
	scores, err := repo.GetScoreHistory(ctx, 10)
	if err != nil {
		t.Fatalf("GetScoreHistory failed: %v", err)
	}
	if len(scores) < 2 {
		t.Fatalf("expected at least 2 scores in history, got %d", len(scores))
	}
}

func TestSchedulerAuditLog(t *testing.T) {
	repo, cleanup := setupTestDB(t, true)
	defer cleanup()

	ctx := context.Background()
	if err := repo.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// 1. Test pause / resume state
	if err := repo.SetSchedulerState(ctx, true); err != nil {
		t.Fatalf("SetSchedulerState(true) failed: %v", err)
	}
	paused, err := repo.GetSchedulerState(ctx)
	if err != nil {
		t.Fatalf("GetSchedulerState failed: %v", err)
	}
	if !paused {
		t.Errorf("expected scheduler to be paused, got %v", paused)
	}

	if err := repo.SetSchedulerState(ctx, false); err != nil {
		t.Fatalf("SetSchedulerState(false) failed: %v", err)
	}
	paused, err = repo.GetSchedulerState(ctx)
	if err != nil {
		t.Fatalf("GetSchedulerState failed: %v", err)
	}
	if paused {
		t.Errorf("expected scheduler to not be paused, got %v", paused)
	}

	// 2. Log scheduled runs
	testExpFile := "experiments/test-exp-" + uuid.New().String()[:8] + ".yaml"
	if err := repo.LogScheduledRun(ctx, "success", testExpFile); err != nil {
		t.Fatalf("LogScheduledRun(success) failed: %v", err)
	}

	testExpFile2 := "experiments/test-skip-" + uuid.New().String()[:8] + ".yaml"
	if err := repo.LogScheduledRun(ctx, "skipped_paused", testExpFile2); err != nil {
		t.Fatalf("LogScheduledRun(skipped_paused) failed: %v", err)
	}

	// 3. Fetch recent scheduled runs
	runs, err := repo.GetRecentScheduledRuns(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentScheduledRuns failed: %v", err)
	}

	var foundSuccess, foundSkipped *ScheduledRun
	for i := range runs {
		if runs[i].ExperimentFile == testExpFile {
			foundSuccess = &runs[i]
		}
		if runs[i].ExperimentFile == testExpFile2 {
			foundSkipped = &runs[i]
		}
	}

	if foundSuccess == nil {
		t.Fatalf("scheduled run for %s not found", testExpFile)
	}
	if foundSuccess.Outcome != "success" {
		t.Errorf("expected outcome 'success', got %q", foundSuccess.Outcome)
	}

	if foundSkipped == nil {
		t.Fatalf("scheduled run for %s not found", testExpFile2)
	}
	if foundSkipped.Outcome != "skipped_paused" {
		t.Errorf("expected outcome 'skipped_paused', got %q", foundSkipped.Outcome)
	}
}

func TestCalculateLabelStats_Unit(t *testing.T) {
	// 1. Empty runs
	emptyStats := CalculateLabelStats("empty", nil)
	if emptyStats.RunCount != 0 || emptyStats.AvgScore != 0 || emptyStats.AvgRecoveryMs != 0 || emptyStats.P95RecoveryMs != nil {
		t.Errorf("unexpected stats for empty runs: %+v", emptyStats)
	}

	// 2. 5 runs for p95 calculation and correct averages
	rec1, rec2, rec3, rec4, rec5 := 1000, 2000, 3000, 4000, 5000
	runs := []ExperimentResult{
		{ResilienceScore: 60, RecoveryTimeMs: &rec1, SafetyIntervened: false},
		{ResilienceScore: 70, RecoveryTimeMs: &rec2, SafetyIntervened: false},
		{ResilienceScore: 80, RecoveryTimeMs: &rec3, SafetyIntervened: true},
		{ResilienceScore: 90, RecoveryTimeMs: &rec4, SafetyIntervened: false},
		{ResilienceScore: 100, RecoveryTimeMs: &rec5, SafetyIntervened: false},
	}
	stats := CalculateLabelStats("test-p95", runs)
	if stats.RunCount != 5 {
		t.Errorf("expected RunCount 5, got %d", stats.RunCount)
	}
	if stats.AvgScore != 80.0 {
		t.Errorf("expected AvgScore 80.0, got %f", stats.AvgScore)
	}
	if stats.AvgRecoveryMs != 3000.0 {
		t.Errorf("expected AvgRecoveryMs 3000.0, got %f", stats.AvgRecoveryMs)
	}
	if stats.P95RecoveryMs == nil || *stats.P95RecoveryMs != 5000.0 {
		t.Errorf("expected P95RecoveryMs 5000.0, got %v", stats.P95RecoveryMs)
	}
	if stats.SafetyInterventions != 1 {
		t.Errorf("expected SafetyInterventions 1, got %d", stats.SafetyInterventions)
	}

	// 3. 2 runs (too few for p95, p95 must be nil)
	smallRuns := []ExperimentResult{
		{ResilienceScore: 50, RecoveryTimeMs: &rec1},
		{ResilienceScore: 70, RecoveryTimeMs: &rec2},
	}
	smallStats := CalculateLabelStats("small", smallRuns)
	if smallStats.RunCount != 2 {
		t.Errorf("expected RunCount 2, got %d", smallStats.RunCount)
	}
	if smallStats.P95RecoveryMs != nil {
		t.Errorf("expected P95RecoveryMs nil for <5 runs, got %v", smallStats.P95RecoveryMs)
	}
}

func TestCompareLabels_Storage(t *testing.T) {
	repo, cleanup := setupTestDB(t, true)
	defer cleanup()
	ctx := context.Background()

	labelA := fmt.Sprintf("baseline-%d", time.Now().UnixNano())
	labelB := fmt.Sprintf("hardened-%d", time.Now().UnixNano())
	labelSmall := fmt.Sprintf("small-%d", time.Now().UnixNano())

	now := time.Now()

	// Insert 5 baseline runs (seeded=false)
	// scores: 50, 60, 70, 80, 90 => avg = 70.0
	// recoveries: 1000, 2000, 3000, 4000, 5000 => avg = 3000.0, p95 = 5000.0
	for i, score := range []int{50, 60, 70, 80, 90} {
		rec := (i + 1) * 1000
		intervened := (i == 0) // 1 safety intervention
		err := repo.SaveExperimentResult(ctx, &ExperimentResult{
			ID:               uuid.New(),
			ExperimentType:   "kill-pod",
			TargetNamespace:  "demo",
			RecoveryTimeMs:   &rec,
			SafetyIntervened: intervened,
			ResilienceScore:  score,
			CreatedAt:        now.Add(time.Duration(i) * time.Minute),
			Seeded:           false,
			Label:            &labelA,
		})
		if err != nil {
			t.Fatalf("failed to insert baseline experiment: %v", err)
		}
	}

	// Insert 1 baseline run with seeded = TRUE (must be EXCLUDED from comparison!)
	recSeeded := 100
	err := repo.SaveExperimentResult(ctx, &ExperimentResult{
		ID:               uuid.New(),
		ExperimentType:   "kill-pod",
		TargetNamespace:  "demo",
		RecoveryTimeMs:   &recSeeded,
		SafetyIntervened: false,
		ResilienceScore:  100, // Would skew average if not excluded
		CreatedAt:        now.Add(10 * time.Minute),
		Seeded:           true,
		Label:            &labelA,
	})
	if err != nil {
		t.Fatalf("failed to insert seeded baseline experiment: %v", err)
	}

	// Insert 5 hardened runs (seeded=false)
	// scores: 85, 90, 95, 95, 100 => avg = 93.0
	// recoveries: 500, 600, 700, 800, 900 => avg = 700.0, p95 = 900.0
	for i, score := range []int{85, 90, 95, 95, 100} {
		rec := 500 + (i * 100)
		err := repo.SaveExperimentResult(ctx, &ExperimentResult{
			ID:               uuid.New(),
			ExperimentType:   "kill-pod",
			TargetNamespace:  "demo",
			RecoveryTimeMs:   &rec,
			SafetyIntervened: false,
			ResilienceScore:  score,
			CreatedAt:        now.Add(time.Duration(i+20) * time.Minute),
			Seeded:           false,
			Label:            &labelB,
		})
		if err != nil {
			t.Fatalf("failed to insert hardened experiment: %v", err)
		}
	}

	// Insert 2 small-sample runs (seeded=false)
	for i, score := range []int{75, 85} {
		rec := 1500
		err := repo.SaveExperimentResult(ctx, &ExperimentResult{
			ID:               uuid.New(),
			ExperimentType:   "kill-pod",
			TargetNamespace:  "demo",
			RecoveryTimeMs:   &rec,
			SafetyIntervened: false,
			ResilienceScore:  score,
			CreatedAt:        now.Add(time.Duration(i+40) * time.Minute),
			Seeded:           false,
			Label:            &labelSmall,
		})
		if err != nil {
			t.Fatalf("failed to insert small sample experiment: %v", err)
		}
	}

	// 1. Compare baseline vs hardened (5 runs each, seeded row excluded)
	res, err := repo.CompareLabels(ctx, labelA, labelB, "")
	if err != nil {
		t.Fatalf("CompareLabels failed: %v", err)
	}

	if res.LabelA.RunCount != 5 {
		t.Errorf("expected labelA RunCount 5 (seeded row excluded), got %d", res.LabelA.RunCount)
	}
	if res.LabelA.AvgScore != 70.0 {
		t.Errorf("expected labelA AvgScore 70.0, got %f", res.LabelA.AvgScore)
	}
	if res.LabelA.AvgRecoveryMs != 3000.0 {
		t.Errorf("expected labelA AvgRecoveryMs 3000.0, got %f", res.LabelA.AvgRecoveryMs)
	}
	if res.LabelA.P95RecoveryMs == nil || *res.LabelA.P95RecoveryMs != 5000.0 {
		t.Errorf("expected labelA P95RecoveryMs 5000.0, got %v", res.LabelA.P95RecoveryMs)
	}
	if res.LabelA.SafetyInterventions != 1 {
		t.Errorf("expected labelA SafetyInterventions 1, got %d", res.LabelA.SafetyInterventions)
	}

	if res.LabelB.RunCount != 5 {
		t.Errorf("expected labelB RunCount 5, got %d", res.LabelB.RunCount)
	}
	if res.LabelB.AvgScore != 93.0 {
		t.Errorf("expected labelB AvgScore 93.0, got %f", res.LabelB.AvgScore)
	}
	if res.LabelB.AvgRecoveryMs != 700.0 {
		t.Errorf("expected labelB AvgRecoveryMs 700.0, got %f", res.LabelB.AvgRecoveryMs)
	}
	if res.LabelB.P95RecoveryMs == nil || *res.LabelB.P95RecoveryMs != 900.0 {
		t.Errorf("expected labelB P95RecoveryMs 900.0, got %v", res.LabelB.P95RecoveryMs)
	}

	// Differences
	if res.Difference.ScoreDelta != 23.0 {
		t.Errorf("expected ScoreDelta 23.0, got %f", res.Difference.ScoreDelta)
	}
	if res.Difference.RecoveryMsDelta != -2300.0 {
		t.Errorf("expected RecoveryMsDelta -2300.0, got %f", res.Difference.RecoveryMsDelta)
	}
	if res.Difference.P95RecoveryMsDelta == nil || *res.Difference.P95RecoveryMsDelta != -4100.0 {
		t.Errorf("expected P95RecoveryMsDelta -4100.0, got %v", res.Difference.P95RecoveryMsDelta)
	}
	if res.Difference.SafetyInterventionsDelta != -1 {
		t.Errorf("expected SafetyInterventionsDelta -1, got %d", res.Difference.SafetyInterventionsDelta)
	}
	if res.Warning != "" {
		t.Errorf("expected no warning for 5 runs each, got %q", res.Warning)
	}
	if !res.HasSufficientData {
		t.Errorf("expected HasSufficientData to be true")
	}

	// 2. Test too-few-runs warning (< 3 runs)
	resSmall, err := repo.CompareLabels(ctx, labelSmall, labelB, "")
	if err != nil {
		t.Fatalf("CompareLabels with small sample failed: %v", err)
	}
	if resSmall.HasSufficientData {
		t.Errorf("expected HasSufficientData to be false for small sample")
	}
	if resSmall.Warning == "" || !strings.Contains(resSmall.Warning, "sample size is too small") {
		t.Errorf("expected warning about small sample, got %q", resSmall.Warning)
	}

	// 3. Test missing / no data
	_, err = repo.CompareLabels(ctx, "nonexistent-label", labelB, "")
	if err == nil {
		t.Errorf("expected error when comparing nonexistent label, got nil")
	}
}
