DROP TABLE IF EXISTS idempotency_keys;
ALTER TABLE outbox DROP COLUMN IF EXISTS trace_context;
