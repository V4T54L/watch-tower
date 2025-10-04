-- Drop indexes (optional but cleaner)
DROP INDEX IF EXISTS idx_alert_rules_tenant_id;
DROP INDEX IF EXISTS idx_s3_chunks_tenant_id_ts_range;
DROP INDEX IF EXISTS idx_logs_tenant_id_timestamp;
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_tenant_id;
DROP INDEX IF EXISTS idx_tenants_api_key;

-- Drop tables
DROP TABLE IF EXISTS "alert_rules";
DROP TABLE IF EXISTS "s3_chunks";
DROP TABLE IF EXISTS "logs";
DROP TABLE IF EXISTS "users";
DROP TABLE IF EXISTS "tenants";

-- Drop enum type
DROP TYPE IF EXISTS user_role;

-- Drop extension
DROP EXTENSION IF EXISTS "uuid-ossp";
