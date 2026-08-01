package kafka

import (
	"context"
	"time"

	"github.com/company/pda-backend/internal/platform/event"
	"github.com/company/pda-backend/internal/platform/messaging"
	"github.com/google/uuid"
)

type OutboxDeliveryStore interface {
	messaging.OutboxRepository
	MarkFailed(context.Context, uuid.UUID, error, time.Time) error
}

type OutboxWorker struct {
	Store     OutboxDeliveryStore
	Publisher event.DomainEventPublisher
	BatchSize int
	Metrics   *messaging.Metrics
}

func (w OutboxWorker) RunOnce(ctx context.Context) (int, error) {
	if w.BatchSize < 1 {
		w.BatchSize = 100
	}
	records, err := w.Store.ClaimPending(ctx, w.BatchSize)
	if err != nil {
		return 0, err
	}
	for _, record := range records {
		if err := w.Publisher.Publish(ctx, record.Envelope); err != nil {
			next := time.Now().UTC().Add(5 * time.Second)
			if markErr := w.Store.MarkFailed(ctx, record.ID, err, next); markErr != nil {
				return 0, markErr
			}
			if w.Metrics != nil {
				w.Metrics.Failed.Add(1)
			}
			continue
		}
		if err := w.Store.MarkPublished(ctx, record.ID, time.Now().UTC()); err != nil {
			return 0, err
		}
		if w.Metrics != nil {
			w.Metrics.Published.Add(1)
		}
	}
	return len(records), nil
}
