package usecase

import (
	"context"

	"chimera/internal/domain"
)

type TrackService struct {
	tracks TrackRepository
}

func NewTrackService(tracks TrackRepository) *TrackService {
	return &TrackService{tracks: tracks}
}

func (s *TrackService) List(ctx context.Context, q domain.PageQuery) (domain.TrackPage, error) {
	if err := q.Validate(); err != nil {
		return domain.TrackPage{}, err
	}
	tracks, err := s.tracks.List(ctx)
	if err != nil {
		return domain.TrackPage{}, err
	}
	return q.Page(tracks), nil
}

func (s *TrackService) GetByID(ctx context.Context, id string) (domain.Track, error) {
	if err := domain.TrackID(id).Validate(); err != nil {
		return domain.Track{}, err
	}
	return s.tracks.GetByID(ctx, id)
}

func (s *TrackService) Create(ctx context.Context, in domain.TrackWrite) (domain.Track, error) {
	if err := in.Validate(); err != nil {
		return domain.Track{}, err
	}
	return s.tracks.Create(ctx, in.Track(""))
}

func (s *TrackService) Update(ctx context.Context, id string, in domain.TrackWrite) (domain.Track, error) {
	if err := domain.TrackID(id).Validate(); err != nil {
		return domain.Track{}, err
	}
	if err := in.Validate(); err != nil {
		return domain.Track{}, err
	}
	return s.tracks.Update(ctx, in.Track(id))
}

func (s *TrackService) Delete(ctx context.Context, id string) error {
	if err := domain.TrackID(id).Validate(); err != nil {
		return err
	}
	return s.tracks.Delete(ctx, id)
}
