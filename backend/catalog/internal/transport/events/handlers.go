// Package events maps Kafka envelopes to Catalog use cases.
package events

import (
	"context"

	"chimera/catalog/internal/domain"
	eventsv1 "chimera/contracts/gen/events/v1"
	"chimera/platform/kafka"
)

type CleanupService interface {
	RemoveUserTracks(ctx context.Context, d domain.UserDeletion) error
	RemoveObject(ctx context.Context, key domain.ObjectKey) error
}

type Handlers struct {
	cleanup CleanupService
}

func NewHandlers(cleanup CleanupService) *Handlers {
	return &Handlers{cleanup: cleanup}
}

// UserEvents consumes identity.user.v1. Unknown event types are skipped so
// Identity can add new ones without breaking this consumer.
func (h *Handlers) UserEvents(ctx context.Context, event *eventsv1.Envelope) error {
	deleted := event.GetUserDeleted()
	if deleted == nil {
		return nil
	}
	return classify(h.cleanup.RemoveUserTracks(ctx, domain.UserDeletion{
		EventID: event.GetEventId(),
		UserID:  deleted.GetUserId(),
	}))
}

// TrackObjects consumes catalog.track.v1 and deletes the audio of removed tracks.
func (h *Handlers) TrackObjects(ctx context.Context, event *eventsv1.Envelope) error {
	deleted := event.GetTrackDeleted()
	if deleted == nil {
		return nil
	}
	return classify(h.cleanup.RemoveObject(ctx, domain.ObjectKey(deleted.GetObjectKey())))
}

// classify sends malformed events to the DLQ; everything else is retried.
func classify(err error) error {
	if domain.Is(err, domain.CodeInvalid) {
		return kafka.Permanent(err)
	}
	return err
}
