package usecase

import (
	"context"

	"chimera/catalog/internal/domain"
)

// CleanupService reacts to deletions: a removed user loses their tracks,
// a removed track loses its audio object.
type CleanupService struct {
	tracks  UserTrackRemover
	objects ObjectRemover
}

func NewCleanupService(tracks UserTrackRemover, objects ObjectRemover) *CleanupService {
	return &CleanupService{tracks: tracks, objects: objects}
}

func (s *CleanupService) RemoveUserTracks(ctx context.Context, d domain.UserDeletion) error {
	if err := d.Validate(); err != nil {
		return err
	}
	return s.tracks.DeleteByUser(ctx, d)
}

func (s *CleanupService) RemoveObject(ctx context.Context, key domain.ObjectKey) error {
	if err := key.Validate(); err != nil {
		return err
	}
	return s.objects.Remove(ctx, string(key))
}
