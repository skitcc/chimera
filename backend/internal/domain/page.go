package domain

import "strconv"

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

type PageQuery struct {
	Limit        int
	Cursor       string
	start        int
	invalidLimit bool
}

func NewPageQuery(limit int, cursor string, invalidLimit bool) PageQuery {
	return PageQuery{Limit: limit, Cursor: cursor, invalidLimit: invalidLimit}
}

func (q *PageQuery) Validate() error {
	if q.invalidLimit {
		return Invalid("invalid limit")
	}
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
	items, next := pageItems(q, tracks)
	page := TrackPage{
		Items: items,
		Limit: q.Limit,
	}
	page.NextCursor = next
	return page
}

type TrackPage struct {
	Items      []Track
	NextCursor string
	Limit      int
}

type UserPage struct {
	Items      []User
	NextCursor string
	Limit      int
}

func (q PageQuery) PageUsers(users []User) UserPage {
	items, next := pageItems(q, users)
	return UserPage{Items: items, NextCursor: next, Limit: q.Limit}
}

type PlaylistPage struct {
	Items      []Playlist
	NextCursor string
	Limit      int
}

func (q PageQuery) PagePlaylists(playlists []Playlist) PlaylistPage {
	items, next := pageItems(q, playlists)
	return PlaylistPage{Items: items, NextCursor: next, Limit: q.Limit}
}

type PlaylistTrackPage struct {
	Items      []PlaylistTrack
	NextCursor string
	Limit      int
}

func (q PageQuery) PagePlaylistTracks(tracks []PlaylistTrack) PlaylistTrackPage {
	items, next := pageItems(q, tracks)
	return PlaylistTrackPage{Items: items, NextCursor: next, Limit: q.Limit}
}

type PickPage struct {
	Items      []Pick
	NextCursor string
	Limit      int
}

func (q PageQuery) PagePicks(picks []Pick) PickPage {
	items, next := pageItems(q, picks)
	return PickPage{Items: items, NextCursor: next, Limit: q.Limit}
}

func pageItems[T any](q PageQuery, all []T) ([]T, string) {
	start := min(q.start, len(all))
	end := min(len(all), start+q.Limit)
	next := ""
	if end < len(all) {
		next = strconv.Itoa(end)
	}
	return all[start:end], next
}
