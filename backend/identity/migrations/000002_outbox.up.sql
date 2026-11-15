CREATE TABLE IF NOT EXISTS outbox (
    id BIGSERIAL PRIMARY KEY,
    topic TEXT NOT NULL,
    record_key TEXT NOT NULL,
    event_type TEXT NOT NULL,
    event_id UUID NOT NULL UNIQUE,
    payload BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS outbox_unsent_idx ON outbox (id) WHERE sent_at IS NULL;
CREATE INDEX IF NOT EXISTS outbox_sent_at_idx ON outbox (sent_at) WHERE sent_at IS NOT NULL;
