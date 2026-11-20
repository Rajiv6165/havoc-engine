ALTER TABLE experiments ADD COLUMN IF NOT EXISTS probe_requests INTEGER;
ALTER TABLE experiments ADD COLUMN IF NOT EXISTS probe_failures INTEGER;
ALTER TABLE experiments ADD COLUMN IF NOT EXISTS probe_downtime_ms INTEGER;
