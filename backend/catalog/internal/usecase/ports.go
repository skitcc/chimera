package usecase

import (
	"context"

	"chimera/catalog/internal/domain"
)

type TrackRepository interface {
	List(ctx context.Context, filter domain.TrackFilter) ([]domain.Track, error)
	GetByID(ctx context.Context, id string) (domain.Track, error)
	GetMany(ctx context.Context, ids []string) ([]domain.Track, error)
	Create(ctx context.Context, t domain.Track) (domain.Track, error)
	Update(ctx context.Context, t domain.Track) (domain.Track, error)
	// Delete removes the track and records TrackDeleted in the same commit.
	Delete(ctx context.Context, id string) error
}

type UserTrackRemover interface {
	// DeleteByUser drops the user's tracks, blocks new ones and records
	// TrackDeleted for each removed track. A repeated event changes nothing.
	DeleteByUser(ctx context.Context, d domain.UserDeletion) error
}

type ObjectStorage interface {
	PresignPut(ctx context.Context, key string) (string, error)
	PresignGet(ctx context.Context, key string) (string, error)
	Stat(ctx context.Context, key string) (int64, error)
}

type ObjectRemover interface {
	Remove(ctx context.Context, key string) error
}
