-- Create calls with an Idempotency-Key. A repeat with the same key and request
-- returns the resource the first call created; the same key with another request is a conflict.
CREATE TABLE IF NOT EXISTS idempotency_keys (
    operation TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    key TEXT NOT NULL,
    request_hash BYTEA NOT NULL,
    resource_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (operation, actor_id, key)
);

CREATE INDEX IF NOT EXISTS idempotency_keys_created_at_idx ON idempotency_keys (created_at);
