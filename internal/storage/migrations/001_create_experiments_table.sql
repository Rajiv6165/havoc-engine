CREATE TABLE IF NOT EXISTS experiments (
    id UUID PRIMARY KEY,
    experiment_type TEXT NOT NULL,
    target_namespace TEXT NOT NULL,
    recovery_time_ms INT,
    error_rate_percent FLOAT,
    safety_intervened BOOLEAN NOT NULL,
    resilience_score INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
