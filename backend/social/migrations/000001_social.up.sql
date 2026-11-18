-- user_id, track_id and owner_id have no foreign keys: users live in identity
-- and tracks in catalog. UserDeleted and TrackDeleted from Kafka remove the rows.
CREATE TABLE IF NOT EXISTS track_likes (
    user_id UUID NOT NULL,
    track_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, track_id)
);

CREATE TABLE IF NOT EXISTS user_follows (
    follower_id UUID NOT NULL,
    following_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, following_id),
    CONSTRAINT user_follows_not_self CHECK (follower_id <> following_id)
);

CREATE TABLE IF NOT EXISTS playlists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL,
    title TEXT NOT NULL,
    is_public BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS playlist_tracks (
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    track_id UUID NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (playlist_id, track_id)
);

CREATE INDEX IF NOT EXISTS track_likes_user_created_idx ON track_likes (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS track_likes_track_idx ON track_likes (track_id);
CREATE INDEX IF NOT EXISTS user_follows_follower_created_idx ON user_follows (follower_id, created_at DESC);
CREATE INDEX IF NOT EXISTS user_follows_following_created_idx ON user_follows (following_id, created_at DESC);
CREATE INDEX IF NOT EXISTS playlists_owner_created_idx ON playlists (owner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS playlists_owner_public_created_idx ON playlists (owner_id, is_public, created_at DESC);
CREATE INDEX IF NOT EXISTS playlist_tracks_playlist_position_idx ON playlist_tracks (playlist_id, position);
CREATE INDEX IF NOT EXISTS playlist_tracks_track_idx ON playlist_tracks (track_id);

-- Removing an entry closes the gap, so positions stay 0..n-1. Rows removed by
-- the playlist cascade are skipped.
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
