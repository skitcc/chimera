package usecase

import (
	"context"

	"chimera/social/internal/domain"
)

type mockTrackCatalog struct {
	getResult domain.Track
	getErr    error
	getIDs    []string
}

func (m *mockTrackCatalog) GetTrack(_ context.Context, id string) (domain.Track, error) {
	m.getIDs = append(m.getIDs, id)
	return m.getResult, m.getErr
}

func (m *mockTrackCatalog) GetTracks(context.Context, []string) ([]domain.Track, error) {
	panic("unexpected GetTracks call")
}

func (m *mockTrackCatalog) ListByIDs(context.Context, domain.LikedTracksQuery, []string) (domain.TrackPage, error) {
	panic("unexpected ListByIDs call")
}

type trackLikeCall struct {
	userID  string
	trackID string
}

type spyTrackLikeRepository struct {
	addErr   error
	addCalls []trackLikeCall
}

func (s *spyTrackLikeRepository) Add(_ context.Context, userID, trackID string) error {
	s.addCalls = append(s.addCalls, trackLikeCall{userID: userID, trackID: trackID})
	return s.addErr
}

func (s *spyTrackLikeRepository) Remove(context.Context, string, string) error {
	panic("unexpected Remove call")
}

func (s *spyTrackLikeRepository) ListTrackIDs(context.Context, string) ([]string, error) {
	panic("unexpected ListTrackIDs call")
}
