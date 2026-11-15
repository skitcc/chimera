package usecase

import (
	"context"

	"chimera/social/internal/domain"
)

type TrackLikeRepository interface {
	Add(ctx context.Context, userID, trackID string) error
	Remove(ctx context.Context, userID, trackID string) error
	ListTrackIDs(ctx context.Context, userID string) ([]string, error)
}

type FollowRepository interface {
	Add(ctx context.Context, followerID, followingID string) error
	Remove(ctx context.Context, followerID, followingID string) error
	AddMany(ctx context.Context, followerID string, followingIDs []string) error
	RemoveMany(ctx context.Context, followerID string, followingIDs []string) error
	List(ctx context.Context, filter domain.FollowFilter) ([]domain.FollowEdge, error)
}

type PlaylistRepository interface {
	Create(ctx context.Context, playlist domain.Playlist) (domain.Playlist, error)
	GetByID(ctx context.Context, id string) (domain.Playlist, error)
	Update(ctx context.Context, playlist domain.Playlist) (domain.Playlist, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter domain.PlaylistFilter) ([]domain.Playlist, error)
	ListEntries(ctx context.Context, playlistID string) ([]domain.PlaylistEntry, error)
	AddTrack(ctx context.Context, playlistID string, in domain.PlaylistTrackAdd) (domain.PlaylistEntry, error)
	MoveTrack(ctx context.Context, playlistID, trackID string, position int) (domain.PlaylistEntry, error)
	RemoveTrack(ctx context.Context, playlistID, trackID string) error
}

// CleanupRepository applies deletion events. A repeated event changes nothing.
type CleanupRepository interface {
	DeleteUser(ctx context.Context, d domain.UserDeletion) error
	DeleteTrack(ctx context.Context, d domain.TrackDeletion) error
}

// TrackCatalog is the Catalog service.
type TrackCatalog interface {
	// GetTrack returns the track in any status or not_found "track not found".
	GetTrack(ctx context.Context, id string) (domain.Track, error)
	// GetTracks returns the existing tracks among ids, in no particular order.
	GetTracks(ctx context.Context, ids []string) ([]domain.Track, error)
	// ListByIDs pages the given tracks with the Catalog listing rules.
	ListByIDs(ctx context.Context, q domain.LikedTracksQuery, ids []string) (domain.TrackPage, error)
}

// UserDirectory is the Identity service.
type UserDirectory interface {
	GetUser(ctx context.Context, id string) (domain.User, error)
	// GetUsers returns the existing users among ids, in no particular order.
	GetUsers(ctx context.Context, ids []string) ([]domain.User, error)
}
