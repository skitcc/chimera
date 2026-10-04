package v1

import (
	"context"

	"chimera/internal/domain"
	"chimera/internal/transport/http/middleware"
)

type UserService interface {
	List(ctx context.Context) ([]domain.User, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	Create(ctx context.Context, in domain.UserWrite) (domain.User, error)
	Update(ctx context.Context, actorID, id string, in domain.UserWrite) (domain.User, error)
	Delete(ctx context.Context, actorID, id string) error
}

type AuthService interface {
	Register(ctx context.Context, in domain.RegisterInput) (domain.AuthResult, error)
	Login(ctx context.Context, in domain.LoginInput) (domain.AuthResult, error)
}

type TrackService interface {
	List(ctx context.Context, q domain.TrackFeedQuery) (domain.TrackPage, error)
	ListByUploader(ctx context.Context, q domain.TrackOwnerQuery) (domain.TrackPage, error)
	ListLiked(ctx context.Context, q domain.TrackLikeListQuery) (domain.TrackPage, error)
	GetByID(ctx context.Context, id, viewerID string) (domain.Track, error)
	Update(ctx context.Context, actorID, id string, in domain.TrackWrite) (domain.Track, error)
	Delete(ctx context.Context, actorID, id string) error
	InitUpload(ctx context.Context, in domain.TrackUploadInit) (domain.TrackUploadSession, error)
	CompleteUpload(ctx context.Context, in domain.TrackUploadComplete) (domain.Track, error)
	StreamURL(ctx context.Context, id string) (string, error)
	Like(ctx context.Context, in domain.TrackLike) error
	Unlike(ctx context.Context, in domain.TrackLike) error
}

type Logger = middleware.Logger
