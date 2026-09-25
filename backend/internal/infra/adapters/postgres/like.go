package postgres

import (
	"context"

	"chimera/internal/domain"
)

type TrackLikeRepository struct {
	db db
}

func NewTrackLikeRepository(db db) *TrackLikeRepository {
	return &TrackLikeRepository{db: db}
}

func (r *TrackLikeRepository) Add(ctx context.Context, userID, trackID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO track_likes (user_id, track_id) VALUES ($1::uuid, $2::uuid)`,
		userID, trackID,
	)
	if err != nil {
		return mapError(err, "like track")
	}
	return nil
}

func (r *TrackLikeRepository) Remove(ctx context.Context, userID, trackID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM track_likes WHERE user_id = $1::uuid AND track_id = $2::uuid`,
		userID, trackID,
	)
	if err != nil {
		return mapError(err, "unlike track")
	}
	return nil
}

func (r *TrackLikeRepository) ListReadyByUser(ctx context.Context, userID string) ([]domain.Track, error) {
	rows, err := r.db.Query(ctx,
		`SELECT t.id::text, t.user_id::text, t.title, t.artist, t.object_key, t.size_bytes, t.status
		 FROM track_likes l
		 JOIN tracks t ON t.id = l.track_id
		 WHERE l.user_id = $1::uuid AND t.status = $2
		 ORDER BY l.created_at DESC`,
		userID, string(domain.TrackReady),
	)
	if err != nil {
		return nil, mapError(err, "list liked tracks")
	}
	return collectTracks(rows)
}
