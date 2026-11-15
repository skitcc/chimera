package testkit

import "chimera/social/internal/domain"

const (
	defaultUserID  = "user-123"
	defaultTrackID = "track-123"
)

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
