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

type TrackResponse struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
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
