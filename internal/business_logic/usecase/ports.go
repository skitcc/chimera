package usecase

import (
	"context"

	"chimera/internal/business_logic/domain"
)

type UserRepository interface {
	List(ctx context.Context) ([]domain.User, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	Create(ctx context.Context, u domain.User) (domain.User, error)
	Update(ctx context.Context, u domain.User) (domain.User, error)
	Delete(ctx context.Context, id string) error
}

type TrackRepository interface {
	List(ctx context.Context) ([]domain.Track, error)
	GetByID(ctx context.Context, id string) (domain.Track, error)
	Create(ctx context.Context, t domain.Track) (domain.Track, error)
	Update(ctx context.Context, t domain.Track) (domain.Track, error)
	Delete(ctx context.Context, id string) error
}
