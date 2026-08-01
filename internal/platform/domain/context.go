package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ActorContext struct {
	OperatorID    string
	DeviceID      string
	WarehouseID   string
	CorrelationID string
}

func NewActorContext(operatorID, deviceID, warehouseID, correlationID string) (ActorContext, error) {
	actor := ActorContext{operatorID, deviceID, warehouseID, correlationID}
	if strings.TrimSpace(actor.OperatorID) == "" || strings.TrimSpace(actor.DeviceID) == "" ||
		strings.TrimSpace(actor.WarehouseID) == "" || strings.TrimSpace(actor.CorrelationID) == "" {
		return ActorContext{}, fmt.Errorf("actor context fields must be non-empty")
	}
	return actor, nil
}

type CommandMetadata struct {
	CommandID      uuid.UUID
	IdempotencyKey string
	IssuedAt       time.Time
	Actor          ActorContext
}

func NewCommandMetadata(commandID uuid.UUID, idempotencyKey string, issuedAt time.Time, actor ActorContext) (CommandMetadata, error) {
	if commandID == uuid.Nil || strings.TrimSpace(idempotencyKey) == "" || issuedAt.IsZero() {
		return CommandMetadata{}, fmt.Errorf("command metadata is incomplete")
	}
	return CommandMetadata{commandID, idempotencyKey, issuedAt.UTC(), actor}, nil
}
