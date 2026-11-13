package storage

import (
	"context"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func setupTestDB(t *testing.T) (*PostgresRepository, func()) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/havoc?sslmode=disable"
	}
	repo, err := NewPostgresRepository(dsn)
	if err != nil {
		t.Skipf("Skipping storage tests because database is not available: %v", err)
	}

	cleanup := func() {
		_ = repo.Close()
	}
	return repo, cleanup
}

func TestMigrations_AppliedOnceAndTracked(t *testing.T) {
	repo, cleanup := setupTestDB(t)
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
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if err := repo.RunMigrations(ctx); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)

	recTime1 := 2500
	errRate1 := 1.25
	report1 := "Sample postmortem report for live run"
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

	if foundSeeded == nil {
		t.Fatalf("seeded experiment with ID %s not found in recent experiments", seededExp.ID)
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
	repo, cleanup := setupTestDB(t)
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
