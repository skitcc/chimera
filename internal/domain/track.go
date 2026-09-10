package domain

type Track struct {
	ID     string
	Title  string
	Artist string
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

func (w TrackWrite) Track(id string) Track {
	return Track{ID: id, Title: w.Title, Artist: w.Artist}
}
