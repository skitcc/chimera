-- user_id has no foreign key: users live in the identity database.
-- UserDeleted from Kafka removes a user's tracks.
CREATE TABLE IF NOT EXISTS tracks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    title TEXT NOT NULL,
    artist TEXT NOT NULL DEFAULT '',
    object_key TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS tracks_user_id_idx ON tracks (user_id);
CREATE INDEX IF NOT EXISTS tracks_status_created_at_idx ON tracks (status, created_at);
