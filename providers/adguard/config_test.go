package adguard

import (
	"os"
	"testing"
)

func TestEnvPrefix(t *testing.T) {
	tests := []struct {
		name     string
		instance string
		want     string
	}{
		{"simple", "adguard", "DNSWEAVER_ADGUARD_"},
		{"with-hyphen", "adguard-dns", "DNSWEAVER_ADGUARD_DNS_"},
		{"multiple-hyphens", "my-adguard-home", "DNSWEAVER_MY_ADGUARD_HOME_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envPrefix(tt.instance)
			if got != tt.want {
				t.Errorf("envPrefix(%q) = %q, want %q", tt.instance, got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				URL:      "http://adguard.local:3000",
				Username: "admin",
				Password: "secret",
				TTL:      300,
			},
			wantErr: false,
		},
		{
			name: "valid config with https",
			config: Config{
				URL:      "https://adguard.example.com",
				Username: "admin",
				Password: "secret",
				TTL:      300,
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			config: Config{
				Username: "admin",
				Password: "secret",
			},
			wantErr: true,
		},
		{
			name: "missing username",
			config: Config{
				URL:      "http://adguard.local:3000",
				Password: "secret",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			config: Config{
				URL:      "http://adguard.local:3000",
				Username: "admin",
			},
			wantErr: true,
		},
		{
			name: "invalid URL scheme",
			config: Config{
				URL:      "ftp://adguard.local:3000",
				Username: "admin",
				Password: "secret",
			},
			wantErr: true,
		},
		{
			name: "URL with embedded credentials",
			config: Config{
				URL:      "http://admin:pass@adguard.local:3000",
				Username: "admin",
				Password: "secret",
			},
			wantErr: true,
		},
		{
			name: "negative TTL",
			config: Config{
				URL:      "http://adguard.local:3000",
				Username: "admin",
				Password: "secret",
				TTL:      -1,
			},
			wantErr: true,
		},
		{
			name: "zero TTL is valid",
			config: Config{
				URL:      "http://adguard.local:3000",
				Username: "admin",
				Password: "secret",
				TTL:      0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	prefix := "DNSWEAVER_TESTADGUARD_"

	t.Run("valid from env", func(t *testing.T) {
		t.Setenv(prefix+"URL", "http://adguard.local:3000")
		t.Setenv(prefix+"USERNAME", "admin")
		t.Setenv(prefix+"PASSWORD", "secret")
		t.Setenv(prefix+"ZONE", "home.local")
		t.Setenv(prefix+"TTL", "600")

		cfg, err := LoadConfig("testadguard")
		if err != nil {
			t.Fatalf("LoadConfig() unexpected error: %v", err)
		}

		if cfg.URL != "http://adguard.local:3000" {
			t.Errorf("URL = %q, want %q", cfg.URL, "http://adguard.local:3000")
		}
		if cfg.Username != "admin" {
			t.Errorf("Username = %q, want %q", cfg.Username, "admin")
		}
		if cfg.Password != "secret" {
			t.Errorf("Password = %q, want %q", cfg.Password, "secret")
		}
		if cfg.Zone != "home.local" {
			t.Errorf("Zone = %q, want %q", cfg.Zone, "home.local")
		}
		if cfg.TTL != 600 {
			t.Errorf("TTL = %d, want %d", cfg.TTL, 600)
		}
	})

	t.Run("default TTL", func(t *testing.T) {
		t.Setenv(prefix+"URL", "http://adguard.local:3000")
		t.Setenv(prefix+"USERNAME", "admin")
		t.Setenv(prefix+"PASSWORD", "secret")

		cfg, err := LoadConfig("testadguard")
		if err != nil {
			t.Fatalf("LoadConfig() unexpected error: %v", err)
		}

		if cfg.TTL != DefaultTTL {
			t.Errorf("TTL = %d, want default %d", cfg.TTL, DefaultTTL)
		}
	})

	t.Run("invalid TTL", func(t *testing.T) {
		t.Setenv(prefix+"URL", "http://adguard.local:3000")
		t.Setenv(prefix+"USERNAME", "admin")
		t.Setenv(prefix+"PASSWORD", "secret")
		t.Setenv(prefix+"TTL", "not-a-number")

		_, err := LoadConfig("testadguard")
		if err == nil {
			t.Fatal("LoadConfig() expected error for invalid TTL")
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		// Only set URL, missing username and password
		t.Setenv(prefix+"URL", "http://adguard.local:3000")

		_, err := LoadConfig("testadguard")
		if err == nil {
			t.Fatal("LoadConfig() expected error for missing fields")
		}
	})

	t.Run("password from file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "adguard-password-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString("  file-secret  \n"); err != nil {
			t.Fatal(err)
		}
		tmpFile.Close()

		t.Setenv(prefix+"URL", "http://adguard.local:3000")
		t.Setenv(prefix+"USERNAME", "admin")
		t.Setenv(prefix+"PASSWORD_FILE", tmpFile.Name())

		cfg, err := LoadConfig("testadguard")
		if err != nil {
			t.Fatalf("LoadConfig() unexpected error: %v", err)
		}

		if cfg.Password != "file-secret" {
			t.Errorf("Password = %q, want %q (from file, trimmed)", cfg.Password, "file-secret")
		}
	})
}

func TestLoadConfigFromMap(t *testing.T) {
	tests := []struct {
		name    string
		m       map[string]string
		wantErr bool
	}{
		{
			name: "valid map",
			m: map[string]string{
				"url":      "http://adguard.local:3000",
				"username": "admin",
				"password": "secret",
				"zone":     "home.local",
				"ttl":      "600",
			},
			wantErr: false,
		},
		{
			name: "uppercase keys",
			m: map[string]string{
				"URL":      "http://adguard.local:3000",
				"USERNAME": "admin",
				"PASSWORD": "secret",
			},
			wantErr: false,
		},
		{
			name: "missing required fields",
			m: map[string]string{
				"url": "http://adguard.local:3000",
			},
			wantErr: true,
		},
		{
			name: "invalid TTL",
			m: map[string]string{
				"url":      "http://adguard.local:3000",
				"username": "admin",
				"password": "secret",
				"ttl":      "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfigFromMap("test", tt.m)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromMap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
