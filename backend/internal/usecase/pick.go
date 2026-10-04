package usecase

import (
	"context"

	"chimera/internal/domain"
)

const randomPickSize = 20

type PickService struct {
	users     UserRepository
	selection TrackSelectionRepository
	picks     PickRepository
}

func NewPickService(users UserRepository, selection TrackSelectionRepository, picks PickRepository) *PickService {
	return &PickService{users: users, selection: selection, picks: picks}
}

func (s *PickService) Generate(ctx context.Context, userID string) (domain.PickDetail, error) {
	if err := domain.UserID(userID).Validate(); err != nil {
		return domain.PickDetail{}, err
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return domain.PickDetail{}, currentUserError(err)
	}
	tracks, err := s.selection.ListRandomReady(ctx, randomPickSize)
	if err != nil {
		return domain.PickDetail{}, err
	}
	return s.picks.CreateRandom(ctx, userID, "Random pick", tracks)
}

func (s *PickService) List(ctx context.Context, userID string, q domain.PageQuery) (domain.PickPage, error) {
	if err := domain.UserID(userID).Validate(); err != nil {
		return domain.PickPage{}, err
	}
	if err := q.Validate(); err != nil {
		return domain.PickPage{}, err
	}
	picks, err := s.picks.ListByUser(ctx, userID)
	if err != nil {
		return domain.PickPage{}, err
	}
	return q.PagePicks(picks), nil
}

func (s *PickService) Get(ctx context.Context, userID, id string) (domain.PickDetail, error) {
	if err := domain.PickID(id).Validate(); err != nil {
		return domain.PickDetail{}, err
	}
	pick, err := s.picks.GetByID(ctx, id)
	if err != nil {
		return domain.PickDetail{}, err
	}
	if err := pick.Pick.RequireOwner(userID); err != nil {
		return domain.PickDetail{}, err
	}
	return pick, nil
}

func (s *PickService) Rename(ctx context.Context, userID, id string, in domain.PickPatch) (domain.Pick, error) {
	if err := in.Validate(); err != nil {
		return domain.Pick{}, err
	}
	pick, err := s.Get(ctx, userID, id)
	if err != nil {
		return domain.Pick{}, err
	}
	return s.picks.UpdateTitle(ctx, pick.ID, in.Title)
}

func (s *PickService) Delete(ctx context.Context, userID, id string) error {
	pick, err := s.Get(ctx, userID, id)
	if err != nil {
		return err
	}
	return s.picks.Delete(ctx, pick.ID)
}
