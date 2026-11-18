package usecase

import (
	"context"

	"chimera/social/internal/domain"
)

// CleanupService drops social data of users and tracks deleted elsewhere.
type CleanupService struct {
	store CleanupRepository
}

func NewCleanupService(store CleanupRepository) *CleanupService {
	return &CleanupService{store: store}
}

func (s *CleanupService) RemoveUser(ctx context.Context, d domain.UserDeletion) error {
	if err := d.Validate(); err != nil {
		return err
	}
	return s.store.DeleteUser(ctx, d)
}

func (s *CleanupService) RemoveTrack(ctx context.Context, d domain.TrackDeletion) error {
	if err := d.Validate(); err != nil {
		return err
	}
	return s.store.DeleteTrack(ctx, d)
}
