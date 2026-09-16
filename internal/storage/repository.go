package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// ExperimentResult represents the result of a chaos experiment.
type ExperimentResult struct {
	ID               uuid.UUID
	ExperimentType   string
	TargetNamespace  string
	RecoveryTimeMs   *int
	ErrorRatePercent *float64
	SafetyIntervened bool
	ResilienceScore  int
	PostmortemReport *string
	CreatedAt        time.Time
}

// ScheduledRun represents an execution attempt by the scheduler.
type ScheduledRun struct {
	ID             uuid.UUID
	Timestamp      time.Time
	ExperimentFile string
	Outcome        string
}

// ExperimentRepository defines the interface for interacting with experiment storage.
type ExperimentRepository interface {
	SaveExperimentResult(ctx context.Context, result *ExperimentResult) error
	GetRecentExperiments(ctx context.Context, limit int) ([]ExperimentResult, error)
	GetScoreHistory(ctx context.Context, limit int) ([]int, error)
	LogScheduledRun(ctx context.Context, outcome, experimentFile string) error
	GetRecentScheduledRuns(ctx context.Context, limit int) ([]ScheduledRun, error)
	GetSchedulerState(ctx context.Context) (bool, error)
	SetSchedulerState(ctx context.Context, paused bool) error
}

// PostgresRepository is a PostgreSQL implementation of ExperimentRepository.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository creates a new PostgresRepository.
func NewPostgresRepository(dsn string) (*PostgresRepository, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresRepository{db: db}, nil
}

// SaveExperimentResult saves an experiment result to the database.
func (r *PostgresRepository) SaveExperimentResult(ctx context.Context, result *ExperimentResult) error {
	query := `
		INSERT INTO experiments (
			id, experiment_type, target_namespace, recovery_time_ms, 
			error_rate_percent, safety_intervened, resilience_score, postmortem_report, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`
	_, err := r.db.ExecContext(ctx, query,
		result.ID,
		result.ExperimentType,
		result.TargetNamespace,
		result.RecoveryTimeMs,
		result.ErrorRatePercent,
		result.SafetyIntervened,
		result.ResilienceScore,
		result.PostmortemReport,
		result.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert experiment result: %w", err)
	}
	return nil
}

// GetRecentExperiments retrieves the most recent experiments up to the limit.
func (r *PostgresRepository) GetRecentExperiments(ctx context.Context, limit int) ([]ExperimentResult, error) {
	query := `
		SELECT id, experiment_type, target_namespace, recovery_time_ms, 
		       error_rate_percent, safety_intervened, resilience_score, postmortem_report, created_at
		FROM experiments
		ORDER BY created_at DESC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent experiments: %w", err)
	}
	defer rows.Close()

	var results []ExperimentResult
	for rows.Next() {
		var res ExperimentResult
		err := rows.Scan(
			&res.ID,
			&res.ExperimentType,
			&res.TargetNamespace,
			&res.RecoveryTimeMs,
			&res.ErrorRatePercent,
			&res.SafetyIntervened,
			&res.ResilienceScore,
			&res.PostmortemReport,
			&res.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan experiment result: %w", err)
		}
		results = append(results, res)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return results, nil
}

// GetScoreHistory retrieves the resilience scores of the most recent experiments.
func (r *PostgresRepository) GetScoreHistory(ctx context.Context, limit int) ([]int, error) {
	query := `
		SELECT resilience_score
		FROM experiments
		ORDER BY created_at DESC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query score history: %w", err)
	}
	defer rows.Close()

	var scores []int
	for rows.Next() {
		var score int
		if err := rows.Scan(&score); err != nil {
			return nil, fmt.Errorf("failed to scan score: %w", err)
		}
		scores = append(scores, score)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return scores, nil
}

// LogScheduledRun logs an execution attempt by the scheduler.
func (r *PostgresRepository) LogScheduledRun(ctx context.Context, outcome, experimentFile string) error {
	query := `
		INSERT INTO scheduled_runs (id, timestamp, experiment_file, outcome)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.db.ExecContext(ctx, query, uuid.New(), time.Now(), experimentFile, outcome)
	return err
}

// GetRecentScheduledRuns retrieves the most recent scheduled runs up to the limit.
func (r *PostgresRepository) GetRecentScheduledRuns(ctx context.Context, limit int) ([]ScheduledRun, error) {
	query := `
		SELECT id, timestamp, experiment_file, outcome
		FROM scheduled_runs
		ORDER BY timestamp DESC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query scheduled runs: %w", err)
	}
	defer rows.Close()

	var runs []ScheduledRun
	for rows.Next() {
		var run ScheduledRun
		if err := rows.Scan(&run.ID, &run.Timestamp, &run.ExperimentFile, &run.Outcome); err != nil {
			return nil, fmt.Errorf("failed to scan scheduled run: %w", err)
		}
		runs = append(runs, run)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return runs, nil
}

// GetSchedulerState retrieves the current pause state.
func (r *PostgresRepository) GetSchedulerState(ctx context.Context) (bool, error) {
	var isPaused bool
	err := r.db.QueryRowContext(ctx, "SELECT is_paused FROM scheduler_state WHERE id = 1").Scan(&isPaused)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return isPaused, err
}

// SetSchedulerState updates the pause state.
func (r *PostgresRepository) SetSchedulerState(ctx context.Context, paused bool) error {
	query := `
		INSERT INTO scheduler_state (id, is_paused) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET is_paused = EXCLUDED.is_paused
	`
	_, err := r.db.ExecContext(ctx, query, paused)
	return err
}

// RunMigrations runs the initialization SQL.
func (r *PostgresRepository) RunMigrations(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS experiments (
		id UUID PRIMARY KEY,
		experiment_type TEXT NOT NULL,
		target_namespace TEXT NOT NULL,
		recovery_time_ms INT,
		error_rate_percent FLOAT,
		safety_intervened BOOLEAN NOT NULL,
		resilience_score INT NOT NULL,
		postmortem_report TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}
	
	alterQuery := `ALTER TABLE experiments ADD COLUMN IF NOT EXISTS postmortem_report TEXT;`
	_, err = r.db.ExecContext(ctx, alterQuery)
	if err != nil {
		return err
	}

	schedulerQueries := `
	CREATE TABLE IF NOT EXISTS scheduled_runs (
		id UUID PRIMARY KEY,
		timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		experiment_file TEXT NOT NULL,
		outcome TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS scheduler_state (
		id INT PRIMARY KEY,
		is_paused BOOLEAN NOT NULL DEFAULT FALSE
	);
	
	INSERT INTO scheduler_state (id, is_paused) VALUES (1, FALSE) ON CONFLICT DO NOTHING;
	`
	_, err = r.db.ExecContext(ctx, schedulerQueries)
	return err
}

// Close closes the database connection.
func (r *PostgresRepository) Close() error {
	return r.db.Close()
}
