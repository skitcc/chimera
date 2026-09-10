package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chimera/internal/domain"
)

type TrackRepository struct {
	pool *pgxpool.Pool
}

func NewTrackRepository(pool *pgxpool.Pool) *TrackRepository {
	return &TrackRepository{pool: pool}
}

func (r *TrackRepository) List(ctx context.Context) ([]domain.Track, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, title, artist FROM tracks ORDER BY created_at`)
	if err != nil {
		return nil, mapError(err, "list tracks")
	}
	defer rows.Close()

	tracks := make([]domain.Track, 0)
	for rows.Next() {
		var t domain.Track
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist); err != nil {
			return nil, mapError(err, "scan track")
		}
		tracks = append(tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "list tracks")
	}
	return tracks, nil
}

func (r *TrackRepository) GetByID(ctx context.Context, id string) (domain.Track, error) {
	var t domain.Track
	err := r.pool.QueryRow(ctx, `SELECT id::text, title, artist FROM tracks WHERE id = $1::uuid`, id).
		Scan(&t.ID, &t.Title, &t.Artist)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Track{}, domain.NotFound("track not found")
	}
	if err != nil {
		return domain.Track{}, mapError(err, "get track")
	}
	return t, nil
}

func (r *TrackRepository) Create(ctx context.Context, t domain.Track) (domain.Track, error) {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO tracks (title, artist) VALUES ($1, $2)
		 RETURNING id::text, title, artist`,
		t.Title, t.Artist,
	).Scan(&t.ID, &t.Title, &t.Artist)
	if err != nil {
		return domain.Track{}, mapError(err, "create track")
	}
	return t, nil
}

func (r *TrackRepository) Update(ctx context.Context, t domain.Track) (domain.Track, error) {
	err := r.pool.QueryRow(ctx,
		`UPDATE tracks SET title = $2, artist = $3 WHERE id = $1::uuid
		 RETURNING id::text, title, artist`,
		t.ID, t.Title, t.Artist,
	).Scan(&t.ID, &t.Title, &t.Artist)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Track{}, domain.NotFound("track not found")
	}
	if err != nil {
		return domain.Track{}, mapError(err, "update track")
	}
	return t, nil
}

func (r *TrackRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tracks WHERE id = $1::uuid`, id)
	if err != nil {
		return mapError(err, "delete track")
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("track not found")
	}
	return nil
}
