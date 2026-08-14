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
	Environment             string
	DatabaseURL             string
	RedisURL                string
	CacheTTL                string
	KafkaBrokers            string
	KafkaGroupID            string
	KafkaTopicPrefix        string
	KafkaSecurityProtocol   string
	KafkaTLSCAFile          string
	KafkaTLSCertFile        string
	KafkaTLSKeyFile         string
	KafkaTLSServerName      string
	TokenSecret             string
	TokenIssuer             string
	TokenAudience           string
	AccessTokenTTL          string
	RefreshTokenTTL         string
	TokenSigningMode        string
	TokenPrivateKeyFile     string
	TokenPublicKeyFiles     string
	TokenKeyID              string
	UpstreamWMSBaseURL      string
	UpstreamWMSToken        string
	UpstreamWMSServiceToken string
	// WMSTaskTopics are the absolute cross-system topics carrying warehouse task
	// dispatch from WMS. They are never prefixed with the pda topic prefix
	// because they are owned by another system.
	WMSTaskTopics     string
	WMSTaskGroupID    string
	WMSInboundTopic   string
	WMSInboundGroupID string
	Modes             Modes
}

func Load() (Config, error) {
	config := Config{
		Environment:             valueOrDefault("PDA_ENVIRONMENT", "local"),
		DatabaseURL:             valueOrDefault("PDA_DATABASE_URL", "postgres://pda:local-only-pda@localhost:15432/pda?sslmode=disable"),
		RedisURL:                valueOrDefault("PDA_REDIS_URL", "redis://localhost:16379/0"),
		CacheTTL:                valueOrDefault("PDA_CACHE_TTL", "30s"),
		KafkaBrokers:            valueOrDefault("PDA_KAFKA_BROKERS", ""),
		KafkaGroupID:            valueOrDefault("PDA_KAFKA_GROUP_ID", "pda-backend"),
		KafkaTopicPrefix:        valueOrDefault("PDA_KAFKA_TOPIC_PREFIX", "pda"),
		KafkaSecurityProtocol:   valueOrDefault("PDA_KAFKA_SECURITY_PROTOCOL", "PLAINTEXT"),
		KafkaTLSCAFile:          valueOrDefault("PDA_KAFKA_TLS_CA_FILE", ""),
		KafkaTLSCertFile:        valueOrDefault("PDA_KAFKA_TLS_CERT_FILE", ""),
		KafkaTLSKeyFile:         valueOrDefault("PDA_KAFKA_TLS_KEY_FILE", ""),
		KafkaTLSServerName:      valueOrDefault("PDA_KAFKA_TLS_SERVER_NAME", ""),
		TokenSecret:             valueOrDefault("PDA_TOKEN_SECRET", ""),
		TokenIssuer:             valueOrDefault("PDA_TOKEN_ISSUER", "pda-backend"),
		TokenAudience:           valueOrDefault("PDA_TOKEN_AUDIENCE", "pda-app"),
		AccessTokenTTL:          valueOrDefault("PDA_ACCESS_TOKEN_TTL", "15m"),
		RefreshTokenTTL:         valueOrDefault("PDA_REFRESH_TOKEN_TTL", "720h"),
		TokenSigningMode:        valueOrDefault("PDA_TOKEN_SIGNING_MODE", "HS256"),
		TokenPrivateKeyFile:     valueOrDefault("PDA_TOKEN_PRIVATE_KEY_FILE", ""),
		TokenPublicKeyFiles:     valueOrDefault("PDA_TOKEN_PUBLIC_KEY_FILES", ""),
		TokenKeyID:              valueOrDefault("PDA_TOKEN_KEY_ID", "local"),
		UpstreamWMSBaseURL:      valueOrDefault("PDA_UPSTREAM_WMS_BASE_URL", ""),
		UpstreamWMSToken:        valueOrDefault("PDA_UPSTREAM_WMS_TOKEN", ""),
		UpstreamWMSServiceToken: valueOrDefault("PDA_UPSTREAM_WMS_SERVICE_TOKEN", ""),
		WMSTaskTopics:           valueOrDefault("PDA_WMS_TASK_TOPICS", "WMS.PDA.WarehouseTaskCreated.v1,WMS.PDA.WarehouseTaskUpdated.v1,WMS.PDA.WarehouseTaskCancelled.v1"),
		WMSTaskGroupID:          valueOrDefault("PDA_WMS_TASK_GROUP_ID", "pda-backend-wms-tasks-v1"),
		WMSInboundTopic:         valueOrDefault("PDA_WMS_INBOUND_TOPIC", "WMS.Inbound.ReceiptConfirmed.v1"),
		WMSInboundGroupID:       valueOrDefault("PDA_WMS_INBOUND_GROUP_ID", "pda-backend-wms-inbound-v1"),
		Modes: Modes{
			Messaging:   valueOrDefault("PDA_MESSAGING_MODE", "mock"),
			UpstreamWMS: valueOrDefault("PDA_UPSTREAM_WMS_MODE", "mock"),
			Auth:        valueOrDefault("PDA_AUTH_MODE", "internal"),
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
	if !oneOf(c.Modes.Auth, "mock", "oidc", "internal") {
		return fmt.Errorf("invalid auth mode %q", c.Modes.Auth)
	}
	if c.Modes.Auth == "internal" {
		if len(c.TokenSecret) < 32 || strings.TrimSpace(c.TokenIssuer) == "" || strings.TrimSpace(c.TokenAudience) == "" {
			return fmt.Errorf("internal auth requires token secret, issuer, and audience")
		}
		if ttl, err := time.ParseDuration(c.AccessTokenTTL); err != nil || ttl <= 0 {
			return fmt.Errorf("invalid access token TTL")
		}
		if ttl, err := time.ParseDuration(c.RefreshTokenTTL); err != nil || ttl != 30*24*time.Hour {
			return fmt.Errorf("refresh token TTL must be exactly 720h (30 days)")
		}
		if !oneOf(c.TokenSigningMode, "HS256", "RS256") {
			return fmt.Errorf("invalid token signing mode %q", c.TokenSigningMode)
		}
		if c.TokenSigningMode == "RS256" && (strings.TrimSpace(c.TokenPrivateKeyFile) == "" || strings.TrimSpace(c.TokenPublicKeyFiles) == "" || strings.TrimSpace(c.TokenKeyID) == "") {
			return fmt.Errorf("RS256 requires private key, public key set, and key ID")
		}
	}
	if c.Modes.UpstreamWMS == "http" {
		parsed, err := url.Parse(strings.TrimSpace(c.UpstreamWMSBaseURL))
		if err != nil || parsed.Host == "" || !oneOf(parsed.Scheme, "http", "https") {
			return fmt.Errorf("upstream WMS HTTP mode requires a valid HTTP(S) base URL")
		}
		if strings.TrimSpace(c.UpstreamWMSToken) == "" {
			return fmt.Errorf("upstream WMS HTTP mode requires a bearer token")
		}
		if strings.TrimSpace(c.UpstreamWMSServiceToken) == "" {
			return fmt.Errorf("upstream WMS HTTP mode requires a service token")
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
