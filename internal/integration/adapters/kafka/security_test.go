package kafka

import (
	"errors"
	"testing"
)

func TestUnsupportedSecurityFailsClosed(t *testing.T) {
	_, err := NewPublisher(Config{Brokers: []string{"127.0.0.1:19092"}, GroupID: "security", TopicPrefix: "pda", SecurityProtocol: "SASL_SSL"})
	if !errors.Is(err, ErrSecurityUnsupported) {
		t.Fatalf("expected explicit security failure, got %v", err)
	}
}
