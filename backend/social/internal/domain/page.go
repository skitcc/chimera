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

// InvalidLimit reports a limit the caller sent but could not parse. Social
// forwards it to Catalog, which pages liked tracks.
func (q PageQuery) InvalidLimit() bool {
	return q.invalidLimit
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

type FollowPage struct {
	Items      []Follow
	NextCursor string
	Limit      int
}

func (q PageQuery) PageFollows(follows []Follow) FollowPage {
	items, next := pageItems(q, follows)
	return FollowPage{Items: items, NextCursor: next, Limit: q.Limit}
}

type FollowEdgePage struct {
	Items      []FollowEdge
	NextCursor string
	Limit      int
}

// PageFollowEdges pages stored follows before their profiles are fetched,
// so Identity is asked only for the users on this page.
func (q PageQuery) PageFollowEdges(edges []FollowEdge) FollowEdgePage {
	items, next := pageItems(q, edges)
	return FollowEdgePage{Items: items, NextCursor: next, Limit: q.Limit}
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

func pageItems[T any](q PageQuery, all []T) ([]T, string) {
	start := min(q.start, len(all))
	end := min(len(all), start+q.Limit)
	next := ""
	if end < len(all) {
		next = strconv.Itoa(end)
	}
	return all[start:end], next
}
