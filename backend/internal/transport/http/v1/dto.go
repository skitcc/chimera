package v1

import "time"

type SessionCreate struct {
	Email    string `json:"email" binding:"required" format:"email" example:"ada@example.com"`
	Password string `json:"password" binding:"required" example:"long-enough"`
} // @name SessionCreate

type Session struct {
	Token  string `json:"token" binding:"required" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.example"`
	UserID string `json:"user_id" binding:"required" format:"uuid" example:"6b1e2c3a-7d4f-4a1b-9c8e-111111111111"` // Владелец токена. Подставляется в пути и фильтры вместо текущего пользователя.
} // @name Session

type User struct {
	ID    string `json:"id" binding:"required" format:"uuid" example:"6b1e2c3a-7d4f-4a1b-9c8e-111111111111"`
	Email string `json:"email" binding:"required" format:"email" example:"ada@example.com"`
	Name  string `json:"name" binding:"required" example:"Ada"`
} // @name User

type UserCreate struct {
	Email    string `json:"email" binding:"required" format:"email" example:"ada@example.com"`
	Password string `json:"password" binding:"required" minLength:"8" example:"long-enough"`
	Name     string `json:"name" example:"Ada"`
} // @name UserCreate

type UserReplace struct {
	Email *string `json:"email" binding:"required" format:"email" example:"ada@example.com"`
	Name  *string `json:"name" binding:"required" example:"Ada Lovelace"`
} // @name UserReplace

type UserPatch struct {
	Email    *string `json:"email" format:"email"`
	Name     *string `json:"name" example:"Ada Lovelace"`
	Password *string `json:"password" minLength:"8"` // Если поле передано, пароль заменяется. Иначе остаётся прежним.
} // @name UserPatch

type UserPage struct {
	Items      []User `json:"items" binding:"required"`
	NextCursor string `json:"next_cursor,omitempty" example:"20"`
	Limit      int    `json:"limit" binding:"required" example:"20"`
} // @name UserPage

type Follow struct {
	Follower  User      `json:"follower" binding:"required"`
	Following User      `json:"following" binding:"required"`
	CreatedAt time.Time `json:"created_at" binding:"required" format:"date-time" example:"2026-09-25T09:00:00Z"`
} // @name Follow

type FollowPage struct {
	Items      []Follow `json:"items" binding:"required"`
	NextCursor string   `json:"next_cursor,omitempty" example:"20"`
	Limit      int      `json:"limit" binding:"required" example:"20"`
} // @name FollowPage

type FollowsCreate struct {
	FollowerID   string   `json:"follower_id" binding:"required" format:"uuid" example:"6b1e2c3a-7d4f-4a1b-9c8e-111111111111"` // Кто подписывается. Должен совпадать с владельцем токена.
	FollowingIDs []string `json:"following_ids" validate:"required,min=1,max=100,unique" format:"uuid"`                        // На кого подписаться
} // @name FollowsCreate

type Track struct {
	ID        string `json:"id" binding:"required" format:"uuid" example:"7c2f3d4b-8e5a-4b2c-ad9f-222222222222"`
	UserID    string `json:"user_id" binding:"required" format:"uuid" example:"6b1e2c3a-7d4f-4a1b-9c8e-111111111111"`
	Title     string `json:"title" binding:"required" example:"Night Drive"`
	Artist    string `json:"artist" binding:"required" example:"Ada"` // Строка для поиска, не аккаунт
	Status    string `json:"status" binding:"required" enums:"pending,processing,ready" example:"ready"`
	SizeBytes int64  `json:"size_bytes" binding:"required" format:"int64" minimum:"0" example:"3200000"`
} // @name Track

type TrackCreate struct {
	Title  string  `json:"title" binding:"required" minLength:"1" example:"Night Drive"`
	Artist *string `json:"artist" binding:"required" example:"Ada"`
	Size   int64   `json:"size" binding:"required" format:"int64" minimum:"1" example:"3200000"` // Размер файла в байтах
} // @name TrackCreate

type TrackReplace struct {
	Title  *string `json:"title" binding:"required" minLength:"1" example:"Night Drive"`
	Artist *string `json:"artist" binding:"required" example:"Ada"`
} // @name TrackReplace

type TrackPatch struct {
	Title  *string `json:"title" minLength:"1" example:"Night Drive (demo)"`
	Artist *string `json:"artist"`
	Status *string `json:"status" enums:"ready"` // Единственный переход, который задаёт клиент, — публикация загруженного файла.
} // @name TrackPatch

type TrackUpload struct {
	Track     Track  `json:"track" binding:"required"`
	UploadURL string `json:"upload_url" binding:"required" format:"uri" example:"https://storage.example/music-raw/7c2f3d4b-8e5a-4b2c-ad9f-222222222222.mp3?X-Amz-Signature=example"`
} // @name TrackUpload

type TrackPage struct {
	Items      []Track `json:"items" binding:"required"`
	NextCursor string  `json:"next_cursor,omitempty" example:"20"`
	Limit      int     `json:"limit" binding:"required" example:"20"`
} // @name TrackPage

type Playlist struct {
	ID       string `json:"id" binding:"required" format:"uuid" example:"8d3a4e5c-9f6b-4c3d-be0a-333333333333"`
	OwnerID  string `json:"owner_id" binding:"required" format:"uuid" example:"6b1e2c3a-7d4f-4a1b-9c8e-111111111111"`
	Title    string `json:"title" binding:"required" example:"Ночные"`
	IsPublic bool   `json:"is_public" binding:"required" example:"true"`
} // @name Playlist

type PlaylistReplace struct {
	Title    *string `json:"title" binding:"required" minLength:"1" example:"Ночные"`
	IsPublic *bool   `json:"is_public" binding:"required" example:"false"`
} // @name PlaylistReplace

type PlaylistPatch struct {
	Title    *string `json:"title" minLength:"1"`
	IsPublic *bool   `json:"is_public" example:"true"`
} // @name PlaylistPatch

type PlaylistPage struct {
	Items      []Playlist `json:"items" binding:"required"`
	NextCursor string     `json:"next_cursor,omitempty"`
	Limit      int        `json:"limit" binding:"required" example:"20"`
} // @name PlaylistPage

type PlaylistTrack struct {
	Position int   `json:"position" binding:"required" minimum:"0" example:"1"`
	Track    Track `json:"track" binding:"required"`
} // @name PlaylistTrack

type PlaylistTrackCreate struct {
	TrackID  string `json:"track_id" binding:"required" format:"uuid" example:"7c2f3d4b-8e5a-4b2c-ad9f-222222222222"`
	Position *int   `json:"position" binding:"required" minimum:"0" example:"1"`
} // @name PlaylistTrackCreate

type PlaylistTrackPatch struct {
	Position *int `json:"position" binding:"required" minimum:"0" example:"2"`
} // @name PlaylistTrackPatch

type PlaylistTrackPage struct {
	Items      []PlaylistTrack `json:"items" binding:"required"`
	NextCursor string          `json:"next_cursor,omitempty"`
	Limit      int             `json:"limit" binding:"required" example:"20"`
} // @name PlaylistTrackPage

type ErrorResponse struct {
	Code    string `json:"code" binding:"required" enums:"invalid,unauthorized,forbidden,not_found,conflict,too_many_requests,internal"`
	Message string `json:"message" binding:"required"`
} // @name Error
