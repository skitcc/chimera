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
	ReplaceCurrent(ctx context.Context, userID string, in domain.UserReplace) (domain.User, error)
	PatchCurrent(ctx context.Context, userID string, in domain.UserPatch) (domain.User, error)
	DeleteCurrent(ctx context.Context, userID string) error
}

type AuthService interface {
	Register(ctx context.Context, in domain.RegisterInput) (domain.AuthResult, error)
	Login(ctx context.Context, in domain.LoginInput) (domain.AuthResult, error)
}

type TrackService interface {
	List(ctx context.Context, q domain.TrackFeedQuery) (domain.TrackPage, error)
	ListByUploader(ctx context.Context, q domain.TrackOwnerQuery) (domain.TrackPage, error)
	ListLiked(ctx context.Context, q domain.TrackLikeListQuery) (domain.TrackPage, error)
	GetByID(ctx context.Context, id string) (domain.Track, error)
	Update(ctx context.Context, id string, in domain.TrackWrite) (domain.Track, error)
	Delete(ctx context.Context, id string) error
	InitUpload(ctx context.Context, in domain.TrackUploadInit) (domain.TrackUploadSession, error)
	CompleteUpload(ctx context.Context, in domain.TrackUploadComplete) (domain.Track, error)
	StreamURL(ctx context.Context, id string) (string, error)
	Like(ctx context.Context, in domain.TrackLike) error
	Unlike(ctx context.Context, in domain.TrackLike) error
	GetVisibleByID(ctx context.Context, id, viewerID string) (domain.Track, error)
	ReplaceOwned(ctx context.Context, actorID, id string, in domain.TrackReplace) (domain.Track, error)
	PatchOwned(ctx context.Context, actorID, id string, in domain.TrackPatch) (domain.Track, error)
	DeleteOwned(ctx context.Context, actorID, id string) error
	CompleteUploadOwned(ctx context.Context, in domain.TrackUploadComplete) (domain.Track, error)
}

type FollowService interface {
	Follow(ctx context.Context, actorID, userID string) error
	Unfollow(ctx context.Context, actorID, userID string) error
	ListFollowers(ctx context.Context, userID string, q domain.PageQuery) (domain.UserPage, error)
	ListFollowing(ctx context.Context, userID string, q domain.PageQuery) (domain.UserPage, error)
}

type PlaylistService interface {
	Create(ctx context.Context, ownerID string, in domain.PlaylistReplace) (domain.Playlist, error)
	Get(ctx context.Context, id, viewerID string) (domain.Playlist, error)
	ListByOwner(ctx context.Context, ownerID string, publicOnly bool, q domain.PageQuery) (domain.PlaylistPage, error)
	Replace(ctx context.Context, actorID, id string, in domain.PlaylistReplace) (domain.Playlist, error)
	Patch(ctx context.Context, actorID, id string, in domain.PlaylistPatch) (domain.Playlist, error)
	Delete(ctx context.Context, actorID, id string) error
	ListTracks(ctx context.Context, id, viewerID string, q domain.PageQuery) (domain.PlaylistTrackPage, error)
	AddTrack(ctx context.Context, actorID, id string, in domain.PlaylistTrackAdd) (domain.PlaylistTrack, error)
	MoveTrack(ctx context.Context, actorID, id, trackID string, in domain.PlaylistTrackMove) (domain.PlaylistTrack, error)
	RemoveTrack(ctx context.Context, actorID, id, trackID string) error
}

type PickService interface {
	Generate(ctx context.Context, userID string) (domain.PickDetail, error)
	List(ctx context.Context, userID string, q domain.PageQuery) (domain.PickPage, error)
	Get(ctx context.Context, userID, id string) (domain.PickDetail, error)
	Rename(ctx context.Context, userID, id string, in domain.PickPatch) (domain.Pick, error)
	Delete(ctx context.Context, userID, id string) error
}

type Logger interface {
	InfoContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}
