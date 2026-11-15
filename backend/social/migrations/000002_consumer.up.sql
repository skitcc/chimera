-- Events already applied by the Kafka consumer, for idempotent redelivery.
CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Users and tracks whose deletion was applied. New likes, follows, playlists
-- and playlist entries for them are refused, so a write that raced the event
-- cannot leave an orphan row behind.
CREATE TABLE IF NOT EXISTS deleted_users (
    user_id UUID PRIMARY KEY,
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS deleted_tracks (
    track_id UUID PRIMARY KEY,
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
