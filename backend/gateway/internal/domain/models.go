// Package domain holds the models the public API exchanges with the services.
// The Gateway has no business rules: Identity, Catalog and Social validate
// every request and own the error messages.
package domain

import "time"

type PageQuery struct {
	Limit        int
	Cursor       string
	InvalidLimit bool
}

func NewPageQuery(limit int, cursor string, invalidLimit bool) PageQuery {
	return PageQuery{Limit: limit, Cursor: cursor, InvalidLimit: invalidLimit}
}

type User struct {
	ID    string
	Email string
	Name  string
}

type UserPage struct {
	Items      []User
	NextCursor string
	Limit      int
}

type UserWrite struct {
	Email    string
	Name     string
	Password string
}

type UserReplace struct {
	Email string
	Name  string
}

type UserPatch struct {
	Email    *string
	Name     *string
	Password *string
}

type LoginInput struct {
	Email    string
	Password string
}

type AuthResult struct {
	ChallengeID string
	ExpiresAt   int64
	Token       string
	User        User
}

type TrackStatus string

const TrackReady TrackStatus = "ready"

type Track struct {
	ID        string
	UserID    string
	Title     string
	Artist    string
	SizeBytes int64
	Status    TrackStatus
}

type TrackPage struct {
	Items      []Track
	NextCursor string
	Limit      int
}

type TrackListQuery struct {
	PageQuery
	ViewerID string
	UserID   string
	Artist   string
	LikedBy  string
	Status   TrackStatus
}

type TrackUploadInit struct {
	UserID    string
	Title     string
	Artist    string
	SizeBytes int64
}

type TrackUploadSession struct {
	Track     Track
	UploadURL string
}

type TrackReplace struct {
	Title  string
	Artist string
}

type TrackPatch struct {
	Title   *string
	Artist  *string
	Publish bool
}

type TrackLike struct {
	UserID  string
	TrackID string
}

type Follow struct {
	Follower  User
	Following User
	CreatedAt time.Time
}

type FollowPage struct {
	Items      []Follow
	NextCursor string
	Limit      int
}

type FollowListQuery struct {
	PageQuery
	FollowerID  string
	FollowingID string
}

type FollowBatch struct {
	FollowerID   string
	FollowingIDs []string
}

type Playlist struct {
	ID       string
	OwnerID  string
	Title    string
	IsPublic bool
}

type PlaylistPage struct {
	Items      []Playlist
	NextCursor string
	Limit      int
}

type PlaylistListQuery struct {
	PageQuery
	ViewerID string
	OwnerID  string
	IsPublic *bool
}

type PlaylistReplace struct {
	Title    string
	IsPublic bool
}

type PlaylistPatch struct {
	Title    *string
	IsPublic *bool
}

type PlaylistTrack struct {
	Position int
	Track    Track
}

type PlaylistTrackPage struct {
	Items      []PlaylistTrack
	NextCursor string
	Limit      int
}

type PlaylistTrackAdd struct {
	TrackID  string
	Position int
}

type PlaylistTrackMove struct {
	Position int
}
