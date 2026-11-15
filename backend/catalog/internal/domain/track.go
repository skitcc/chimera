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

func (t Track) RequireOwner(userID string) error {
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

const maxBatchTracks = 1000

// TrackIDs is the lookup set of a batch read by another service.
type TrackIDs []string

func (ids TrackIDs) Validate() error {
	if len(ids) > maxBatchTracks {
		return Invalid("too many track ids")
	}
	for _, id := range ids {
		if err := TrackID(id).Validate(); err != nil {
			return err
		}
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

type TrackReplace struct {
	Title  string
	Artist string
}

func (in TrackReplace) Validate() error {
	if in.Title == "" {
		return Invalid("title is required")
	}
	return nil
}

type TrackPatch struct {
	Title   *string
	Artist  *string
	Publish bool
}

func (in TrackPatch) Validate() error {
	if in.Title == nil && in.Artist == nil && !in.Publish {
		return Invalid("at least one field is required")
	}
	if in.Title != nil && *in.Title == "" {
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

// TrackFilter selects tracks. An empty Status matches every status.
// With ByIDs set only tracks in IDs match, so an empty IDs matches nothing.
type TrackFilter struct {
	Status TrackStatus
	UserID string
	Artist string
	ByIDs  bool
	IDs    []string
}

// TrackListQuery is one listing for the feed and an author's tracks. Social
// lists liked tracks through it by passing the liked ids in TrackIDs.
// ViewerID is the token owner or empty for a guest.
type TrackListQuery struct {
	PageQuery
	ViewerID string
	UserID   string
	Artist   string
	Status   TrackStatus
	ByIDs    bool
	TrackIDs []string
}

func (q *TrackListQuery) Validate() error {
	q.Artist = strings.TrimSpace(q.Artist)
	switch q.Status {
	case "", TrackPending, TrackProcessing, TrackReady:
	default:
		return Invalid("invalid status")
	}
	if len(q.TrackIDs) > maxBatchTracks {
		return Invalid("too many track ids")
	}
	if q.Status != "" && q.Status != TrackReady && !q.own() {
		return Forbidden("not allowed")
	}
	return q.PageQuery.Validate()
}

func (q TrackListQuery) Filter() TrackFilter {
	status := q.Status
	if status == "" && !q.own() {
		status = TrackReady
	}
	return TrackFilter{Status: status, UserID: q.UserID, Artist: q.Artist, ByIDs: q.ByIDs, IDs: q.TrackIDs}
}

func (q TrackListQuery) own() bool {
	return q.UserID != "" && q.UserID == q.ViewerID
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

// UserDeletion is the UserDeleted event from Identity. EventID makes the
// cleanup idempotent when Kafka redelivers the event.
type UserDeletion struct {
	EventID string
	UserID  string
}

func (d UserDeletion) Validate() error {
	if d.EventID == "" {
		return Invalid("event id is required")
	}
	if !validUUID(d.EventID) {
		return Invalid("invalid event id")
	}
	return UserID(d.UserID).Validate()
}

// DeletedTrack is what the outbox announces when a track row goes away.
type DeletedTrack struct {
	ID        string
	UserID    string
	ObjectKey string
}

type ObjectKey string

func (k ObjectKey) Validate() error {
	if k == "" {
		return Invalid("object key is required")
	}
	return nil
}
