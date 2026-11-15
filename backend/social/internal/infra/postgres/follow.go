package postgres

import (
	"context"

	"chimera/social/internal/domain"
)

const notDeleted = `NOT EXISTS (SELECT 1 FROM deleted_users d WHERE d.user_id IN ($1::uuid, f.id))`

type FollowRepository struct {
	db Pools
}

func NewFollowRepository(db Pools) *FollowRepository {
	return &FollowRepository{db: db}
}

// Add is idempotent. A pair with an already deleted user is skipped.
func (r *FollowRepository) Add(ctx context.Context, followerID, followingID string) error {
	return r.AddMany(ctx, followerID, []string{followingID})
}

func (r *FollowRepository) Remove(ctx context.Context, followerID, followingID string) error {
	_, err := r.db.Write().Exec(ctx,
		`DELETE FROM user_follows WHERE follower_id = $1::uuid AND following_id = $2::uuid`,
		followerID, followingID,
	)
	if err != nil {
		return mapError(err, "unfollow user")
	}
	return nil
}

func (r *FollowRepository) AddMany(ctx context.Context, followerID string, followingIDs []string) error {
	_, err := r.db.Write().Exec(ctx,
		`INSERT INTO user_follows (follower_id, following_id)
		 SELECT $1::uuid, f.id FROM unnest($2::uuid[]) AS f(id)
		 WHERE `+notDeleted+`
		 ON CONFLICT DO NOTHING`,
		followerID, followingIDs,
	)
	if err != nil {
		return mapError(err, "follow users")
	}
	return nil
}

func (r *FollowRepository) RemoveMany(ctx context.Context, followerID string, followingIDs []string) error {
	_, err := r.db.Write().Exec(ctx,
		`DELETE FROM user_follows WHERE follower_id = $1::uuid AND following_id = ANY($2::uuid[])`,
		followerID, followingIDs,
	)
	if err != nil {
		return mapError(err, "unfollow users")
	}
	return nil
}

func (r *FollowRepository) List(ctx context.Context, filter domain.FollowFilter) ([]domain.FollowEdge, error) {
	var followerID any
	if filter.FollowerID != "" {
		followerID = filter.FollowerID
	}
	var followingID any
	if filter.FollowingID != "" {
		followingID = filter.FollowingID
	}

	rows, err := r.db.Read(ctx).Query(ctx,
		`SELECT follower_id::text, following_id::text, created_at
		 FROM user_follows
		 WHERE ($1::uuid IS NULL OR follower_id = $1)
		   AND ($2::uuid IS NULL OR following_id = $2)
		 ORDER BY created_at DESC, follower_id, following_id`,
		followerID, followingID,
	)
	if err != nil {
		return nil, mapError(err, "list follows")
	}
	defer rows.Close()

	edges := make([]domain.FollowEdge, 0)
	for rows.Next() {
		var e domain.FollowEdge
		if err := rows.Scan(&e.FollowerID, &e.FollowingID, &e.CreatedAt); err != nil {
			return nil, mapError(err, "scan follow")
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "list follows")
	}
	return edges, nil
}
