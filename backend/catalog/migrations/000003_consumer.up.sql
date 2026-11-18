-- Events already applied by the Kafka consumers, for idempotent redelivery.
CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Users whose UserDeleted event was applied. New tracks for them are refused,
-- which closes the race between a delete and an in-flight upload.
CREATE TABLE IF NOT EXISTS deleted_users (
    user_id UUID PRIMARY KEY,
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
