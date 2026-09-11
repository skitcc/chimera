package usecase

import (
	"context"

	"chimera/internal/domain"
)

type TrackService struct {
	tracks   TrackRepository
	objects  ObjectStorage
	maxBytes int64
}

func NewTrackService(tracks TrackRepository, objects ObjectStorage, maxBytes int64) *TrackService {
	return &TrackService{tracks: tracks, objects: objects, maxBytes: maxBytes}
}

func (s *TrackService) List(ctx context.Context, q domain.TrackFeedQuery) (domain.TrackPage, error) {
	if err := q.Validate(); err != nil {
		return domain.TrackPage{}, err
	}
	return s.page(ctx, q.PageQuery, q.Filter())
}

func (s *TrackService) ListByUploader(ctx context.Context, q domain.TrackOwnerQuery) (domain.TrackPage, error) {
	if err := q.Validate(); err != nil {
		return domain.TrackPage{}, err
	}
	return s.page(ctx, q.PageQuery, q.Filter())
}

func (s *TrackService) page(ctx context.Context, q domain.PageQuery, filter domain.TrackFilter) (domain.TrackPage, error) {
	tracks, err := s.tracks.List(ctx, filter)
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

func (s *TrackService) Update(ctx context.Context, id string, in domain.TrackWrite) (domain.Track, error) {
	if err := domain.TrackID(id).Validate(); err != nil {
		return domain.Track{}, err
	}
	if err := in.Validate(); err != nil {
		return domain.Track{}, err
	}
	track, err := s.tracks.GetByID(ctx, id)
	if err != nil {
		return domain.Track{}, err
	}
	track.Title = in.Title
	track.Artist = in.Artist
	return s.tracks.Update(ctx, track)
}

func (s *TrackService) Delete(ctx context.Context, id string) error {
	if err := domain.TrackID(id).Validate(); err != nil {
		return err
	}
	return s.tracks.Delete(ctx, id)
}

func (s *TrackService) InitUpload(ctx context.Context, in domain.TrackUploadInit) (domain.TrackUploadSession, error) {
	if err := in.Validate(); err != nil {
		return domain.TrackUploadSession{}, err
	}
	if err := in.ValidateSize(s.maxBytes); err != nil {
		return domain.TrackUploadSession{}, err
	}

	track, err := s.tracks.Create(ctx, in.Track())
	if err != nil {
		return domain.TrackUploadSession{}, err
	}

	url, err := s.objects.PresignPut(ctx, track.AudioObjectKey())
	if err != nil {
		if delErr := s.tracks.Delete(ctx, track.ID); delErr != nil {
			return domain.TrackUploadSession{}, delErr
		}
		return domain.TrackUploadSession{}, err
	}

	return domain.TrackUploadSession{Track: track, UploadURL: url}, nil
}

func (s *TrackService) CompleteUpload(ctx context.Context, in domain.TrackUploadComplete) (domain.Track, error) {
	if err := in.Validate(); err != nil {
		return domain.Track{}, err
	}

	track, err := s.tracks.GetByID(ctx, in.TrackID)
	if err != nil {
		return domain.Track{}, err
	}
	if err := track.OwnedBy(in.UserID); err != nil {
		return domain.Track{}, err
	}

	if err := s.verifyObject(ctx, track); err != nil {
		return domain.Track{}, err
	}
	if err := track.MarkProcessing(); err != nil {
		return domain.Track{}, err
	}
	if _, err := s.tracks.Update(ctx, track); err != nil {
		return domain.Track{}, err
	}

	return s.publish(ctx, track)
}

func (s *TrackService) StreamURL(ctx context.Context, id string) (string, error) {
	if err := domain.TrackID(id).Validate(); err != nil {
		return "", err
	}
	track, err := s.tracks.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if err := track.EnsureReady(); err != nil {
		return "", err
	}
	return s.objects.PresignGet(ctx, track.AudioObjectKey())
}

func (s *TrackService) publish(ctx context.Context, track domain.Track) (domain.Track, error) {
	if err := s.verifyObject(ctx, track); err != nil {
		return domain.Track{}, err
	}
	if err := track.MarkReady(); err != nil {
		return domain.Track{}, err
	}
	return s.tracks.Update(ctx, track)
}

func (s *TrackService) verifyObject(ctx context.Context, track domain.Track) error {
	size, err := s.objects.Stat(ctx, track.AudioObjectKey())
	if err != nil {
		return err
	}
	return track.ConfirmUpload(size)
}
