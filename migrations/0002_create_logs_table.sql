CREATE TABLE IF NOT EXISTS logs (
    event_id UUID PRIMARY KEY,
    received_at TIMESTAMPTZ NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_logs_event_time ON logs (event_time);
CREATE INDEX IF NOT EXISTS idx_logs_source ON logs (source);
CREATE INDEX IF NOT EXISTS idx_logs_level ON logs (level);
