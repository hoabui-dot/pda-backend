package config

import "testing"

func TestProductionRejectsEveryMockMode(t *testing.T) {
	tests := []Modes{
		{Messaging: "mock", UpstreamWMS: "http", Auth: "oidc"},
		{Messaging: "kafka", UpstreamWMS: "mock", Auth: "oidc"},
		{Messaging: "kafka", UpstreamWMS: "http", Auth: "mock"},
	}
	for _, modes := range tests {
		if err := (Config{Environment: "production", Modes: modes}).Validate(); err == nil {
			t.Fatalf("production accepted mock modes: %+v", modes)
		}
	}
}

func TestProductionAcceptsRealModes(t *testing.T) {
	config := Config{Environment: "production", UpstreamWMSBaseURL: "https://wms.example.test", UpstreamWMSToken: "token", Modes: Modes{Messaging: "kafka", UpstreamWMS: "http", Auth: "oidc"}}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPWMSRequiresConnectionSettings(t *testing.T) {
	config := Config{Environment: "local", Modes: Modes{Messaging: "mock", UpstreamWMS: "http", Auth: "mock"}}
	if err := config.Validate(); err == nil {
		t.Fatal("expected HTTP WMS connection settings to be required")
	}
	config.UpstreamWMSBaseURL = "https://wms.example.test"
	if err := config.Validate(); err == nil {
		t.Fatal("expected HTTP WMS bearer token to be required")
	}
	config.UpstreamWMSToken = "token"
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCacheTTLValidation(t *testing.T) {
	cfg := Config{CacheTTL: "0s", Modes: Modes{Messaging: "mock", UpstreamWMS: "mock", Auth: "mock"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected zero TTL rejection")
	}
	cfg.CacheTTL = "30s"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidModeIsRejected(t *testing.T) {
	config := Config{Environment: "local", Modes: Modes{Messaging: "silent-fallback", UpstreamWMS: "mock", Auth: "mock"}}
	if err := config.Validate(); err == nil {
		t.Fatal("expected invalid mode rejection")
	}
}
