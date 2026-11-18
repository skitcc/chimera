package postgres

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"

	"chimera/platform/idempotency"
	"chimera/platform/pg"
	"chimera/social/internal/domain"
)

const playlistCols = `id::text, owner_id::text, title, is_public`

type PlaylistRepository struct {
	db Pools
}

func NewPlaylistRepository(db Pools) *PlaylistRepository {
	return &PlaylistRepository{db: db}
}

func scanPlaylist(row scanner) (domain.Playlist, error) {
	var playlist domain.Playlist
	err := row.Scan(&playlist.ID, &playlist.OwnerID, &playlist.Title, &playlist.IsPublic)
	return playlist, err
}

const opCreatePlaylist = "social.CreatePlaylist"

// Create refuses owners whose UserDeleted event Social has already applied. With
// an idempotency key in ctx a repeat returns the playlist the first call created.
func (r *PlaylistRepository) Create(ctx context.Context, playlist domain.Playlist) (domain.Playlist, error) {
	if idempotency.From(ctx) == "" {
		return insertPlaylist(ctx, r.db.Write(), playlist)
	}
	var created domain.Playlist
	err := pg.InTx(ctx, r.db.Write(), func(tx pgx.Tx) error {
		replay, err := idempotency.Claim(ctx, tx, opCreatePlaylist, playlist.OwnerID,
			playlist.Title, strconv.FormatBool(playlist.IsPublic))
		if err != nil {
			return err
		}
		if replay != "" {
			created, err = getPlaylist(ctx, tx, replay)
			return err
		}
		created, err = insertPlaylist(ctx, tx, playlist)
		if err != nil {
			return err
		}
		return idempotency.Complete(ctx, tx, opCreatePlaylist, playlist.OwnerID, created.ID)
	})
	return created, err
}

func getPlaylist(ctx context.Context, db pg.DB, id string) (domain.Playlist, error) {
	playlist, err := scanPlaylist(db.QueryRow(ctx, `SELECT `+playlistCols+` FROM playlists WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Playlist{}, domain.NotFound("playlist not found")
	}
	if err != nil {
		return domain.Playlist{}, mapError(err, "get playlist")
	}
	return playlist, nil
}

func insertPlaylist(ctx context.Context, db pg.DB, playlist domain.Playlist) (domain.Playlist, error) {
	playlist, err := scanPlaylist(db.QueryRow(ctx,
		`INSERT INTO playlists (owner_id, title, is_public)
		 SELECT $1::uuid, $2, $3
		 WHERE NOT EXISTS (SELECT 1 FROM deleted_users WHERE user_id = $1::uuid)
		 RETURNING `+playlistCols,
		playlist.OwnerID, playlist.Title, playlist.IsPublic,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Playlist{}, domain.NotFound("user not found")
	}
	if err != nil {
		return domain.Playlist{}, mapError(err, "create playlist")
	}
	return playlist, nil
}

func (r *PlaylistRepository) GetByID(ctx context.Context, id string) (domain.Playlist, error) {
	var playlist domain.Playlist
	err := r.db.Find(ctx, func(db pg.DB) error {
		var err error
		playlist, err = scanPlaylist(db.QueryRow(ctx,
			`SELECT `+playlistCols+` FROM playlists WHERE id = $1::uuid`, id,
		))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Playlist{}, domain.NotFound("playlist not found")
	}
	if err != nil {
		return domain.Playlist{}, mapError(err, "get playlist")
	}
	return playlist, nil
}

func (r *PlaylistRepository) Update(ctx context.Context, playlist domain.Playlist) (domain.Playlist, error) {
	playlist, err := scanPlaylist(r.db.Write().QueryRow(ctx,
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
	tag, err := r.db.Write().Exec(ctx, `DELETE FROM playlists WHERE id = $1::uuid`, id)
	if err != nil {
		return mapError(err, "delete playlist")
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("playlist not found")
	}
	return nil
}

func (r *PlaylistRepository) List(ctx context.Context, filter domain.PlaylistFilter) ([]domain.Playlist, error) {
	var ownerID any
	if filter.OwnerID != "" {
		ownerID = filter.OwnerID
	}
	var isPublic any
	if filter.IsPublic != nil {
		isPublic = *filter.IsPublic
	}

	rows, err := r.db.Read(ctx).Query(ctx,
		`SELECT `+playlistCols+`
		 FROM playlists
		 WHERE ($1::uuid IS NULL OR owner_id = $1)
		   AND ($2::boolean IS NULL OR is_public = $2)
		 ORDER BY created_at DESC, id DESC`,
		ownerID, isPublic,
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

func (r *PlaylistRepository) ListEntries(ctx context.Context, playlistID string) ([]domain.PlaylistEntry, error) {
	rows, err := r.db.Read(ctx).Query(ctx,
		`SELECT position, track_id::text FROM playlist_tracks
		 WHERE playlist_id = $1::uuid
		 ORDER BY position`,
		playlistID,
	)
	if err != nil {
		return nil, mapError(err, "list playlist tracks")
	}
	defer rows.Close()

	entries := make([]domain.PlaylistEntry, 0)
	for rows.Next() {
		var e domain.PlaylistEntry
		if err := rows.Scan(&e.Position, &e.TrackID); err != nil {
			return nil, mapError(err, "scan playlist track")
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "list playlist tracks")
	}
	return entries, nil
}

// AddTrack refuses tracks whose TrackDeleted event Social has already applied.
func (r *PlaylistRepository) AddTrack(ctx context.Context, playlistID string, in domain.PlaylistTrackAdd) (domain.PlaylistEntry, error) {
	var entry domain.PlaylistEntry
	err := pg.InTx(ctx, r.db.Write(), func(tx pgx.Tx) error {
		count, err := lockPlaylist(ctx, tx, playlistID)
		if err != nil {
			return err
		}
		if in.Position > count {
			return domain.Invalid("position exceeds playlist length")
		}
		if _, err := tx.Exec(ctx,
			`UPDATE playlist_tracks SET position = position + 1
			 WHERE playlist_id = $1::uuid AND position >= $2`,
			playlistID, in.Position,
		); err != nil {
			return mapError(err, "shift playlist tracks")
		}
		tag, err := tx.Exec(ctx,
			`INSERT INTO playlist_tracks (playlist_id, track_id, position)
			 SELECT $1::uuid, $2::uuid, $3
			 WHERE NOT EXISTS (SELECT 1 FROM deleted_tracks WHERE track_id = $2::uuid)`,
			playlistID, in.TrackID, in.Position,
		)
		if err != nil {
			return mapError(err, "add playlist track")
		}
		if tag.RowsAffected() == 0 {
			return domain.NotFound("track not found")
		}
		entry, err = getEntry(ctx, tx, playlistID, in.TrackID)
		return err
	})
	return entry, err
}

func (r *PlaylistRepository) MoveTrack(ctx context.Context, playlistID, trackID string, position int) (domain.PlaylistEntry, error) {
	var entry domain.PlaylistEntry
	err := pg.InTx(ctx, r.db.Write(), func(tx pgx.Tx) error {
		count, err := lockPlaylist(ctx, tx, playlistID)
		if err != nil {
			return err
		}
		old, err := getEntry(ctx, tx, playlistID, trackID)
		if err != nil {
			return err
		}
		if position >= count {
			return domain.Invalid("position exceeds playlist length")
		}
		if err := shiftAround(ctx, tx, playlistID, trackID, old.Position, position); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE playlist_tracks SET position = $3
			 WHERE playlist_id = $1::uuid AND track_id = $2::uuid`,
			playlistID, trackID, position,
		); err != nil {
			return mapError(err, "move playlist track")
		}
		entry, err = getEntry(ctx, tx, playlistID, trackID)
		return err
	})
	return entry, err
}

func shiftAround(ctx context.Context, tx pgx.Tx, playlistID, trackID string, old, position int) error {
	var err error
	switch {
	case position < old:
		_, err = tx.Exec(ctx,
			`UPDATE playlist_tracks SET position = position + 1
			 WHERE playlist_id = $1::uuid AND track_id <> $2::uuid
			   AND position >= $3 AND position < $4`,
			playlistID, trackID, position, old,
		)
	case position > old:
		_, err = tx.Exec(ctx,
			`UPDATE playlist_tracks SET position = position - 1
			 WHERE playlist_id = $1::uuid AND track_id <> $2::uuid
			   AND position > $3 AND position <= $4`,
			playlistID, trackID, old, position,
		)
	}
	if err != nil {
		return mapError(err, "shift playlist tracks")
	}
	return nil
}

func (r *PlaylistRepository) RemoveTrack(ctx context.Context, playlistID, trackID string) error {
	return pg.InTx(ctx, r.db.Write(), func(tx pgx.Tx) error {
		if _, err := lockPlaylist(ctx, tx, playlistID); err != nil {
			return err
		}
		err := tx.QueryRow(ctx,
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
		return nil
	})
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

func getEntry(ctx context.Context, tx pgx.Tx, playlistID, trackID string) (domain.PlaylistEntry, error) {
	entry := domain.PlaylistEntry{TrackID: trackID}
	err := tx.QueryRow(ctx,
		`SELECT position FROM playlist_tracks
		 WHERE playlist_id = $1::uuid AND track_id = $2::uuid`,
		playlistID, trackID,
	).Scan(&entry.Position)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PlaylistEntry{}, domain.NotFound("playlist track not found")
	}
	if err != nil {
		return domain.PlaylistEntry{}, mapError(err, "get playlist track")
	}
	return entry, nil
}
