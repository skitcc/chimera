package usecase

import (
	"context"

	"chimera/catalog/internal/domain"
)

type fakeTrackRepository struct {
	tracks       map[string]domain.Track
	order        []string
	listErr      error
	getErr       error
	createErr    error
	updateErrors []error
	deleteErr    error
	nextTrackID  string
}

func newFakeTrackRepository() *fakeTrackRepository {
	return &fakeTrackRepository{
		tracks:      make(map[string]domain.Track),
		nextTrackID: testTrackID,
	}
}

func (f *fakeTrackRepository) seed(tracks ...domain.Track) {
	for _, track := range tracks {
		if _, ok := f.tracks[track.ID]; !ok {
			f.order = append(f.order, track.ID)
		}
		f.tracks[track.ID] = track
	}
}

func (f *fakeTrackRepository) List(_ context.Context, filter domain.TrackFilter) ([]domain.Track, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	tracks := make([]domain.Track, 0, len(f.tracks))
	for _, id := range f.order {
		track := f.tracks[id]
		if filter.Status != "" && track.Status != filter.Status {
			continue
		}
		if filter.UserID != "" && track.UserID != filter.UserID {
			continue
		}
		if filter.Artist != "" && track.Artist != filter.Artist {
			continue
		}
		tracks = append(tracks, track)
	}
	return tracks, nil
}

func (f *fakeTrackRepository) GetMany(_ context.Context, ids []string) ([]domain.Track, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	tracks := make([]domain.Track, 0, len(ids))
	for _, id := range ids {
		if track, ok := f.tracks[id]; ok {
			tracks = append(tracks, track)
		}
	}
	return tracks, nil
}

func (f *fakeTrackRepository) GetByID(_ context.Context, id string) (domain.Track, error) {
	if f.getErr != nil {
		return domain.Track{}, f.getErr
	}
	track, ok := f.tracks[id]
	if !ok {
		return domain.Track{}, domain.NotFound("track not found")
	}
	return track, nil
}

func (f *fakeTrackRepository) Create(_ context.Context, track domain.Track) (domain.Track, error) {
	if f.createErr != nil {
		return domain.Track{}, f.createErr
	}
	if track.ID == "" {
		track.ID = f.nextTrackID
	}
	f.seed(track)
	return track, nil
}

func (f *fakeTrackRepository) Update(_ context.Context, track domain.Track) (domain.Track, error) {
	if len(f.updateErrors) > 0 {
		err := f.updateErrors[0]
		f.updateErrors = f.updateErrors[1:]
		if err != nil {
			return domain.Track{}, err
		}
	}
	if _, ok := f.tracks[track.ID]; !ok {
		return domain.Track{}, domain.NotFound("track not found")
	}
	f.tracks[track.ID] = track
	return track, nil
}

func (f *fakeTrackRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.tracks[id]; !ok {
		return domain.NotFound("track not found")
	}
	delete(f.tracks, id)
	for i, trackID := range f.order {
		if trackID == id {
			f.order = append(f.order[:i], f.order[i+1:]...)
			break
		}
	}
	return nil
}

type fakeObjectStorage struct {
	sizes      map[string]int64
	putURL     string
	getURL     string
	putErr     error
	getErr     error
	statErrors []error
}

func newFakeObjectStorage() *fakeObjectStorage {
	return &fakeObjectStorage{
		sizes:  make(map[string]int64),
		putURL: "https://uploads.example/track",
		getURL: "https://streams.example/track",
	}
}

func (f *fakeObjectStorage) PresignPut(context.Context, string) (string, error) {
	if f.putErr != nil {
		return "", f.putErr
	}
	return f.putURL, nil
}

func (f *fakeObjectStorage) PresignGet(context.Context, string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.getURL, nil
}

func (f *fakeObjectStorage) Stat(_ context.Context, key string) (int64, error) {
	if len(f.statErrors) > 0 {
		err := f.statErrors[0]
		f.statErrors = f.statErrors[1:]
		if err != nil {
			return 0, err
		}
	}
	return f.sizes[key], nil
}
