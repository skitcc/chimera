package domain

type Track struct {
	ID     string
	Title  string
	Artist string
}

type TrackWrite struct {
	Title  string
	Artist string
}
