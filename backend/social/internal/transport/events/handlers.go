// Package events maps Kafka envelopes to Social use cases.
package events

import (
	"context"

	eventsv1 "chimera/contracts/gen/events/v1"
	"chimera/platform/kafka"
	"chimera/social/internal/domain"
)

type CleanupService interface {
	RemoveUser(ctx context.Context, d domain.UserDeletion) error
	RemoveTrack(ctx context.Context, d domain.TrackDeletion) error
}

type Handlers struct {
	cleanup CleanupService
}

func NewHandlers(cleanup CleanupService) *Handlers {
	return &Handlers{cleanup: cleanup}
}

// Handle consumes identity.user.v1 and catalog.track.v1. Unknown event types
// are skipped so producers can add new ones without breaking this consumer.
func (h *Handlers) Handle(ctx context.Context, event *eventsv1.Envelope) error {
	switch p := event.GetPayload().(type) {
	case *eventsv1.Envelope_UserDeleted:
		return classify(h.cleanup.RemoveUser(ctx, domain.UserDeletion{
			EventID: event.GetEventId(),
			UserID:  p.UserDeleted.GetUserId(),
		}))
	case *eventsv1.Envelope_TrackDeleted:
		return classify(h.cleanup.RemoveTrack(ctx, domain.TrackDeletion{
			EventID: event.GetEventId(),
			TrackID: p.TrackDeleted.GetTrackId(),
		}))
	default:
		return nil
	}
}

// classify sends malformed events to the DLQ; everything else is retried.
func classify(err error) error {
	if domain.Is(err, domain.CodeInvalid) {
		return kafka.Permanent(err)
	}
	return err
}
