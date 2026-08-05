package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/company/pda-backend/internal/platform/event"
	"github.com/company/pda-backend/internal/platform/messaging"
	kafkago "github.com/segmentio/kafka-go"
)

var ErrBrokerUnavailable = errors.New("kafka broker is unavailable; no delivery was acknowledged")
var ErrSecurityUnsupported = errors.New("requested Kafka security protocol is not configured")

type Config struct {
	Brokers          []string
	GroupID          string
	TopicPrefix      string
	SecurityProtocol string
	TLSCAFile        string
	TLSCertFile      string
	TLSKeyFile       string
	TLSServerName    string
}

func (c Config) Validate() error {
	if len(NormalizeBrokers(c.Brokers)) == 0 {
		return fmt.Errorf("at least one Kafka broker is required")
	}
	if strings.TrimSpace(c.GroupID) == "" || strings.TrimSpace(c.TopicPrefix) == "" {
		return fmt.Errorf("Kafka group ID and topic prefix are required")
	}
	return nil
}

// Publisher is intentionally fail-closed until a verified Kafka client is selected.
// It never reports success or silently falls back to the mock publisher.
type Publisher struct {
	Config  Config
	writer  *kafkago.Writer
	Metrics *messaging.Metrics
}

func NewPublisher(cfg Config) (*Publisher, error) {
	if cfg.SecurityProtocol == "" {
		cfg.SecurityProtocol = "PLAINTEXT"
	}
	cfg.Brokers = NormalizeBrokers(cfg.Brokers)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	tlsConfig, err := loadTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Publisher{Config: cfg, Metrics: &messaging.Metrics{}, writer: &kafkago.Writer{
		Addr: kafkago.TCP(cfg.Brokers...), Topic: "", Balancer: &kafkago.Hash{},
		RequiredAcks: kafkago.RequireAll, MaxAttempts: 3, BatchTimeout: 20 * time.Millisecond,
		Transport: &kafkago.Transport{TLS: tlsConfig},
	}}, nil
}

func loadTLSConfig(cfg Config) (*tls.Config, error) {
	switch cfg.SecurityProtocol {
	case "PLAINTEXT":
		return nil, nil
	case "TLS":
		if strings.TrimSpace(cfg.TLSCAFile) == "" {
			return nil, fmt.Errorf("Kafka TLS requires a CA file")
		}
		caPEM, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("read Kafka TLS CA file: %w", err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("Kafka TLS CA file contains no certificates")
		}
		config := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: strings.TrimSpace(cfg.TLSServerName)}
		if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
			if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
				return nil, fmt.Errorf("Kafka TLS client certificate and key must be provided together")
			}
			certificate, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
			if err != nil {
				return nil, fmt.Errorf("load Kafka TLS client certificate: %w", err)
			}
			config.Certificates = []tls.Certificate{certificate}
		}
		return config, nil
	default:
		return nil, ErrSecurityUnsupported
	}
}

func (p *Publisher) Publish(ctx context.Context, envelope event.DomainEventEnvelope) error {
	if p == nil || p.writer == nil {
		return ErrBrokerUnavailable
	}
	if err := envelope.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	topic := ResolveTopic(p.Config.TopicPrefix, envelope.Topic)
	err = p.writer.WriteMessages(ctx, kafkago.Message{Topic: topic, Key: []byte(envelope.AggregateID), Value: data, Time: envelope.OccurredAt})
	if p.Metrics != nil {
		if err != nil {
			p.Metrics.Failed.Add(1)
		} else {
			p.Metrics.Published.Add(1)
		}
	}
	return err
}

func ResolveTopic(prefix, topic string) string {
	prefix = strings.Trim(prefix, ".")
	topic = strings.Trim(topic, ".")
	if prefix == "" || strings.HasPrefix(topic, prefix+".") {
		return topic
	}
	return prefix + "." + topic
}

func NormalizeBrokers(brokers []string) []string {
	normalized := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			normalized = append(normalized, broker)
		}
	}
	return normalized
}
