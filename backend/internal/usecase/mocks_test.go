package usecase

import (
	"context"

	"chimera/internal/domain"
)

type mockTrackRepository struct {
	getResult domain.Track
	getErr    error
	getIDs    []string
}

func (m *mockTrackRepository) List(context.Context, domain.TrackFilter) ([]domain.Track, error) {
	panic("unexpected List call")
}

func (m *mockTrackRepository) GetByID(_ context.Context, id string) (domain.Track, error) {
	m.getIDs = append(m.getIDs, id)
	return m.getResult, m.getErr
}

func (m *mockTrackRepository) Create(context.Context, domain.Track) (domain.Track, error) {
	panic("unexpected Create call")
}

func (m *mockTrackRepository) Update(context.Context, domain.Track) (domain.Track, error) {
	panic("unexpected Update call")
}

func (m *mockTrackRepository) Delete(context.Context, string) error {
	panic("unexpected Delete call")
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

func (s *spyTrackLikeRepository) ListReadyByUser(context.Context, string) ([]domain.Track, error) {
	panic("unexpected ListReadyByUser call")
}

type stubObjectStorage struct{}

func (stubObjectStorage) PresignPut(context.Context, string) (string, error) {
	panic("unexpected PresignPut call")
}

func (stubObjectStorage) PresignGet(context.Context, string) (string, error) {
	panic("unexpected PresignGet call")
}

func (stubObjectStorage) Stat(context.Context, string) (int64, error) {
	panic("unexpected Stat call")
}
