package domain

import "strconv"

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

type PageQuery struct {
	Limit  int
	Cursor string
	start  int
}

func (q *PageQuery) Validate() error {
	if q.Limit <= 0 {
		q.Limit = defaultPageLimit
	}
	if q.Limit > maxPageLimit {
		return Invalid("limit exceeded")
	}
	if q.Cursor == "" {
		q.start = 0
		return nil
	}
	n, err := strconv.Atoi(q.Cursor)
	if err != nil || n < 0 {
		return Invalid("invalid cursor")
	}
	q.start = n
	return nil
}

func (q PageQuery) Page(tracks []Track) TrackPage {
	start := min(q.start, len(tracks))
	end := min(len(tracks), start+q.Limit)
	page := TrackPage{
		Items: tracks[start:end],
		Limit: q.Limit,
	}
	if end < len(tracks) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page
}

type TrackPage struct {
	Items      []Track
	NextCursor string
	Limit      int
}
