package domain

type PageQuery struct {
	Limit  int
	Cursor string
}

type TrackPage struct {
	Items      []Track
	NextCursor string
	Limit      int
}
