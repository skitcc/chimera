package v1

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

type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type TrackWriteRequest struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

type TrackUploadInitRequest struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Size   int64  `json:"size"`
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

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
