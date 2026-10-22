package domain

type Playlist struct {
	ID       string
	OwnerID  string
	Title    string
	IsPublic bool
}

func (p Playlist) RequireOwner(userID string) error {
	if p.OwnerID != userID {
		return Forbidden("not allowed")
	}
	return nil
}

func (p Playlist) VisibleTo(userID string) error {
	if p.IsPublic || userID != "" && p.OwnerID == userID {
		return nil
	}
	return NotFound("playlist not found")
}

type PlaylistID string

func (id PlaylistID) Validate() error {
	if id == "" {
		return Invalid("playlist id is required")
	}
	return nil
}

type PlaylistReplace struct {
	Title    string
	IsPublic bool
}

func (in PlaylistReplace) Validate() error {
	if in.Title == "" {
		return Invalid("title is required")
	}
	return nil
}

type PlaylistPatch struct {
	Title    *string
	IsPublic *bool
}

func (in PlaylistPatch) Validate() error {
	if in.Title == nil && in.IsPublic == nil {
		return Invalid("at least one field is required")
	}
	if in.Title != nil && *in.Title == "" {
		return Invalid("title is required")
	}
	return nil
}

// PlaylistFilter selects playlists. A nil IsPublic matches both visibilities.
type PlaylistFilter struct {
	OwnerID  string
	IsPublic *bool
}

// PlaylistListQuery is one listing for the public feed and a user's playlists.
// ViewerID is the token owner or empty for a guest.
type PlaylistListQuery struct {
	PageQuery
	ViewerID string
	OwnerID  string
	IsPublic *bool
}

func (q *PlaylistListQuery) Validate() error {
	if q.IsPublic != nil && !*q.IsPublic && !q.own() {
		return Forbidden("not allowed")
	}
	return q.PageQuery.Validate()
}

func (q PlaylistListQuery) Filter() PlaylistFilter {
	isPublic := q.IsPublic
	if isPublic == nil && !q.own() {
		public := true
		isPublic = &public
	}
	return PlaylistFilter{OwnerID: q.OwnerID, IsPublic: isPublic}
}

func (q PlaylistListQuery) own() bool {
	return q.OwnerID != "" && q.OwnerID == q.ViewerID
}

type PlaylistTrack struct {
	Position int
	Track    Track
}

type PlaylistTrackAdd struct {
	TrackID  string
	Position int
}

func (in PlaylistTrackAdd) Validate() error {
	if err := TrackID(in.TrackID).Validate(); err != nil {
		return err
	}
	if in.Position < 0 {
		return Invalid("position must be non-negative")
	}
	return nil
}

type PlaylistTrackMove struct {
	Position int
}

func (in PlaylistTrackMove) Validate() error {
	if in.Position < 0 {
		return Invalid("position must be non-negative")
	}
	return nil
}
