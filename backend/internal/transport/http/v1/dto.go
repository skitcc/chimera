package v1

import "time"

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserWriteRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password,omitempty"`
}

type UserReplaceRequest struct {
	Email *string `json:"email"`
	Name  *string `json:"name"`
}

type UserPatchRequest struct {
	Email    *string `json:"email"`
	Name     *string `json:"name"`
	Password *string `json:"password"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type TrackWriteRequest struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

type TrackReplaceRequest struct {
	Title  *string `json:"title"`
	Artist *string `json:"artist"`
}

type TrackPatchRequest struct {
	Title  *string `json:"title"`
	Artist *string `json:"artist"`
}

type TrackUploadInitRequest struct {
	Title  string  `json:"title"`
	Artist *string `json:"artist"`
	Size   int64   `json:"size"`
}

type TrackResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Status    string `json:"status"`
	SizeBytes int64  `json:"size_bytes"`
}

type TrackUploadInitResponse struct {
	Track     TrackResponse `json:"track"`
	UploadURL string        `json:"upload_url"`
}

type TrackPageResponse struct {
	Items      []TrackResponse `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
	Limit      int             `json:"limit"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type UserPageResponse struct {
	Items      []UserResponse `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
	Limit      int            `json:"limit"`
}

type PlaylistReplaceRequest struct {
	Title    *string `json:"title"`
	IsPublic *bool   `json:"is_public"`
}

type PlaylistPatchRequest struct {
	Title    *string `json:"title"`
	IsPublic *bool   `json:"is_public"`
}

type PlaylistResponse struct {
	ID       string `json:"id"`
	OwnerID  string `json:"owner_id"`
	Title    string `json:"title"`
	IsPublic bool   `json:"is_public"`
}

type PlaylistPageResponse struct {
	Items      []PlaylistResponse `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
	Limit      int                `json:"limit"`
}

type PlaylistTrackAddRequest struct {
	TrackID  string `json:"track_id"`
	Position *int   `json:"position"`
}

type PlaylistTrackMoveRequest struct {
	Position *int `json:"position"`
}

type PlaylistTrackResponse struct {
	Position int           `json:"position"`
	Track    TrackResponse `json:"track"`
}

type PlaylistTrackPageResponse struct {
	Items      []PlaylistTrackResponse `json:"items"`
	NextCursor string                  `json:"next_cursor,omitempty"`
	Limit      int                     `json:"limit"`
}

type PickPatchRequest struct {
	Title *string `json:"title"`
}

type PickResponse struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Title       string    `json:"title"`
	GeneratedAt time.Time `json:"generated_at"`
}

type PickTrackResponse struct {
	Position int           `json:"position"`
	Note     string        `json:"note"`
	Track    TrackResponse `json:"track"`
}

type PickDetailResponse struct {
	PickResponse
	Tracks []PickTrackResponse `json:"tracks"`
}

type PickPageResponse struct {
	Items      []PickResponse `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
	Limit      int            `json:"limit"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
