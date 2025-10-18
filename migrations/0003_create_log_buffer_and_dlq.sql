CREATE TABLE IF NOT EXISTS log_buffer (
    id UUID PRIMARY KEY,
    received_at TIMESTAMPTZ NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    consumer_group TEXT,
    acknowledged BOOLEAN NOT NULL DEFAULT FALSE,
    retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_log_buffer_ack ON log_buffer (acknowledged, consumer_group);

-- Dead-letter queue for failed logs
CREATE TABLE IF NOT EXISTS log_dlq (
    id UUID PRIMARY KEY,
    failed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reason TEXT,
    event_time TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb
);
