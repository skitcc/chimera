package testkit

import "chimera/catalog/internal/domain"

const (
	defaultUserID    = "user-123"
	defaultTrackID   = "track-123"
	defaultTitle     = "Test Track"
	defaultArtist    = "Test Artist"
	defaultObjectKey = "tracks/test-track.mp3"
	defaultSizeBytes = int64(1024)
)

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
