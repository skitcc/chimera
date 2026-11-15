// Package events names the Kafka topics and event types shared by producers and consumers.
package events

const (
	TopicUser  = "identity.user.v1"
	TopicTrack = "catalog.track.v1"

	// DLQSuffix is appended to a topic name for records a consumer could not process.
	DLQSuffix = ".dlq"
)

const (
	TypeUserDeleted  = "chimera.identity.user.deleted.v1"
	TypeTrackDeleted = "chimera.catalog.track.deleted.v1"
)
