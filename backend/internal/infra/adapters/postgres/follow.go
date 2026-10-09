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

func (r *FollowRepository) ListFollowers(ctx context.Context, userID string) ([]domain.User, error) {
	return r.listUsers(ctx,
		`SELECT u.id::text, u.email, u.name
		 FROM user_follows f
		 JOIN users u ON u.id = f.follower_id
		 WHERE f.following_id = $1::uuid
		 ORDER BY f.created_at DESC, u.id`,
		userID,
	)
}

func (r *FollowRepository) ListFollowing(ctx context.Context, userID string) ([]domain.User, error) {
	return r.listUsers(ctx,
		`SELECT u.id::text, u.email, u.name
		 FROM user_follows f
		 JOIN users u ON u.id = f.following_id
		 WHERE f.follower_id = $1::uuid
		 ORDER BY f.created_at DESC, u.id`,
		userID,
	)
}

func (r *FollowRepository) listUsers(ctx context.Context, query, userID string) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, mapError(err, "list follows")
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Email, &user.Name); err != nil {
			return nil, mapError(err, "scan follow")
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "list follows")
	}
	return users, nil
}
