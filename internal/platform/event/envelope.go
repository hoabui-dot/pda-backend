package event

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DomainEventEnvelope struct {
	EventID          uuid.UUID       `json:"eventId"`
	EventType        string          `json:"eventType"`
	EventVersion     int             `json:"eventVersion"`
	AggregateType    string          `json:"aggregateType"`
	AggregateID      string          `json:"aggregateId"`
	AggregateVersion int64           `json:"aggregateVersion"`
	OccurredAt       time.Time       `json:"occurredAt"`
	CorrelationID    string          `json:"correlationId"`
	CausationID      uuid.UUID       `json:"causationId"`
	WarehouseID      string          `json:"warehouseId"`
	OperatorID       string          `json:"operatorId"`
	DeviceID         string          `json:"deviceId"`
	Topic            string          `json:"topic"`
	Payload          json.RawMessage `json:"payload"`
}

func (e DomainEventEnvelope) Validate() error {
	if e.EventID == uuid.Nil || e.CausationID == uuid.Nil || e.EventVersion < 1 || e.AggregateVersion < 1 || e.OccurredAt.IsZero() {
		return fmt.Errorf("event envelope identity, versions, and timestamp are required")
	}
	for name, value := range map[string]string{"eventType": e.EventType, "aggregateType": e.AggregateType, "aggregateId": e.AggregateID, "correlationId": e.CorrelationID, "warehouseId": e.WarehouseID, "operatorId": e.OperatorID, "deviceId": e.DeviceID, "topic": e.Topic} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("event envelope %s is required", name)
		}
	}
	if len(e.Payload) == 0 || !json.Valid(e.Payload) {
		return fmt.Errorf("event payload must be valid JSON")
	}
	return nil
}

type DomainEventPublisher interface {
	Publish(context.Context, DomainEventEnvelope) error
}
