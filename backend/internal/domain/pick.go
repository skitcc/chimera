package domain

import "time"

type Pick struct {
	ID          string
	UserID      string
	Title       string
	GeneratedAt time.Time
}

func (p Pick) RequireOwner(userID string) error {
	if p.UserID != userID {
		return Forbidden("not allowed")
	}
	return nil
}

type PickID string

func (id PickID) Validate() error {
	if id == "" {
		return Invalid("pick id is required")
	}
	return nil
}

type PickTrack struct {
	Position int
	Note     string
	Track    Track
}

type PickDetail struct {
	Pick
	Tracks []PickTrack
}

type PickPatch struct {
	Title string
}

func (in PickPatch) Validate() error {
	if in.Title == "" {
		return Invalid("title is required")
	}
	return nil
}
