DROP TRIGGER IF EXISTS playlist_tracks_compact_after_delete ON playlist_tracks;
DROP FUNCTION IF EXISTS compact_playlist_track_positions();
DROP TABLE IF EXISTS playlist_tracks;
DROP TABLE IF EXISTS playlists;
DROP TABLE IF EXISTS user_follows;
DROP TABLE IF EXISTS track_likes;
