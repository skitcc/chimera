package memory

import (
	"context"
	"strconv"
	"sync"

	"chimera/internal/business_logic/apperrors"
	"chimera/internal/business_logic/domain"
)

type TrackRepository struct {
	mu     sync.RWMutex
	tracks []domain.Track
	seq    int
}

func NewTrackRepository() *TrackRepository {
	return &TrackRepository{
		tracks: []domain.Track{
			{ID: "1", Title: "Night Drive", Artist: "Lumen"},
			{ID: "2", Title: "Open Road", Artist: "Northbound"},
			{ID: "3", Title: "Low Tide", Artist: "Harbour"},
			{ID: "4", Title: "Static Bloom", Artist: "Violet Grid"},
			{ID: "5", Title: "Afterlight", Artist: "Kite"},
		},
		seq: 5,
	}
}

func (r *TrackRepository) List(_ context.Context) ([]domain.Track, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.Track, len(r.tracks))
	copy(out, r.tracks)
	return out, nil
}

func (r *TrackRepository) GetByID(_ context.Context, id string) (domain.Track, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, t := range r.tracks {
		if t.ID == id {
			return t, nil
		}
	}
	return domain.Track{}, apperrors.NotFound("track not found")
}

func (r *TrackRepository) Create(_ context.Context, t domain.Track) (domain.Track, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	t.ID = strconv.Itoa(r.seq)
	r.tracks = append(r.tracks, t)
	return t, nil
}

func (r *TrackRepository) Update(_ context.Context, t domain.Track) (domain.Track, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, cur := range r.tracks {
		if cur.ID == t.ID {
			r.tracks[i] = t
			return t, nil
		}
	}
	return domain.Track{}, apperrors.NotFound("track not found")
}

func (r *TrackRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, t := range r.tracks {
		if t.ID == id {
			r.tracks = append(r.tracks[:i], r.tracks[i+1:]...)
			return nil
		}
	}
	return apperrors.NotFound("track not found")
}
