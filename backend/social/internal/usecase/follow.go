package usecase

import (
	"context"

	"chimera/social/internal/domain"
)

type FollowService struct {
	users   UserDirectory
	follows FollowRepository
}

func NewFollowService(users UserDirectory, follows FollowRepository) *FollowService {
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
		if _, err := s.users.GetUser(ctx, id); err != nil {
			return domain.FollowPage{}, err
		}
	}
	edges, err := s.follows.List(ctx, q.Filter())
	if err != nil {
		return domain.FollowPage{}, err
	}
	page := q.PageFollowEdges(edges)
	follows, err := s.hydrate(ctx, page.Items)
	if err != nil {
		return domain.FollowPage{}, err
	}
	return domain.FollowPage{Items: follows, NextCursor: page.NextCursor, Limit: page.Limit}, nil
}

func (s *FollowService) Get(ctx context.Context, followerID, followingID string) (domain.Follow, error) {
	if err := validatePair(followerID, followingID); err != nil {
		return domain.Follow{}, err
	}
	edges, err := s.follows.List(ctx, domain.FollowFilter{FollowerID: followerID, FollowingID: followingID})
	if err != nil {
		return domain.Follow{}, err
	}
	follows, err := s.hydrate(ctx, edges)
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
	if _, err := s.users.GetUser(ctx, followingID); err != nil {
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
	users, err := s.users.GetUsers(ctx, in.FollowingIDs)
	if err != nil {
		return err
	}
	if len(users) != len(in.FollowingIDs) {
		return domain.NotFound("not found")
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

// hydrate attaches Identity profiles. A follow whose user is already gone in
// Identity is dropped; the UserDeleted event removes the row itself.
func (s *FollowService) hydrate(ctx context.Context, edges []domain.FollowEdge) ([]domain.Follow, error) {
	ids := make([]string, 0, 2*len(edges))
	for _, e := range edges {
		ids = append(ids, e.FollowerID, e.FollowingID)
	}
	users, err := s.users.GetUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]domain.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}
	follows := make([]domain.Follow, 0, len(edges))
	for _, e := range edges {
		follower, okFollower := byID[e.FollowerID]
		following, okFollowing := byID[e.FollowingID]
		if !okFollower || !okFollowing {
			continue
		}
		follows = append(follows, domain.Follow{Follower: follower, Following: following, CreatedAt: e.CreatedAt})
	}
	return follows, nil
}

func validatePair(followerID, followingID string) error {
	if err := domain.UserID(followerID).Validate(); err != nil {
		return err
	}
	return domain.UserID(followingID).Validate()
}
