package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"chimera/internal/domain"
)

const trackCols = `id::text, user_id::text, title, artist, object_key, size_bytes, status`

type TrackRepository struct {
	db db
}

func NewTrackRepository(db db) *TrackRepository {
	return &TrackRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTrack(row scanner) (domain.Track, error) {
	var t domain.Track
	var status string
	err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.Artist, &t.ObjectKey, &t.SizeBytes, &status)
	t.Status = domain.TrackStatus(status)
	return t, err
}

func collectTracks(rows pgx.Rows) ([]domain.Track, error) {
	defer rows.Close()

	tracks := make([]domain.Track, 0)
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, mapError(err, "scan track")
		}
		tracks = append(tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "list tracks")
	}
	return tracks, nil
}

func (r *TrackRepository) List(ctx context.Context, filter domain.TrackFilter) ([]domain.Track, error) {
	var status any
	if filter.Status != "" {
		status = string(filter.Status)
	}
	var userID any
	if filter.UserID != "" {
		userID = filter.UserID
	}
	var artist any
	if filter.Artist != "" {
		artist = filter.Artist
	}

	rows, err := r.db.Query(ctx,
		`SELECT `+trackCols+`
		 FROM tracks
		 WHERE ($1::text IS NULL OR status = $1)
		   AND ($2::uuid IS NULL OR user_id = $2)
		   AND ($3::text IS NULL OR lower(btrim(artist)) = lower(btrim($3)))
		 ORDER BY created_at, id`,
		status, userID, artist,
	)
	if err != nil {
		return nil, mapError(err, "list tracks")
	}
	return collectTracks(rows)
}

func (r *TrackRepository) ListRandomReady(ctx context.Context, limit int) ([]domain.Track, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+trackCols+`
		 FROM tracks
		 WHERE status = $1
		 ORDER BY random()
		 LIMIT $2`,
		string(domain.TrackReady), limit,
	)
	if err != nil {
		return nil, mapError(err, "list random tracks")
	}
	return collectTracks(rows)
}

func (r *TrackRepository) GetByID(ctx context.Context, id string) (domain.Track, error) {
	t, err := scanTrack(r.db.QueryRow(ctx, `SELECT `+trackCols+` FROM tracks WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Track{}, domain.NotFound("track not found")
	}
	if err != nil {
		return domain.Track{}, mapError(err, "get track")
	}
	return t, nil
}

func (r *TrackRepository) Create(ctx context.Context, t domain.Track) (domain.Track, error) {
	t, err := scanTrack(r.db.QueryRow(ctx,
		`INSERT INTO tracks (id, user_id, title, artist, object_key, size_bytes, status)
		 SELECT i, $1::uuid, $2, $3, i::text || '.mp3', $4, $5
		 FROM (SELECT gen_random_uuid() AS i) s
		 RETURNING `+trackCols,
		t.UserID, t.Title, t.Artist, t.SizeBytes, string(t.Status),
	))
	if err != nil {
		return domain.Track{}, mapError(err, "create track")
	}
	return t, nil
}

func (r *TrackRepository) Update(ctx context.Context, t domain.Track) (domain.Track, error) {
	t, err := scanTrack(r.db.QueryRow(ctx,
		`UPDATE tracks SET user_id = $2::uuid, title = $3, artist = $4, object_key = $5, size_bytes = $6, status = $7
		 WHERE id = $1::uuid
		 RETURNING `+trackCols,
		t.ID, t.UserID, t.Title, t.Artist, t.AudioObjectKey(), t.SizeBytes, string(t.Status),
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Track{}, domain.NotFound("track not found")
	}
	if err != nil {
		return domain.Track{}, mapError(err, "update track")
	}
	return t, nil
}

func (r *TrackRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM tracks WHERE id = $1::uuid`, id)
	if err != nil {
		return mapError(err, "delete track")
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("track not found")
	}
	return nil
}
