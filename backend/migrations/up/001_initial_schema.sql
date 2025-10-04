-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Tenants Table
CREATE TABLE "tenants" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    "name" VARCHAR(255) NOT NULL,
    "api_key" VARCHAR(255) NOT NULL UNIQUE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tenants_api_key ON "tenants" ("api_key");

-- Users Table
CREATE TYPE user_role AS ENUM ('admin', 'viewer', 'support');

CREATE TABLE "users" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    "tenant_id" UUID NOT NULL REFERENCES "tenants" ("id") ON DELETE CASCADE,
    "email" VARCHAR(255) NOT NULL UNIQUE,
    "password_hash" VARCHAR(255) NOT NULL,
    "role" user_role NOT NULL DEFAULT 'viewer',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_tenant_id ON "users" ("tenant_id");

CREATE INDEX idx_users_email ON "users" ("email");

-- Logs Table (Hot Storage)
CREATE TABLE "logs" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    "tenant_id" UUID NOT NULL REFERENCES "tenants" ("id") ON DELETE CASCADE,
    "timestamp" TIMESTAMPTZ NOT NULL,
    "severity" VARCHAR(50),
    "service" VARCHAR(50),
    "message" TEXT
);

-- This index is crucial for querying logs by tenant and time range.
CREATE INDEX idx_logs_tenant_id_timestamp ON "logs" ("tenant_id", "timestamp" DESC);

-- S3 Chunks Metadata Table (for Cold Storage)
CREATE TABLE "s3_chunks" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    "tenant_id" UUID NOT NULL,
    "s3_key" VARCHAR(1024) NOT NULL UNIQUE,
    "start_ts" TIMESTAMPTZ NOT NULL,
    "end_ts" TIMESTAMPTZ NOT NULL,
    "record_count" BIGINT NOT NULL,
    "bloom_filter" BYTEA,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_s3_chunks_tenant_id_ts_range ON "s3_chunks" (
    "tenant_id",
    "start_ts",
    "end_ts"
);

-- Alert Rules Table
CREATE TABLE "alert_rules" (
    "id" UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    "tenant_id" UUID NOT NULL REFERENCES "tenants" ("id") ON DELETE CASCADE,
    "name" VARCHAR(255) NOT NULL,
    "description" TEXT,
    "query" TEXT NOT NULL,
    "threshold" INT NOT NULL,
    "interval_seconds" INT NOT NULL,
    "notification_channel" VARCHAR(255) NOT NULL,
    "is_enabled" BOOLEAN NOT NULL DEFAULT TRUE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alert_rules_tenant_id ON "alert_rules" ("tenant_id");