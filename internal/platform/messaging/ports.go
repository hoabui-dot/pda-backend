package messaging

import (
	"context"
	"time"

	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
)

type OutboxRecord struct {
	ID          uuid.UUID
	Envelope    event.DomainEventEnvelope
	CreatedAt   time.Time
	PublishedAt *time.Time
}

type OutboxRepository interface {
	Append(context.Context, OutboxRecord) error
	ClaimPending(context.Context, int) ([]OutboxRecord, error)
	MarkPublished(context.Context, uuid.UUID, time.Time) error
}

type InboxRepository interface {
	AlreadyProcessed(context.Context, uuid.UUID) (bool, error)
	MarkProcessed(context.Context, uuid.UUID, time.Time) error
}
