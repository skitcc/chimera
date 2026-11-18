package usecase

import (
	"context"

	"chimera/social/internal/domain"
)

type fakeTrackCatalog struct {
	tracks  map[string]domain.Track
	getErr  error
	listErr error
	listIDs []string
}

func newFakeTrackCatalog() *fakeTrackCatalog {
	return &fakeTrackCatalog{tracks: make(map[string]domain.Track)}
}

func (f *fakeTrackCatalog) seed(tracks ...domain.Track) {
	for _, track := range tracks {
		f.tracks[track.ID] = track
	}
}

func (f *fakeTrackCatalog) GetTrack(_ context.Context, id string) (domain.Track, error) {
	if f.getErr != nil {
		return domain.Track{}, f.getErr
	}
	track, ok := f.tracks[id]
	if !ok {
		return domain.Track{}, domain.NotFound("track not found")
	}
	return track, nil
}

func (f *fakeTrackCatalog) GetTracks(_ context.Context, ids []string) ([]domain.Track, error) {
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

// ListByIDs keeps the requested order and the status filter; paging is Catalog's.
func (f *fakeTrackCatalog) ListByIDs(_ context.Context, q domain.LikedTracksQuery, ids []string) (domain.TrackPage, error) {
	f.listIDs = ids
	if f.listErr != nil {
		return domain.TrackPage{}, f.listErr
	}
	page := domain.TrackPage{Items: []domain.Track{}, Limit: q.Page.Limit}
	for _, id := range ids {
		track, ok := f.tracks[id]
		if !ok || (q.Status != "" && track.Status != q.Status) {
			continue
		}
		page.Items = append(page.Items, track)
	}
	return page, nil
}

type fakeTrackLikeRepository struct {
	likes     map[string]map[string]bool
	order     []string
	addErr    error
	removeErr error
	listErr   error
}

func newFakeTrackLikeRepository() *fakeTrackLikeRepository {
	return &fakeTrackLikeRepository{likes: make(map[string]map[string]bool)}
}

func (f *fakeTrackLikeRepository) Add(_ context.Context, userID, trackID string) error {
	if f.addErr != nil {
		return f.addErr
	}
	if f.likes[userID] == nil {
		f.likes[userID] = make(map[string]bool)
	}
	f.likes[userID][trackID] = true
	f.order = append(f.order, trackID)
	return nil
}

func (f *fakeTrackLikeRepository) Remove(_ context.Context, userID, trackID string) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	delete(f.likes[userID], trackID)
	return nil
}

func (f *fakeTrackLikeRepository) ListTrackIDs(_ context.Context, userID string) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	ids := make([]string, 0)
	for _, id := range f.order {
		if f.likes[userID][id] {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

type fakeCleanupRepository struct {
	users  []domain.UserDeletion
	tracks []domain.TrackDeletion
	err    error
}

func (f *fakeCleanupRepository) DeleteUser(_ context.Context, d domain.UserDeletion) error {
	f.users = append(f.users, d)
	return f.err
}

func (f *fakeCleanupRepository) DeleteTrack(_ context.Context, d domain.TrackDeletion) error {
	f.tracks = append(f.tracks, d)
	return f.err
}
