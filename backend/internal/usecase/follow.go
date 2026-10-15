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

func (s *FollowService) List(ctx context.Context, q domain.FollowListQuery) (domain.FollowPage, error) {
	if err := q.Validate(); err != nil {
		return domain.FollowPage{}, err
	}
	for _, id := range []string{q.FollowerID, q.FollowingID} {
		if id == "" {
			continue
		}
		if _, err := s.users.GetByID(ctx, id); err != nil {
			return domain.FollowPage{}, err
		}
	}
	follows, err := s.follows.List(ctx, q.Filter())
	if err != nil {
		return domain.FollowPage{}, err
	}
	return q.PageFollows(follows), nil
}

func (s *FollowService) Get(ctx context.Context, followerID, followingID string) (domain.Follow, error) {
	if err := validatePair(followerID, followingID); err != nil {
		return domain.Follow{}, err
	}
	follows, err := s.follows.List(ctx, domain.FollowFilter{FollowerID: followerID, FollowingID: followingID})
	if err != nil {
		return domain.Follow{}, err
	}
	if len(follows) == 0 {
		return domain.Follow{}, domain.NotFound("follow not found")
	}
	return follows[0], nil
}

func (s *FollowService) Put(ctx context.Context, actorID, followerID, followingID string) error {
	if err := validatePair(followerID, followingID); err != nil {
		return err
	}
	if err := domain.UserID(followerID).RequireActor(actorID); err != nil {
		return err
	}
	if followerID == followingID {
		return domain.Invalid("cannot follow yourself")
	}
	if _, err := s.users.GetByID(ctx, followingID); err != nil {
		return err
	}
	return s.follows.Add(ctx, followerID, followingID)
}

func (s *FollowService) Delete(ctx context.Context, actorID, followerID, followingID string) error {
	if err := validatePair(followerID, followingID); err != nil {
		return err
	}
	if err := domain.UserID(followerID).RequireActor(actorID); err != nil {
		return err
	}
	return s.follows.Remove(ctx, followerID, followingID)
}

func (s *FollowService) CreateMany(ctx context.Context, actorID string, in domain.FollowBatch) error {
	if err := in.ValidateCreate(); err != nil {
		return err
	}
	if err := domain.UserID(in.FollowerID).RequireActor(actorID); err != nil {
		return err
	}
	return s.follows.AddMany(ctx, in.FollowerID, in.FollowingIDs)
}

func (s *FollowService) DeleteMany(ctx context.Context, actorID string, in domain.FollowBatch) error {
	if err := in.Validate(); err != nil {
		return err
	}
	if err := domain.UserID(in.FollowerID).RequireActor(actorID); err != nil {
		return err
	}
	return s.follows.RemoveMany(ctx, in.FollowerID, in.FollowingIDs)
}

func validatePair(followerID, followingID string) error {
	if err := domain.UserID(followerID).Validate(); err != nil {
		return err
	}
	return domain.UserID(followingID).Validate()
}
