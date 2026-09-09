package port

import (
	"context"

	"chimera/internal/business_logic/domain"
)

type TrackService interface {
	List(ctx context.Context, q domain.PageQuery) (domain.TrackPage, error)
	GetByID(ctx context.Context, id string) (domain.Track, error)
	Create(ctx context.Context, in domain.TrackWrite) (domain.Track, error)
	Update(ctx context.Context, id string, in domain.TrackWrite) (domain.Track, error)
	Delete(ctx context.Context, id string) error
}
