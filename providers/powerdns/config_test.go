package powerdns

import "testing"

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{URL: "http://ns1:8081", APIKey: "k", Zone: "example.com", TTL: 300}, false},
		{"missing URL", Config{APIKey: "k", Zone: "example.com"}, true},
		{"missing API key", Config{URL: "http://ns1:8081", Zone: "example.com"}, true},
		{"missing zone", Config{URL: "http://ns1:8081", APIKey: "k"}, true},
		{"negative TTL", Config{URL: "http://ns1:8081", APIKey: "k", Zone: "example.com", TTL: -1}, true},
		{"zero TTL ok", Config{URL: "http://ns1:8081", APIKey: "k", Zone: "example.com", TTL: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfigFromMap_Defaults(t *testing.T) {
	cfg, err := LoadConfigFromMap("my-pdns", map[string]string{
		"URL":     "http://ns1:8081/",
		"API_KEY": "secret",
		"ZONE":    "example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServerID != DefaultServerID {
		t.Errorf("ServerID = %q, want %q", cfg.ServerID, DefaultServerID)
	}
	if cfg.TTL != DefaultTTL {
		t.Errorf("TTL = %d, want %d", cfg.TTL, DefaultTTL)
	}
	if cfg.URL != "http://ns1:8081" {
		t.Errorf("URL = %q, want trailing slash trimmed", cfg.URL)
	}
}

func TestLoadConfigFromMap_Overrides(t *testing.T) {
	cfg, err := LoadConfigFromMap("my-pdns", map[string]string{
		"URL":       "http://ns1:8081",
		"API_KEY":   "secret",
		"ZONE":      "example.com",
		"SERVER_ID": "ns1",
		"TTL":       "600",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServerID != "ns1" {
		t.Errorf("ServerID = %q, want ns1", cfg.ServerID)
	}
	if cfg.TTL != 600 {
		t.Errorf("TTL = %d, want 600", cfg.TTL)
	}
}

func TestLoadConfigFromMap_InvalidTTL(t *testing.T) {
	_, err := LoadConfigFromMap("my-pdns", map[string]string{
		"URL": "http://ns1:8081", "API_KEY": "secret", "ZONE": "example.com", "TTL": "abc",
	})
	if err == nil {
		t.Error("expected error for invalid TTL, got nil")
	}
}

func TestLoadConfigFromMap_MissingRequired(t *testing.T) {
	_, err := LoadConfigFromMap("my-pdns", map[string]string{"URL": "http://ns1:8081"})
	if err == nil {
		t.Error("expected error for missing API_KEY/ZONE, got nil")
	}
}
