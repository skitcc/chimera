package testkit

import "chimera/internal/domain"

const (
	defaultUserID       = "user-123"
	defaultTrackID      = "track-123"
	defaultEmail        = "listener@example.com"
	defaultPassword     = "password123"
	defaultPasswordHash = "hashed-password"
	defaultName         = "Test Listener"
	defaultToken        = "test-token"
	defaultTitle        = "Test Track"
	defaultArtist       = "Test Artist"
	defaultObjectKey    = "tracks/test-track.mp3"
	defaultSizeBytes    = int64(1024)
)

type UserBuilder struct {
	user         domain.User
	password     string
	passwordHash string
}

func UserMother() UserBuilder {
	return UserBuilder{
		user: domain.User{
			ID:    defaultUserID,
			Email: defaultEmail,
			Name:  defaultName,
		},
		password:     defaultPassword,
		passwordHash: defaultPasswordHash,
	}
}

func (b UserBuilder) WithID(id string) UserBuilder {
	b.user.ID = id
	return b
}

func (b UserBuilder) WithEmail(email string) UserBuilder {
	b.user.Email = email
	return b
}

func (b UserBuilder) WithName(name string) UserBuilder {
	b.user.Name = name
	return b
}

func (b UserBuilder) WithPassword(password string) UserBuilder {
	b.password = password
	return b
}

func (b UserBuilder) WithPasswordHash(passwordHash string) UserBuilder {
	b.passwordHash = passwordHash
	return b
}

func (b UserBuilder) Build() domain.User {
	return b.user
}

func (b UserBuilder) BuildAuthUser() domain.AuthUser {
	return domain.AuthUser{User: b.user, PasswordHash: b.passwordHash}
}

func (b UserBuilder) BuildWrite() domain.UserWrite {
	return domain.UserWrite{
		Email:    b.user.Email,
		Name:     b.user.Name,
		Password: b.password,
	}
}

type AuthBuilder struct {
	email    string
	password string
	name     string
	token    string
	user     domain.User
}

func AuthMother() AuthBuilder {
	return AuthBuilder{
		email:    defaultEmail,
		password: defaultPassword,
		name:     defaultName,
		token:    defaultToken,
		user:     UserMother().Build(),
	}
}

func (b AuthBuilder) WithEmail(email string) AuthBuilder {
	b.email = email
	return b
}

func (b AuthBuilder) WithPassword(password string) AuthBuilder {
	b.password = password
	return b
}

func (b AuthBuilder) WithName(name string) AuthBuilder {
	b.name = name
	return b
}

func (b AuthBuilder) WithToken(token string) AuthBuilder {
	b.token = token
	return b
}

func (b AuthBuilder) WithUser(user domain.User) AuthBuilder {
	b.user = user
	return b
}

func (b AuthBuilder) BuildRegisterInput() domain.RegisterInput {
	return domain.RegisterInput{
		Email:    b.email,
		Password: b.password,
		Name:     b.name,
	}
}

func (b AuthBuilder) BuildLoginInput() domain.LoginInput {
	return domain.LoginInput{
		Email:    b.email,
		Password: b.password,
	}
}

func (b AuthBuilder) BuildResult() domain.AuthResult {
	return domain.AuthResult{Token: b.token, User: b.user}
}

type TrackBuilder struct {
	track domain.Track
}

func TrackMother() TrackBuilder {
	return TrackBuilder{
		track: domain.Track{
			ID:        defaultTrackID,
			UserID:    defaultUserID,
			Title:     defaultTitle,
			Artist:    defaultArtist,
			ObjectKey: defaultObjectKey,
			SizeBytes: defaultSizeBytes,
			Status:    domain.TrackReady,
		},
	}
}

func (b TrackBuilder) WithID(id string) TrackBuilder {
	b.track.ID = id
	return b
}

func (b TrackBuilder) WithUserID(userID string) TrackBuilder {
	b.track.UserID = userID
	return b
}

func (b TrackBuilder) WithTitle(title string) TrackBuilder {
	b.track.Title = title
	return b
}

func (b TrackBuilder) WithArtist(artist string) TrackBuilder {
	b.track.Artist = artist
	return b
}

func (b TrackBuilder) WithObjectKey(objectKey string) TrackBuilder {
	b.track.ObjectKey = objectKey
	return b
}

func (b TrackBuilder) WithSizeBytes(sizeBytes int64) TrackBuilder {
	b.track.SizeBytes = sizeBytes
	return b
}

func (b TrackBuilder) WithStatus(status domain.TrackStatus) TrackBuilder {
	b.track.Status = status
	return b
}

func (b TrackBuilder) Build() domain.Track {
	return b.track
}

func (b TrackBuilder) BuildWrite() domain.TrackWrite {
	return domain.TrackWrite{Title: b.track.Title, Artist: b.track.Artist}
}

func (b TrackBuilder) BuildUploadInit() domain.TrackUploadInit {
	return domain.TrackUploadInit{
		UserID:    b.track.UserID,
		Title:     b.track.Title,
		Artist:    b.track.Artist,
		SizeBytes: b.track.SizeBytes,
	}
}

func (b TrackBuilder) BuildUploadComplete() domain.TrackUploadComplete {
	return domain.TrackUploadComplete{
		TrackID: b.track.ID,
		UserID:  b.track.UserID,
	}
}

type TrackLikeBuilder struct {
	like domain.TrackLike
}

func TrackLikeMother() TrackLikeBuilder {
	return TrackLikeBuilder{
		like: domain.TrackLike{
			UserID:  defaultUserID,
			TrackID: defaultTrackID,
		},
	}
}

func (b TrackLikeBuilder) WithUserID(userID string) TrackLikeBuilder {
	b.like.UserID = userID
	return b
}

func (b TrackLikeBuilder) WithTrackID(trackID string) TrackLikeBuilder {
	b.like.TrackID = trackID
	return b
}

func (b TrackLikeBuilder) Build() domain.TrackLike {
	return b.like
}

type PageQueryBuilder struct {
	limit  int
	cursor string
}

func PageQueryMother() PageQueryBuilder {
	return PageQueryBuilder{limit: 20}
}

func (b PageQueryBuilder) WithLimit(limit int) PageQueryBuilder {
	b.limit = limit
	return b
}

func (b PageQueryBuilder) WithCursor(cursor string) PageQueryBuilder {
	b.cursor = cursor
	return b
}

func (b PageQueryBuilder) Build() domain.PageQuery {
	return domain.PageQuery{Limit: b.limit, Cursor: b.cursor}
}

type TrackFeedQueryBuilder struct {
	pageQuery domain.PageQuery
	artist    string
}

func TrackFeedQueryMother() TrackFeedQueryBuilder {
	return TrackFeedQueryBuilder{
		pageQuery: PageQueryMother().Build(),
		artist:    defaultArtist,
	}
}

func (b TrackFeedQueryBuilder) WithPageQuery(pageQuery domain.PageQuery) TrackFeedQueryBuilder {
	b.pageQuery = pageQuery
	return b
}

func (b TrackFeedQueryBuilder) WithArtist(artist string) TrackFeedQueryBuilder {
	b.artist = artist
	return b
}

func (b TrackFeedQueryBuilder) Build() domain.TrackFeedQuery {
	return domain.TrackFeedQuery{PageQuery: b.pageQuery, Artist: b.artist}
}

type TrackOwnerQueryBuilder struct {
	pageQuery domain.PageQuery
	userID    string
	status    domain.TrackStatus
}

func TrackOwnerQueryMother() TrackOwnerQueryBuilder {
	return TrackOwnerQueryBuilder{
		pageQuery: PageQueryMother().Build(),
		userID:    defaultUserID,
		status:    domain.TrackReady,
	}
}

func (b TrackOwnerQueryBuilder) WithPageQuery(pageQuery domain.PageQuery) TrackOwnerQueryBuilder {
	b.pageQuery = pageQuery
	return b
}

func (b TrackOwnerQueryBuilder) WithUserID(userID string) TrackOwnerQueryBuilder {
	b.userID = userID
	return b
}

func (b TrackOwnerQueryBuilder) WithStatus(status domain.TrackStatus) TrackOwnerQueryBuilder {
	b.status = status
	return b
}

func (b TrackOwnerQueryBuilder) Build() domain.TrackOwnerQuery {
	return domain.TrackOwnerQuery{
		PageQuery: b.pageQuery,
		UserID:    b.userID,
		Status:    b.status,
	}
}

type TrackLikeListQueryBuilder struct {
	pageQuery domain.PageQuery
	userID    string
}

func TrackLikeListQueryMother() TrackLikeListQueryBuilder {
	return TrackLikeListQueryBuilder{
		pageQuery: PageQueryMother().Build(),
		userID:    defaultUserID,
	}
}

func (b TrackLikeListQueryBuilder) WithPageQuery(pageQuery domain.PageQuery) TrackLikeListQueryBuilder {
	b.pageQuery = pageQuery
	return b
}

func (b TrackLikeListQueryBuilder) WithUserID(userID string) TrackLikeListQueryBuilder {
	b.userID = userID
	return b
}

func (b TrackLikeListQueryBuilder) Build() domain.TrackLikeListQuery {
	return domain.TrackLikeListQuery{
		PageQuery: b.pageQuery,
		UserID:    b.userID,
	}
}
