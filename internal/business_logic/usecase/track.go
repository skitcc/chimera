package usecase

import (
	"context"
	"strconv"

	"chimera/internal/business_logic/apperrors"
	"chimera/internal/business_logic/domain"
)

var stubTracks = []domain.Track{
	{ID: "1", Title: "Night Drive", Artist: "Lumen"},
	{ID: "2", Title: "Open Road", Artist: "Northbound"},
	{ID: "3", Title: "Low Tide", Artist: "Harbour"},
	{ID: "4", Title: "Static Bloom", Artist: "Violet Grid"},
	{ID: "5", Title: "Afterlight", Artist: "Kite"},
}

type TrackService struct{}

func NewTrackService() *TrackService {
	return &TrackService{}
}

func (s *TrackService) List(_ context.Context, q domain.PageQuery) (domain.TrackPage, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	start := 0
	if q.Cursor != "" {
		n, err := strconv.Atoi(q.Cursor)
		if err != nil || n < 0 {
			return domain.TrackPage{}, apperrors.Invalid("invalid cursor")
		}
		start = n
	}
	if start > len(stubTracks) {
		start = len(stubTracks)
	}

	end := start + limit
	if end > len(stubTracks) {
		end = len(stubTracks)
	}

	page := domain.TrackPage{
		Items: stubTracks[start:end],
		Limit: limit,
	}
	if end < len(stubTracks) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

func (s *TrackService) GetByID(_ context.Context, id string) (domain.Track, error) {
	if id == "" {
		return domain.Track{}, apperrors.Invalid("track id is required")
	}
	for _, t := range stubTracks {
		if t.ID == id {
			return t, nil
		}
	}
	return domain.Track{}, apperrors.NotFound("track not found")
}

func (s *TrackService) Create(_ context.Context, in domain.TrackWrite) (domain.Track, error) {
	if in.Title == "" {
		return domain.Track{}, apperrors.Invalid("title is required")
	}
	return domain.Track{ID: "stub-track", Title: in.Title, Artist: in.Artist}, nil
}

func (s *TrackService) Update(_ context.Context, id string, in domain.TrackWrite) (domain.Track, error) {
	if id == "" {
		return domain.Track{}, apperrors.Invalid("track id is required")
	}
	if in.Title == "" {
		return domain.Track{}, apperrors.Invalid("title is required")
	}
	return domain.Track{ID: id, Title: in.Title, Artist: in.Artist}, nil
}

func (s *TrackService) Delete(_ context.Context, id string) error {
	if id == "" {
		return apperrors.Invalid("track id is required")
	}
	return nil
}
