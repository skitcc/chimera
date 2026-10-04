package usecase

import (
	"context"

	"chimera/internal/domain"
)

type FollowService struct {
	users   UserRepository
	follows FollowRepository
}

func NewFollowService(users UserRepository, follows FollowRepository) *FollowService {
	return &FollowService{users: users, follows: follows}
}

func (s *FollowService) Follow(ctx context.Context, actorID, userID string) error {
	if err := domain.UserID(actorID).Validate(); err != nil {
		return err
	}
	if err := domain.UserID(userID).Validate(); err != nil {
		return err
	}
	if actorID == userID {
		return domain.Invalid("cannot follow yourself")
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return err
	}
	return s.follows.Add(ctx, actorID, userID)
}

func (s *FollowService) Unfollow(ctx context.Context, actorID, userID string) error {
	if err := domain.UserID(actorID).Validate(); err != nil {
		return err
	}
	if err := domain.UserID(userID).Validate(); err != nil {
		return err
	}
	return s.follows.Remove(ctx, actorID, userID)
}

func (s *FollowService) ListFollowers(ctx context.Context, userID string, q domain.PageQuery) (domain.UserPage, error) {
	if err := domain.UserID(userID).Validate(); err != nil {
		return domain.UserPage{}, err
	}
	if err := q.Validate(); err != nil {
		return domain.UserPage{}, err
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return domain.UserPage{}, err
	}
	users, err := s.follows.ListFollowers(ctx, userID)
	if err != nil {
		return domain.UserPage{}, err
	}
	return q.PageUsers(users), nil
}

func (s *FollowService) ListFollowing(ctx context.Context, userID string, q domain.PageQuery) (domain.UserPage, error) {
	if err := domain.UserID(userID).Validate(); err != nil {
		return domain.UserPage{}, err
	}
	if err := q.Validate(); err != nil {
		return domain.UserPage{}, err
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return domain.UserPage{}, err
	}
	users, err := s.follows.ListFollowing(ctx, userID)
	if err != nil {
		return domain.UserPage{}, err
	}
	return q.PageUsers(users), nil
}
