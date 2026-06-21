package ovh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfig() *Config {
	return &Config{
		ApplicationKey:    "app-key",
		ApplicationSecret: "app-secret",
		ConsumerKey:       "consumer-key",
		Zone:              "example.com",
		Endpoint:          DefaultEndpoint,
		TTL:               DefaultTTL,
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"valid", func(*Config) {}, ""},
		{"missing app key", func(c *Config) { c.ApplicationKey = "" }, "APPLICATION_KEY is required"},
		{"missing app secret", func(c *Config) { c.ApplicationSecret = "" }, "APPLICATION_SECRET is required"},
		{"missing consumer key", func(c *Config) { c.ConsumerKey = "" }, "CONSUMER_KEY is required"},
		{"missing zone", func(c *Config) { c.Zone = "" }, "ZONE is required"},
		{"missing endpoint", func(c *Config) { c.Endpoint = "" }, "ENDPOINT is required"},
		{"unknown endpoint", func(c *Config) { c.Endpoint = "ovh-mars" }, "not a known OVH region"},
		{"negative ttl", func(c *Config) { c.TTL = -1 }, "TTL must be non-negative"},
		{"too small ttl", func(c *Config) { c.TTL = 30 }, "at least 60 seconds"},
		{"zero ttl allowed", func(c *Config) { c.TTL = 0 }, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestConfig_EndpointURL(t *testing.T) {
	cfg := validConfig()
	if got := cfg.EndpointURL(); got != "https://eu.api.ovh.com/1.0" {
		t.Errorf("expected ovh-eu URL, got %s", got)
	}

	cfg.Endpoint = "ovh-us"
	if got := cfg.EndpointURL(); got != "https://api.us.ovhcloud.com/1.0" {
		t.Errorf("expected ovh-us URL, got %s", got)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Setenv("DNSWEAVER_MYOVH_APPLICATION_KEY", "ak")
	t.Setenv("DNSWEAVER_MYOVH_APPLICATION_SECRET", "as")
	t.Setenv("DNSWEAVER_MYOVH_CONSUMER_KEY", "ck")
	t.Setenv("DNSWEAVER_MYOVH_ZONE", "example.com")
	t.Setenv("DNSWEAVER_MYOVH_TTL", "600")

	cfg, err := LoadConfig("myovh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ApplicationKey != "ak" || cfg.ApplicationSecret != "as" || cfg.ConsumerKey != "ck" {
		t.Errorf("credentials not loaded: %+v", cfg)
	}
	if cfg.Zone != "example.com" {
		t.Errorf("expected zone example.com, got %s", cfg.Zone)
	}
	if cfg.Endpoint != DefaultEndpoint {
		t.Errorf("expected default endpoint, got %s", cfg.Endpoint)
	}
	if cfg.TTL != 600 {
		t.Errorf("expected TTL 600, got %d", cfg.TTL)
	}
}

func TestLoadConfig_HyphenatedInstanceName(t *testing.T) {
	t.Setenv("DNSWEAVER_PUBLIC_DNS_APPLICATION_KEY", "ak")
	t.Setenv("DNSWEAVER_PUBLIC_DNS_APPLICATION_SECRET", "as")
	t.Setenv("DNSWEAVER_PUBLIC_DNS_CONSUMER_KEY", "ck")
	t.Setenv("DNSWEAVER_PUBLIC_DNS_ZONE", "example.com")

	cfg, err := LoadConfig("public-dns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ApplicationKey != "ak" {
		t.Errorf("expected hyphenated instance name to normalize, got %+v", cfg)
	}
}

func TestLoadConfig_FileSecret(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret")
	if err := os.WriteFile(secretPath, []byte("  file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DNSWEAVER_MYOVH_APPLICATION_KEY", "ak")
	t.Setenv("DNSWEAVER_MYOVH_APPLICATION_SECRET_FILE", secretPath)
	t.Setenv("DNSWEAVER_MYOVH_CONSUMER_KEY", "ck")
	t.Setenv("DNSWEAVER_MYOVH_ZONE", "example.com")

	cfg, err := LoadConfig("myovh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ApplicationSecret != "file-secret" {
		t.Errorf("expected secret read and trimmed from file, got %q", cfg.ApplicationSecret)
	}
}

func TestLoadConfigFromMap(t *testing.T) {
	cfg, err := LoadConfigFromMap("myovh", map[string]string{
		"APPLICATION_KEY":    "ak",
		"APPLICATION_SECRET": "as",
		"CONSUMER_KEY":       "ck",
		"ZONE":               "example.com",
		"ENDPOINT":           "ovh-ca",
		"TTL":                "120",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Endpoint != "ovh-ca" {
		t.Errorf("expected endpoint ovh-ca, got %s", cfg.Endpoint)
	}
	if cfg.TTL != 120 {
		t.Errorf("expected TTL 120, got %d", cfg.TTL)
	}
}

func TestLoadConfigFromMap_Invalid(t *testing.T) {
	_, err := LoadConfigFromMap("myovh", map[string]string{
		"APPLICATION_KEY": "ak",
	})
	if err == nil {
		t.Fatal("expected validation error for incomplete config")
	}
}

func TestLoadConfigFromMap_InvalidTTL(t *testing.T) {
	_, err := LoadConfigFromMap("myovh", map[string]string{
		"APPLICATION_KEY":    "ak",
		"APPLICATION_SECRET": "as",
		"CONSUMER_KEY":       "ck",
		"ZONE":               "example.com",
		"TTL":                "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for invalid TTL")
	}
}
