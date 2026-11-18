package usecase

import (
	"context"

	"chimera/social/internal/domain"
)

type LikeService struct {
	tracks TrackCatalog
	likes  TrackLikeRepository
}

func NewLikeService(tracks TrackCatalog, likes TrackLikeRepository) *LikeService {
	return &LikeService{tracks: tracks, likes: likes}
}

func (s *LikeService) PutLike(ctx context.Context, actorID string, in domain.TrackLike) error {
	if err := in.Validate(); err != nil {
		return err
	}
	if err := domain.UserID(in.UserID).RequireActor(actorID); err != nil {
		return err
	}
	if err := s.Like(ctx, in); err != nil && !domain.Is(err, domain.CodeConflict) {
		return err
	}
	return nil
}

func (s *LikeService) DeleteLike(ctx context.Context, actorID string, in domain.TrackLike) error {
	if err := in.Validate(); err != nil {
		return err
	}
	if err := domain.UserID(in.UserID).RequireActor(actorID); err != nil {
		return err
	}
	return s.Unlike(ctx, in)
}

func (s *LikeService) Like(ctx context.Context, in domain.TrackLike) error {
	if err := in.Validate(); err != nil {
		return err
	}
	track, err := s.tracks.GetTrack(ctx, in.TrackID)
	if err != nil {
		return err
	}
	if err := track.EnsureReady(); err != nil {
		return err
	}
	return s.likes.Add(ctx, in.UserID, in.TrackID)
}

func (s *LikeService) Unlike(ctx context.Context, in domain.TrackLike) error {
	if err := in.Validate(); err != nil {
		return err
	}
	if _, err := s.tracks.GetTrack(ctx, in.TrackID); err != nil {
		return err
	}
	return s.likes.Remove(ctx, in.UserID, in.TrackID)
}

// ListLiked returns the viewer's liked tracks. Catalog filters and pages them
// exactly like any other track listing.
func (s *LikeService) ListLiked(ctx context.Context, q domain.LikedTracksQuery) (domain.TrackPage, error) {
	if err := q.Validate(); err != nil {
		return domain.TrackPage{}, err
	}
	ids, err := s.likes.ListTrackIDs(ctx, q.LikedBy)
	if err != nil {
		return domain.TrackPage{}, err
	}
	return s.tracks.ListByIDs(ctx, q, ids)
}
