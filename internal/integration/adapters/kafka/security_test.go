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

func TestTLSRequiresExplicitCA(t *testing.T) {
	_, err := NewPublisher(Config{Brokers: []string{"127.0.0.1:19092"}, GroupID: "security", TopicPrefix: "pda", SecurityProtocol: "TLS"})
	if err == nil {
		t.Fatal("TLS must require an explicit CA file")
	}
}
