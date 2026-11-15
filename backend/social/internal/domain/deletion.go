package domain

// UserDeletion is UserDeleted from Identity. EventID makes the cleanup
// idempotent when Kafka redelivers the event.
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

// TrackDeletion is TrackDeleted from Catalog.
type TrackDeletion struct {
	EventID string
	TrackID string
}

func (d TrackDeletion) Validate() error {
	if d.EventID == "" {
		return Invalid("event id is required")
	}
	if !validUUID(d.EventID) {
		return Invalid("invalid event id")
	}
	return TrackID(d.TrackID).Validate()
}
