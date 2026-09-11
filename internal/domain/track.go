package domain

import "strconv"

type TrackStatus string

const (
	TrackPending    TrackStatus = "pending"
	TrackProcessing TrackStatus = "processing"
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
		return Unauthorized("not allowed")
	}
	return nil
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
	if t.Status != TrackPending {
		return Conflict("track is not awaiting upload")
	}
	t.Status = TrackProcessing
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
