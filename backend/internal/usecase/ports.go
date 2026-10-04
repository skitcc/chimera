package usecase

import (
	"context"

	"chimera/internal/domain"
)

type UserRepository interface {
	List(ctx context.Context) ([]domain.User, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	Create(ctx context.Context, u domain.User, passwordHash string) (domain.User, error)
	Update(ctx context.Context, u domain.User) (domain.User, error)
	Delete(ctx context.Context, id string) error
}

type ProfileRepository interface {
	UpdateProfile(ctx context.Context, user domain.User, passwordHash *string) (domain.User, error)
}

type AuthUserRepository interface {
	Create(ctx context.Context, u domain.User, passwordHash string) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.AuthUser, error)
}

type TrackRepository interface {
	List(ctx context.Context, filter domain.TrackFilter) ([]domain.Track, error)
	GetByID(ctx context.Context, id string) (domain.Track, error)
	Create(ctx context.Context, t domain.Track) (domain.Track, error)
	Update(ctx context.Context, t domain.Track) (domain.Track, error)
	Delete(ctx context.Context, id string) error
}

type TrackLikeRepository interface {
	Add(ctx context.Context, userID, trackID string) error
	Remove(ctx context.Context, userID, trackID string) error
	ListReadyByUser(ctx context.Context, userID string) ([]domain.Track, error)
}

type TrackSelectionRepository interface {
	ListRandomReady(ctx context.Context, limit int) ([]domain.Track, error)
}

type FollowRepository interface {
	Add(ctx context.Context, followerID, followingID string) error
	Remove(ctx context.Context, followerID, followingID string) error
	ListFollowers(ctx context.Context, userID string) ([]domain.User, error)
	ListFollowing(ctx context.Context, userID string) ([]domain.User, error)
}

type PlaylistRepository interface {
	Create(ctx context.Context, playlist domain.Playlist) (domain.Playlist, error)
	GetByID(ctx context.Context, id string) (domain.Playlist, error)
	Update(ctx context.Context, playlist domain.Playlist) (domain.Playlist, error)
	Delete(ctx context.Context, id string) error
	ListByOwner(ctx context.Context, ownerID string, publicOnly bool) ([]domain.Playlist, error)
	ListTracks(ctx context.Context, playlistID string) ([]domain.PlaylistTrack, error)
	AddTrack(ctx context.Context, playlistID string, in domain.PlaylistTrackAdd) (domain.PlaylistTrack, error)
	MoveTrack(ctx context.Context, playlistID, trackID string, position int) (domain.PlaylistTrack, error)
	RemoveTrack(ctx context.Context, playlistID, trackID string) error
}

type PickRepository interface {
	CreateRandom(ctx context.Context, userID, title string, tracks []domain.Track) (domain.PickDetail, error)
	ListByUser(ctx context.Context, userID string) ([]domain.Pick, error)
	GetByID(ctx context.Context, id string) (domain.PickDetail, error)
	UpdateTitle(ctx context.Context, id, title string) (domain.Pick, error)
	Delete(ctx context.Context, id string) error
}

type ObjectStorage interface {
	PresignPut(ctx context.Context, key string) (string, error)
	PresignGet(ctx context.Context, key string) (string, error)
	Stat(ctx context.Context, key string) (int64, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenIssuer interface {
	Issue(user domain.User) (string, error)
}
