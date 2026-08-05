package kafka

// Integration envelope translation.
//
// pda-backend serialises its internal DomainEventEnvelope in camelCase
// (eventId, eventType, aggregateId). mes-system and ricoh-wms both serialise
// their envelope in snake_case (event_id, event_type, aggregate_id), which is
// also the shape prompt section 9 specifies.
//
// Rewriting either internal model would be a breaking change for consumers that
// already work, so the canonical cross-system envelope is snake_case and this
// file is the translation boundary. Nothing outside the Kafka adapters needs to
// know that two representations exist.

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// IntegrationEnvelope is the canonical cross-system envelope. Field names match
// the envelope already produced by mes-system and ricoh-wms so existing
// consumers are unaffected.
type IntegrationEnvelope struct {
	EventID          string          `json:"event_id"`
	EventType        string          `json:"event_type"`
	EventVersion     int             `json:"event_version,omitempty"`
	OccurredAt       string          `json:"occurred_at"`
	PublishedAt      string          `json:"published_at,omitempty"`
	SourceService    string          `json:"source_service"`
	Producer         string          `json:"producer,omitempty"`
	TraceID          string          `json:"trace_id,omitempty"`
	AggregateType    string          `json:"aggregate_type,omitempty"`
	AggregateID      string          `json:"aggregate_id,omitempty"`
	AggregateVersion *int64          `json:"aggregate_version,omitempty"`
	CorrelationID    string          `json:"correlation_id,omitempty"`
	CausationID      string          `json:"causation_id,omitempty"`
	SiteID           string          `json:"site_id,omitempty"`
	SchemaVersion    int             `json:"schema_version,omitempty"`
	Metadata         map[string]any  `json:"metadata,omitempty"`
	Payload          json.RawMessage `json:"payload"`
}

// ErrUnsupportedEventVersion marks a delivery whose contract version this build
// does not implement. Section 9 requires such a message to go to the DLQ rather
// than be interpreted optimistically.
var ErrUnsupportedEventVersion = fmt.Errorf("unsupported event version")

// Validate enforces the envelope invariants from prompt section 9 that a
// consumer can check without knowing the payload schema.
func (e IntegrationEnvelope) Validate() error {
	for name, value := range map[string]string{
		"event_id":   e.EventID,
		"event_type": e.EventType,
		"payload":    string(e.Payload),
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("integration envelope %s is required", name)
		}
	}
	if len(e.Payload) == 0 || !json.Valid(e.Payload) {
		return fmt.Errorf("integration envelope payload must be valid JSON")
	}
	if e.OccurredAt != "" {
		if _, err := parseTimestamp(e.OccurredAt); err != nil {
			return fmt.Errorf("integration envelope occurred_at must be UTC ISO-8601: %w", err)
		}
	}
	return nil
}

// ContractVersion returns the trailing .vN of the event type. It is the version
// a consumer gates on, because the event type is the only part of the message
// guaranteed to be present before the payload is understood.
func (e IntegrationEnvelope) ContractVersion() int {
	index := strings.LastIndex(e.EventType, ".v")
	if index < 0 {
		return 0
	}
	version := 0
	if _, err := fmt.Sscanf(e.EventType[index+2:], "%d", &version); err != nil {
		return 0
	}
	return version
}

// PartitionKey is the value the producer declared as its ordering key. Falling
// back to aggregate_id preserves per-aggregate ordering when a producer omits
// the metadata block.
func (e IntegrationEnvelope) PartitionKey() string {
	if e.Metadata != nil {
		if key, ok := e.Metadata["partition_key"].(string); ok && key != "" {
			return key
		}
	}
	return e.AggregateID
}

func parseTimestamp(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", value)
}
