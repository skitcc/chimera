package v1

import (
	"context"

	"chimera/internal/domain"
	"chimera/internal/transport/http/middleware"
)

type UserService interface {
	List(ctx context.Context, q domain.PageQuery) (domain.UserPage, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	Create(ctx context.Context, in domain.UserWrite) (domain.User, error)
	Replace(ctx context.Context, actorID, id string, in domain.UserReplace) (domain.User, error)
	Patch(ctx context.Context, actorID, id string, in domain.UserPatch) (domain.User, error)
	Delete(ctx context.Context, actorID, id string) error
}

type SessionService interface {
	Login(ctx context.Context, in domain.LoginInput) (domain.AuthResult, error)
}

type TrackService interface {
	Search(ctx context.Context, q domain.TrackListQuery) (domain.TrackPage, error)
	GetByID(ctx context.Context, id, viewerID string) (domain.Track, error)
	InitUpload(ctx context.Context, in domain.TrackUploadInit) (domain.TrackUploadSession, error)
	StreamURL(ctx context.Context, id string) (string, error)
	ReplaceOwned(ctx context.Context, actorID, id string, in domain.TrackReplace) (domain.Track, error)
	PatchOwned(ctx context.Context, actorID, id string, in domain.TrackPatch) (domain.Track, error)
	DeleteOwned(ctx context.Context, actorID, id string) error
	PutLike(ctx context.Context, actorID string, in domain.TrackLike) error
	DeleteLike(ctx context.Context, actorID string, in domain.TrackLike) error
}

type FollowService interface {
	List(ctx context.Context, q domain.FollowListQuery) (domain.FollowPage, error)
	Get(ctx context.Context, followerID, followingID string) (domain.Follow, error)
	Put(ctx context.Context, actorID, followerID, followingID string) error
	Delete(ctx context.Context, actorID, followerID, followingID string) error
	CreateMany(ctx context.Context, actorID string, in domain.FollowBatch) error
	DeleteMany(ctx context.Context, actorID string, in domain.FollowBatch) error
}

type PlaylistService interface {
	List(ctx context.Context, q domain.PlaylistListQuery) (domain.PlaylistPage, error)
	Create(ctx context.Context, ownerID string, in domain.PlaylistReplace) (domain.Playlist, error)
	Get(ctx context.Context, id, viewerID string) (domain.Playlist, error)
	Replace(ctx context.Context, actorID, id string, in domain.PlaylistReplace) (domain.Playlist, error)
	Patch(ctx context.Context, actorID, id string, in domain.PlaylistPatch) (domain.Playlist, error)
	Delete(ctx context.Context, actorID, id string) error
	ListTracks(ctx context.Context, id, viewerID string, q domain.PageQuery) (domain.PlaylistTrackPage, error)
	AddTrack(ctx context.Context, actorID, id string, in domain.PlaylistTrackAdd) (domain.PlaylistTrack, error)
	MoveTrack(ctx context.Context, actorID, id, trackID string, in domain.PlaylistTrackMove) (domain.PlaylistTrack, error)
	RemoveTrack(ctx context.Context, actorID, id, trackID string) error
}

type Logger = middleware.Logger
