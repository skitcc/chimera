package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"chimera/internal/domain"
)

const playlistCols = `id::text, owner_id::text, title, is_public`

type playlistDB interface {
	db
	Begin(context.Context) (pgx.Tx, error)
}

type PlaylistRepository struct {
	db playlistDB
}

func NewPlaylistRepository(db playlistDB) *PlaylistRepository {
	return &PlaylistRepository{db: db}
}

func scanPlaylist(row scanner) (domain.Playlist, error) {
	var playlist domain.Playlist
	err := row.Scan(&playlist.ID, &playlist.OwnerID, &playlist.Title, &playlist.IsPublic)
	return playlist, err
}

func (r *PlaylistRepository) Create(ctx context.Context, playlist domain.Playlist) (domain.Playlist, error) {
	playlist, err := scanPlaylist(r.db.QueryRow(ctx,
		`INSERT INTO playlists (owner_id, title, is_public)
		 VALUES ($1::uuid, $2, $3)
		 RETURNING `+playlistCols,
		playlist.OwnerID, playlist.Title, playlist.IsPublic,
	))
	if err != nil {
		return domain.Playlist{}, mapError(err, "create playlist")
	}
	return playlist, nil
}

func (r *PlaylistRepository) GetByID(ctx context.Context, id string) (domain.Playlist, error) {
	playlist, err := scanPlaylist(r.db.QueryRow(ctx,
		`SELECT `+playlistCols+` FROM playlists WHERE id = $1::uuid`, id,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Playlist{}, domain.NotFound("playlist not found")
	}
	if err != nil {
		return domain.Playlist{}, mapError(err, "get playlist")
	}
	return playlist, nil
}

func (r *PlaylistRepository) Update(ctx context.Context, playlist domain.Playlist) (domain.Playlist, error) {
	playlist, err := scanPlaylist(r.db.QueryRow(ctx,
		`UPDATE playlists SET title = $2, is_public = $3
		 WHERE id = $1::uuid
		 RETURNING `+playlistCols,
		playlist.ID, playlist.Title, playlist.IsPublic,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Playlist{}, domain.NotFound("playlist not found")
	}
	if err != nil {
		return domain.Playlist{}, mapError(err, "update playlist")
	}
	return playlist, nil
}

func (r *PlaylistRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM playlists WHERE id = $1::uuid`, id)
	if err != nil {
		return mapError(err, "delete playlist")
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("playlist not found")
	}
	return nil
}

func (r *PlaylistRepository) ListByOwner(ctx context.Context, ownerID string, publicOnly bool) ([]domain.Playlist, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+playlistCols+`
		 FROM playlists
		 WHERE owner_id = $1::uuid AND (NOT $2 OR is_public)
		 ORDER BY created_at DESC, id DESC`,
		ownerID, publicOnly,
	)
	if err != nil {
		return nil, mapError(err, "list playlists")
	}
	defer rows.Close()

	playlists := make([]domain.Playlist, 0)
	for rows.Next() {
		playlist, err := scanPlaylist(rows)
		if err != nil {
			return nil, mapError(err, "scan playlist")
		}
		playlists = append(playlists, playlist)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "list playlists")
	}
	return playlists, nil
}

func (r *PlaylistRepository) ListTracks(ctx context.Context, playlistID string) ([]domain.PlaylistTrack, error) {
	rows, err := r.db.Query(ctx,
		`SELECT pt.position, `+trackCols+`
		 FROM playlist_tracks pt
		 JOIN tracks t ON t.id = pt.track_id
		 WHERE pt.playlist_id = $1::uuid AND t.status = $2
		 ORDER BY pt.position`,
		playlistID, string(domain.TrackReady),
	)
	if err != nil {
		return nil, mapError(err, "list playlist tracks")
	}
	defer rows.Close()

	tracks := make([]domain.PlaylistTrack, 0)
	for rows.Next() {
		var item domain.PlaylistTrack
		var status string
		err := rows.Scan(
			&item.Position,
			&item.Track.ID,
			&item.Track.UserID,
			&item.Track.Title,
			&item.Track.Artist,
			&item.Track.ObjectKey,
			&item.Track.SizeBytes,
			&status,
		)
		if err != nil {
			return nil, mapError(err, "scan playlist track")
		}
		item.Track.Status = domain.TrackStatus(status)
		tracks = append(tracks, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "list playlist tracks")
	}
	return tracks, nil
}

func (r *PlaylistRepository) AddTrack(ctx context.Context, playlistID string, in domain.PlaylistTrackAdd) (domain.PlaylistTrack, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.PlaylistTrack{}, mapError(err, "begin add playlist track")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	count, err := lockPlaylist(ctx, tx, playlistID)
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	if in.Position > count {
		return domain.PlaylistTrack{}, domain.Invalid("position exceeds playlist length")
	}
	if _, err := tx.Exec(ctx,
		`UPDATE playlist_tracks SET position = position + 1
		 WHERE playlist_id = $1::uuid AND position >= $2`,
		playlistID, in.Position,
	); err != nil {
		return domain.PlaylistTrack{}, mapError(err, "shift playlist tracks")
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO playlist_tracks (playlist_id, track_id, position)
		 VALUES ($1::uuid, $2::uuid, $3)`,
		playlistID, in.TrackID, in.Position,
	); err != nil {
		return domain.PlaylistTrack{}, mapError(err, "add playlist track")
	}
	item, err := getPlaylistTrack(ctx, tx, playlistID, in.TrackID)
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PlaylistTrack{}, mapError(err, "commit add playlist track")
	}
	return item, nil
}

func (r *PlaylistRepository) MoveTrack(ctx context.Context, playlistID, trackID string, position int) (domain.PlaylistTrack, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.PlaylistTrack{}, mapError(err, "begin move playlist track")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	count, err := lockPlaylist(ctx, tx, playlistID)
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	var old int
	err = tx.QueryRow(ctx,
		`SELECT position FROM playlist_tracks
		 WHERE playlist_id = $1::uuid AND track_id = $2::uuid`,
		playlistID, trackID,
	).Scan(&old)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PlaylistTrack{}, domain.NotFound("playlist track not found")
	}
	if err != nil {
		return domain.PlaylistTrack{}, mapError(err, "get playlist track position")
	}
	if position >= count {
		return domain.PlaylistTrack{}, domain.Invalid("position exceeds playlist length")
	}
	if position < old {
		_, err = tx.Exec(ctx,
			`UPDATE playlist_tracks SET position = position + 1
			 WHERE playlist_id = $1::uuid AND track_id <> $2::uuid
			   AND position >= $3 AND position < $4`,
			playlistID, trackID, position, old,
		)
	} else if position > old {
		_, err = tx.Exec(ctx,
			`UPDATE playlist_tracks SET position = position - 1
			 WHERE playlist_id = $1::uuid AND track_id <> $2::uuid
			   AND position > $3 AND position <= $4`,
			playlistID, trackID, old, position,
		)
	}
	if err != nil {
		return domain.PlaylistTrack{}, mapError(err, "shift playlist tracks")
	}
	if _, err := tx.Exec(ctx,
		`UPDATE playlist_tracks SET position = $3
		 WHERE playlist_id = $1::uuid AND track_id = $2::uuid`,
		playlistID, trackID, position,
	); err != nil {
		return domain.PlaylistTrack{}, mapError(err, "move playlist track")
	}
	item, err := getPlaylistTrack(ctx, tx, playlistID, trackID)
	if err != nil {
		return domain.PlaylistTrack{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PlaylistTrack{}, mapError(err, "commit move playlist track")
	}
	return item, nil
}

func (r *PlaylistRepository) RemoveTrack(ctx context.Context, playlistID, trackID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return mapError(err, "begin remove playlist track")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := lockPlaylist(ctx, tx, playlistID); err != nil {
		return err
	}
	err = tx.QueryRow(ctx,
		`DELETE FROM playlist_tracks
		 WHERE playlist_id = $1::uuid AND track_id = $2::uuid
		 RETURNING 1`,
		playlistID, trackID,
	).Scan(new(int))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound("playlist track not found")
	}
	if err != nil {
		return mapError(err, "remove playlist track")
	}
	if err := tx.Commit(ctx); err != nil {
		return mapError(err, "commit remove playlist track")
	}
	return nil
}

func lockPlaylist(ctx context.Context, tx pgx.Tx, playlistID string) (int, error) {
	err := tx.QueryRow(ctx,
		`SELECT 1 FROM playlists WHERE id = $1::uuid FOR UPDATE`,
		playlistID,
	).Scan(new(int))
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.NotFound("playlist not found")
	}
	if err != nil {
		return 0, mapError(err, "lock playlist")
	}
	var count int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM playlist_tracks WHERE playlist_id = $1::uuid`,
		playlistID,
	).Scan(&count); err != nil {
		return 0, mapError(err, "count playlist tracks")
	}
	return count, nil
}

func getPlaylistTrack(ctx context.Context, tx pgx.Tx, playlistID, trackID string) (domain.PlaylistTrack, error) {
	var item domain.PlaylistTrack
	var status string
	err := tx.QueryRow(ctx,
		`SELECT pt.position, `+trackCols+`
		 FROM playlist_tracks pt
		 JOIN tracks t ON t.id = pt.track_id
		 WHERE pt.playlist_id = $1::uuid AND pt.track_id = $2::uuid`,
		playlistID, trackID,
	).Scan(
		&item.Position,
		&item.Track.ID,
		&item.Track.UserID,
		&item.Track.Title,
		&item.Track.Artist,
		&item.Track.ObjectKey,
		&item.Track.SizeBytes,
		&status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PlaylistTrack{}, domain.NotFound("playlist track not found")
	}
	if err != nil {
		return domain.PlaylistTrack{}, mapError(err, "get playlist track")
	}
	item.Track.Status = domain.TrackStatus(status)
	return item, nil
}
