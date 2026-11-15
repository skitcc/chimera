package postgres

import (
	"context"

	"chimera/platform/pg"
	"chimera/social/internal/domain"
)

type TrackLikeRepository struct {
	db pg.DB
}

func NewTrackLikeRepository(db pg.DB) *TrackLikeRepository {
	return &TrackLikeRepository{db: db}
}

// Add refuses users and tracks whose deletion Social has already applied.
func (r *TrackLikeRepository) Add(ctx context.Context, userID, trackID string) error {
	tag, err := r.db.Exec(ctx,
		`INSERT INTO track_likes (user_id, track_id)
		 SELECT $1::uuid, $2::uuid
		 WHERE NOT EXISTS (SELECT 1 FROM deleted_users WHERE user_id = $1::uuid)
		   AND NOT EXISTS (SELECT 1 FROM deleted_tracks WHERE track_id = $2::uuid)`,
		userID, trackID,
	)
	if err != nil {
		return mapError(err, "like track")
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("not found")
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

func (r *TrackLikeRepository) ListTrackIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT track_id::text FROM track_likes WHERE user_id = $1::uuid ORDER BY created_at DESC, track_id`,
		userID,
	)
	if err != nil {
		return nil, mapError(err, "list liked tracks")
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, mapError(err, "scan liked track")
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "list liked tracks")
	}
	return ids, nil
}
