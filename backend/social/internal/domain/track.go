package domain

import "strings"

type TrackStatus string

const (
	TrackPending    TrackStatus = "pending"
	TrackProcessing TrackStatus = "processing"
	TrackReady      TrackStatus = "ready"
)

// Track is the Catalog view of a track. Social keeps only track ids.
type Track struct {
	ID        string
	UserID    string
	Title     string
	Artist    string
	SizeBytes int64
	Status    TrackStatus
}

func (t Track) EnsureReady() error {
	if t.Status != TrackReady {
		return Conflict("track is not ready")
	}
	return nil
}

type TrackID string

func (id TrackID) Validate() error {
	if id == "" {
		return Invalid("track id is required")
	}
	return nil
}

type TrackLike struct {
	UserID  string
	TrackID string
}

func (in TrackLike) Validate() error {
	if err := UserID(in.UserID).Validate(); err != nil {
		return err
	}
	return TrackID(in.TrackID).Validate()
}

// LikedTracksQuery lists tracks liked by LikedBy. Social checks who may see
// the likes; Catalog applies the status, owner and paging rules to the ids.
type LikedTracksQuery struct {
	Page     PageQuery
	ViewerID string
	LikedBy  string
	UserID   string
	Artist   string
	Status   TrackStatus
}

func (q *LikedTracksQuery) Validate() error {
	q.Artist = strings.TrimSpace(q.Artist)
	switch q.Status {
	case "", TrackPending, TrackProcessing, TrackReady:
	default:
		return Invalid("invalid status")
	}
	if q.ViewerID == "" {
		return Unauthorized("missing token")
	}
	if q.LikedBy != q.ViewerID {
		return Forbidden("not allowed")
	}
	return nil
}

type TrackPage struct {
	Items      []Track
	NextCursor string
	Limit      int
}
