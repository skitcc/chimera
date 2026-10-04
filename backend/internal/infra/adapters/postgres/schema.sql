CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tracks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    artist TEXT NOT NULL DEFAULT '',
    object_key TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE tracks ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id);
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS object_key TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS size_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE tracks DROP CONSTRAINT IF EXISTS tracks_user_id_fkey;
ALTER TABLE tracks
    ADD CONSTRAINT tracks_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS track_likes (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, track_id)
);

CREATE TABLE IF NOT EXISTS user_follows (
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    following_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, following_id),
    CONSTRAINT user_follows_not_self CHECK (follower_id <> following_id)
);

CREATE TABLE IF NOT EXISTS playlists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    is_public BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS playlist_tracks (
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (playlist_id, track_id)
);

CREATE TABLE IF NOT EXISTS picks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pick_tracks (
    pick_id UUID NOT NULL REFERENCES picks(id) ON DELETE CASCADE,
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    note TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (pick_id, track_id)
);

CREATE INDEX IF NOT EXISTS tracks_user_id_idx ON tracks (user_id);
CREATE INDEX IF NOT EXISTS tracks_status_created_at_idx ON tracks (status, created_at);
CREATE INDEX IF NOT EXISTS track_likes_user_created_idx ON track_likes (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS user_follows_follower_created_idx ON user_follows (follower_id, created_at DESC);
CREATE INDEX IF NOT EXISTS user_follows_following_created_idx ON user_follows (following_id, created_at DESC);
CREATE INDEX IF NOT EXISTS playlists_owner_created_idx ON playlists (owner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS playlists_owner_public_created_idx ON playlists (owner_id, is_public, created_at DESC);
CREATE INDEX IF NOT EXISTS playlist_tracks_playlist_position_idx ON playlist_tracks (playlist_id, position);
CREATE INDEX IF NOT EXISTS picks_user_generated_idx ON picks (user_id, generated_at DESC);
CREATE INDEX IF NOT EXISTS pick_tracks_pick_position_idx ON pick_tracks (pick_id, position);

CREATE OR REPLACE FUNCTION compact_playlist_track_positions()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM playlists WHERE id = OLD.playlist_id) THEN
        UPDATE playlist_tracks
        SET position = position - 1
        WHERE playlist_id = OLD.playlist_id
          AND position > OLD.position;
    END IF;
    RETURN OLD;
END;
$$;

DROP TRIGGER IF EXISTS playlist_tracks_compact_after_delete ON playlist_tracks;
CREATE TRIGGER playlist_tracks_compact_after_delete
AFTER DELETE ON playlist_tracks
FOR EACH ROW
EXECUTE FUNCTION compact_playlist_track_positions();

WITH ranked AS (
    SELECT playlist_id, track_id,
           row_number() OVER (PARTITION BY playlist_id ORDER BY position, created_at, track_id) - 1 AS position
    FROM playlist_tracks
)
UPDATE playlist_tracks AS target
SET position = ranked.position
FROM ranked
WHERE target.playlist_id = ranked.playlist_id
  AND target.track_id = ranked.track_id
  AND target.position <> ranked.position;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'playlist_tracks_playlist_position_key'
          AND conrelid = 'playlist_tracks'::regclass
    ) THEN
        ALTER TABLE playlist_tracks
            ADD CONSTRAINT playlist_tracks_playlist_position_key
            UNIQUE (playlist_id, position)
            DEFERRABLE INITIALLY DEFERRED;
    END IF;
END;
$$;
