package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
)

type PublishedMarker interface {
	MarkPublished(context.Context, uuid.UUID, time.Time) error
}

type MarkingPublisher struct {
	Publisher event.DomainEventPublisher
	Marker    PublishedMarker
	Now       func() time.Time
}

func (p MarkingPublisher) Publish(ctx context.Context, envelope event.DomainEventEnvelope) error {
	if p.Publisher == nil || p.Marker == nil {
		return fmt.Errorf("Kafka marking publisher requires publisher and marker")
	}
	if err := p.Publisher.Publish(ctx, envelope); err != nil {
		return err
	}
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	return p.Marker.MarkPublished(ctx, envelope.EventID, now().UTC())
}
