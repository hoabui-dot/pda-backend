package kafka

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWithPublishedAtAddsKafkaPublicationBoundary(t *testing.T) {
	when := time.Date(2026, 8, 20, 12, 0, 0, 123000000, time.UTC)
	got := withPublishedAt([]byte(`{"event_type":"PDA.TaskReceivedOnDevice.v1"}`), when)
	var envelope map[string]any
	if err := json.Unmarshal(got, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["published_at"] != when.Format(time.RFC3339Nano) {
		t.Fatalf("published_at = %v, want %s", envelope["published_at"], when.Format(time.RFC3339Nano))
	}
}

func TestWithPublishedAtPreservesProducerBoundary(t *testing.T) {
	raw := []byte(`{"event_type":"x","published_at":"2026-08-20T11:00:00Z"}`)
	got := withPublishedAt(raw, time.Now())
	if string(got) != string(raw) {
		t.Fatalf("existing published_at was changed: %s", got)
	}
}
