package domain

import (
	"strconv"
	"strings"
)

type TrackStatus string

const (
	TrackPending    TrackStatus = "pending"
	TrackProcessing TrackStatus = "processing"
	TrackReady      TrackStatus = "ready"
)

type Track struct {
	ID        string
	UserID    string
	Title     string
	Artist    string
	ObjectKey string
	SizeBytes int64
	Status    TrackStatus
}

func (t Track) AudioObjectKey() string {
	if t.ObjectKey != "" {
		return t.ObjectKey
	}
	return t.ID + ".mp3"
}

func (t Track) OwnedBy(userID string) error {
	if t.UserID != userID {
		return Forbidden("not allowed")
	}
	return nil
}

func (t Track) VisibleTo(userID string) error {
	if t.Status == TrackReady {
		return nil
	}
	if userID != "" && t.UserID == userID {
		return nil
	}
	return NotFound("track not found")
}

func (t Track) ConfirmUpload(size int64) error {
	if size <= 0 {
		return Invalid("upload not found")
	}
	if t.SizeBytes > 0 && size != t.SizeBytes {
		return Invalid("upload size mismatch: expected " + strconv.FormatInt(t.SizeBytes, 10) + ", got " + strconv.FormatInt(size, 10))
	}
	return nil
}

func (t *Track) MarkProcessing() error {
	switch t.Status {
	case TrackPending:
		t.Status = TrackProcessing
		return nil
	case TrackProcessing:
		return nil
	default:
		return Conflict("track is not awaiting upload")
	}
}

func (t *Track) MarkReady() error {
	if t.Status != TrackProcessing {
		return Conflict("track is not ready to publish")
	}
	t.Status = TrackReady
	return nil
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

type TrackWrite struct {
	Title  string
	Artist string
}

func (w TrackWrite) Validate() error {
	if w.Title == "" {
		return Invalid("title is required")
	}
	return nil
}

type TrackUploadInit struct {
	UserID    string
	Title     string
	Artist    string
	SizeBytes int64
}

func (in TrackUploadInit) Validate() error {
	if in.UserID == "" {
		return Invalid("user id is required")
	}
	if in.Title == "" {
		return Invalid("title is required")
	}
	if in.SizeBytes <= 0 {
		return Invalid("size is required")
	}
	return nil
}

func (in TrackUploadInit) ValidateSize(maxBytes int64) error {
	if maxBytes > 0 && in.SizeBytes > maxBytes {
		return Invalid("file too large")
	}
	return nil
}

func (in TrackUploadInit) Track() Track {
	return Track{
		UserID:    in.UserID,
		Title:     in.Title,
		Artist:    in.Artist,
		SizeBytes: in.SizeBytes,
		Status:    TrackPending,
	}
}

type TrackUploadComplete struct {
	TrackID string
	UserID  string
}

func (in TrackUploadComplete) Validate() error {
	if err := TrackID(in.TrackID).Validate(); err != nil {
		return err
	}
	if in.UserID == "" {
		return Invalid("user id is required")
	}
	return nil
}

type TrackUploadSession struct {
	Track     Track
	UploadURL string
}

type TrackFilter struct {
	Status TrackStatus
	UserID string
	Artist string
}

type TrackFeedQuery struct {
	PageQuery
	Artist string
}

func (q *TrackFeedQuery) Validate() error {
	q.Artist = strings.TrimSpace(q.Artist)
	return q.PageQuery.Validate()
}

func (q TrackFeedQuery) Filter() TrackFilter {
	return TrackFilter{Status: TrackReady, Artist: q.Artist}
}

type TrackOwnerQuery struct {
	PageQuery
	UserID string
	Status TrackStatus
}

func (q *TrackOwnerQuery) Validate() error {
	if err := UserID(q.UserID).Validate(); err != nil {
		return err
	}
	return q.PageQuery.Validate()
}

func (q TrackOwnerQuery) Filter() TrackFilter {
	return TrackFilter{UserID: q.UserID, Status: q.Status}
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

type TrackLikeListQuery struct {
	PageQuery
	UserID string
}

func (q *TrackLikeListQuery) Validate() error {
	if err := UserID(q.UserID).Validate(); err != nil {
		return err
	}
	return q.PageQuery.Validate()
}
