package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"havoc-engine/internal/config"
	"havoc-engine/internal/storage"
)

type mockClock struct {
	t time.Time
}

func (m mockClock) Now() time.Time {
	return m.t
}

func TestScheduler_AllowedWindows(t *testing.T) {
	cfg := &config.Config{
		Scheduler: config.SchedulerConfig{
			Enabled: true,
			AllowedWindows: []config.TimeWindow{
				{
					Days:      []string{"Monday", "Wednesday", "Friday"},
					StartTime: "09:00",
					EndTime:   "17:00",
				},
			},
		},
	}
	
	sched := &Scheduler{cfg: cfg}

	tests := []struct {
		name     string
		time     time.Time
		expected bool
	}{
		{
			name:     "Monday 10am (allowed)",
			time:     time.Date(2023, 10, 2, 10, 0, 0, 0, time.UTC), // Oct 2 2023 is Monday
			expected: true,
		},
		{
			name:     "Monday 8am (too early)",
			time:     time.Date(2023, 10, 2, 8, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "Tuesday 10am (wrong day)",
			time:     time.Date(2023, 10, 3, 10, 0, 0, 0, time.UTC), // Tuesday
			expected: false,
		},
		{
			name:     "Friday 16:59 (allowed)",
			time:     time.Date(2023, 10, 6, 16, 59, 0, 0, time.UTC), // Friday
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sched.isInAllowedWindow(tc.time); got != tc.expected {
				t.Errorf("expected %v, got %v for time %v", tc.expected, got, tc.time)
			}
		})
	}
}

func TestScheduler_Tick(t *testing.T) {
	// 1. Setup a test database
	dsn := "postgres://postgres:postgres@localhost:5432/havoc?sslmode=disable"
	repo, err := storage.NewPostgresRepository(dsn)
	if err != nil {
		t.Skipf("Skipping integration test because DB is not available: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	_ = repo.RunMigrations(ctx)
	
	// Ensure not paused
	_ = repo.SetSchedulerState(ctx, false)

	// Create a dummy experiment dir
	tempDir := t.TempDir()
	expFile := filepath.Join(tempDir, "test-exp.yaml")
	err = os.WriteFile(expFile, []byte("name: test\naction: test-action"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Scheduler: config.SchedulerConfig{
			Enabled: true,
		},
	}

	// Always trigger rand > 0.05 by using a custom source if we really wanted to.
	// But since we can't easily mock rand.Float32() globally, we'll just test the pause state
	// which returns early and logs "skipped_paused".
	
	// We'll set it to paused to verify the audit log creates "skipped_paused"
	_ = repo.SetSchedulerState(ctx, true)
	
	sched := NewScheduler(cfg, repo, mockClock{time.Now()}, tempDir, nil)
	
	// Execute a single tick
	sched.tick(ctx)
	
	// Check audit log
	runs, err := repo.GetRecentScheduledRuns(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	
	if len(runs) == 0 {
		t.Fatalf("expected 1 run logged, got 0")
	}
	
	if runs[0].Outcome != "skipped_paused" {
		t.Errorf("expected outcome 'skipped_paused', got %q", runs[0].Outcome)
	}
}
