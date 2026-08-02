package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Modes struct {
	Messaging   string `json:"messaging"`
	UpstreamWMS string `json:"upstreamWms"`
	Auth        string `json:"auth"`
}

type Config struct {
	Environment           string
	DatabaseURL           string
	RedisURL              string
	CacheTTL              string
	KafkaBrokers          string
	KafkaGroupID          string
	KafkaTopicPrefix      string
	KafkaSecurityProtocol string
	UpstreamWMSBaseURL    string
	UpstreamWMSToken      string
	Modes                 Modes
}

func Load() (Config, error) {
	config := Config{
		Environment:           valueOrDefault("PDA_ENVIRONMENT", "local"),
		DatabaseURL:           valueOrDefault("PDA_DATABASE_URL", "postgres://pda:local-only-pda@localhost:15432/pda?sslmode=disable"),
		RedisURL:              valueOrDefault("PDA_REDIS_URL", "redis://localhost:16379/0"),
		CacheTTL:              valueOrDefault("PDA_CACHE_TTL", "30s"),
		KafkaBrokers:          valueOrDefault("PDA_KAFKA_BROKERS", ""),
		KafkaGroupID:          valueOrDefault("PDA_KAFKA_GROUP_ID", "pda-backend"),
		KafkaTopicPrefix:      valueOrDefault("PDA_KAFKA_TOPIC_PREFIX", "pda"),
		KafkaSecurityProtocol: valueOrDefault("PDA_KAFKA_SECURITY_PROTOCOL", "PLAINTEXT"),
		UpstreamWMSBaseURL:    valueOrDefault("PDA_UPSTREAM_WMS_BASE_URL", ""),
		UpstreamWMSToken:      valueOrDefault("PDA_UPSTREAM_WMS_TOKEN", ""),
		Modes: Modes{
			Messaging:   valueOrDefault("PDA_MESSAGING_MODE", "mock"),
			UpstreamWMS: valueOrDefault("PDA_UPSTREAM_WMS_MODE", "mock"),
			Auth:        valueOrDefault("PDA_AUTH_MODE", "mock"),
		},
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) Validate() error {
	if c.CacheTTL != "" {
		ttl, err := time.ParseDuration(c.CacheTTL)
		if err != nil || ttl <= 0 {
			return fmt.Errorf("invalid cache TTL %q", c.CacheTTL)
		}
	}
	if !oneOf(c.Modes.Messaging, "mock", "kafka") {
		return fmt.Errorf("invalid messaging mode %q", c.Modes.Messaging)
	}
	if !oneOf(c.Modes.UpstreamWMS, "mock", "http") {
		return fmt.Errorf("invalid upstream WMS mode %q", c.Modes.UpstreamWMS)
	}
	if !oneOf(c.Modes.Auth, "mock", "oidc") {
		return fmt.Errorf("invalid auth mode %q", c.Modes.Auth)
	}
	if c.Modes.UpstreamWMS == "http" {
		parsed, err := url.Parse(strings.TrimSpace(c.UpstreamWMSBaseURL))
		if err != nil || parsed.Host == "" || !oneOf(parsed.Scheme, "http", "https") {
			return fmt.Errorf("upstream WMS HTTP mode requires a valid HTTP(S) base URL")
		}
		if strings.TrimSpace(c.UpstreamWMSToken) == "" {
			return fmt.Errorf("upstream WMS HTTP mode requires a bearer token")
		}
	}
	if c.Environment == "production" && (c.Modes.Messaging == "mock" || c.Modes.UpstreamWMS == "mock" || c.Modes.Auth == "mock") {
		return fmt.Errorf("production environment rejects mock modes")
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
