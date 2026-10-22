package postgres

import (
	"context"

	"chimera/internal/domain"
)

type FollowRepository struct {
	db db
}

func NewFollowRepository(db db) *FollowRepository {
	return &FollowRepository{db: db}
}

func (r *FollowRepository) Add(ctx context.Context, followerID, followingID string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO user_follows (follower_id, following_id)
		 VALUES ($1::uuid, $2::uuid)
		 ON CONFLICT DO NOTHING`,
		followerID, followingID,
	)
	if err != nil {
		return mapError(err, "follow user")
	}
	return nil
}

func (r *FollowRepository) Remove(ctx context.Context, followerID, followingID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM user_follows WHERE follower_id = $1::uuid AND following_id = $2::uuid`,
		followerID, followingID,
	)
	if err != nil {
		return mapError(err, "unfollow user")
	}
	return nil
}

func (r *FollowRepository) AddMany(ctx context.Context, followerID string, followingIDs []string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO user_follows (follower_id, following_id)
		 SELECT $1::uuid, unnest($2::uuid[])
		 ON CONFLICT DO NOTHING`,
		followerID, followingIDs,
	)
	if err != nil {
		return mapError(err, "follow users")
	}
	return nil
}

func (r *FollowRepository) RemoveMany(ctx context.Context, followerID string, followingIDs []string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM user_follows WHERE follower_id = $1::uuid AND following_id = ANY($2::uuid[])`,
		followerID, followingIDs,
	)
	if err != nil {
		return mapError(err, "unfollow users")
	}
	return nil
}

func (r *FollowRepository) List(ctx context.Context, filter domain.FollowFilter) ([]domain.Follow, error) {
	var followerID any
	if filter.FollowerID != "" {
		followerID = filter.FollowerID
	}
	var followingID any
	if filter.FollowingID != "" {
		followingID = filter.FollowingID
	}

	rows, err := r.db.Query(ctx,
		`SELECT a.id::text, a.email, a.name, b.id::text, b.email, b.name, f.created_at
		 FROM user_follows f
		 JOIN users a ON a.id = f.follower_id
		 JOIN users b ON b.id = f.following_id
		 WHERE ($1::uuid IS NULL OR f.follower_id = $1)
		   AND ($2::uuid IS NULL OR f.following_id = $2)
		 ORDER BY f.created_at DESC, f.follower_id, f.following_id`,
		followerID, followingID,
	)
	if err != nil {
		return nil, mapError(err, "list follows")
	}
	defer rows.Close()

	follows := make([]domain.Follow, 0)
	for rows.Next() {
		var f domain.Follow
		if err := rows.Scan(
			&f.Follower.ID, &f.Follower.Email, &f.Follower.Name,
			&f.Following.ID, &f.Following.Email, &f.Following.Name,
			&f.CreatedAt,
		); err != nil {
			return nil, mapError(err, "scan follow")
		}
		follows = append(follows, f)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "list follows")
	}
	return follows, nil
}
