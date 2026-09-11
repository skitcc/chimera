package v1

import (
	"context"

	"chimera/internal/domain"
)

type UserService interface {
	List(ctx context.Context) ([]domain.User, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	Create(ctx context.Context, in domain.UserWrite) (domain.User, error)
	Update(ctx context.Context, id string, in domain.UserWrite) (domain.User, error)
	Delete(ctx context.Context, id string) error
}

type AuthService interface {
	Register(ctx context.Context, in domain.RegisterInput) (domain.AuthResult, error)
	Login(ctx context.Context, in domain.LoginInput) (domain.AuthResult, error)
}

type TrackService interface {
	List(ctx context.Context, q domain.PageQuery) (domain.TrackPage, error)
	GetByID(ctx context.Context, id string) (domain.Track, error)
	Create(ctx context.Context, in domain.TrackWrite) (domain.Track, error)
	Update(ctx context.Context, id string, in domain.TrackWrite) (domain.Track, error)
	Delete(ctx context.Context, id string) error
	InitUpload(ctx context.Context, in domain.TrackUploadInit) (domain.TrackUploadSession, error)
	CompleteUpload(ctx context.Context, in domain.TrackUploadComplete) (domain.Track, error)
}

type Logger interface {
	InfoContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}
