// Package powerdns implements the dnsweaver provider interface for the
// PowerDNS Authoritative Server HTTP API (/api/v1, X-API-Key auth).
package powerdns

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DefaultTTL is the fallback TTL for created records when neither the record
// nor the instance specifies one.
const DefaultTTL = 300

// DefaultServerID is the PowerDNS server id segment used when SERVER_ID is
// unset. "localhost" is correct for typical single-server installs.
const DefaultServerID = "localhost"

// Config holds PowerDNS-specific configuration.
type Config struct {
	URL      string // Base URL, e.g. http://ns1.example.com:8081 (no /api/v1)
	APIKey   string // X-API-Key value
	Zone     string // Zone name, e.g. example.com
	ServerID string // PowerDNS server id (defaults to DefaultServerID)
	TTL      int    // Fallback TTL for created records
}

// Validate checks that all required configuration is present.
func (c *Config) Validate() error {
	var errs []string
	if c.URL == "" {
		errs = append(errs, "URL is required")
	}
	if c.APIKey == "" {
		errs = append(errs, "API_KEY is required")
	}
	if c.Zone == "" {
		errs = append(errs, "ZONE is required")
	}
	if c.TTL < 0 {
		errs = append(errs, "TTL must be non-negative")
	}
	if len(errs) > 0 {
		return fmt.Errorf("powerdns config validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// LoadConfigFromMap creates a Config from a configuration map supplied by the
// provider registry. Supported keys: URL, API_KEY, ZONE (required);
// SERVER_ID, TTL (optional). The _FILE secret resolution for API_KEY happens
// upstream in internal/config before the map reaches here.
func LoadConfigFromMap(instanceName string, config map[string]string) (*Config, error) {
	cfg := &Config{
		URL:      strings.TrimRight(config["URL"], "/"),
		APIKey:   config["API_KEY"],
		Zone:     config["ZONE"],
		ServerID: config["SERVER_ID"],
		TTL:      DefaultTTL,
	}
	if cfg.ServerID == "" {
		cfg.ServerID = DefaultServerID
	}
	if ttlStr := config["TTL"]; ttlStr != "" {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL value %q: %w", ttlStr, err)
		}
		cfg.TTL = ttl
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration for %s: %w", instanceName, err)
	}
	return cfg, nil
}

// LoadConfig loads PowerDNS configuration from environment variables.
// Pattern: DNSWEAVER_{INSTANCE_NAME}_{SETTING}. API_KEY supports the _FILE
// suffix (Docker/Kubernetes secrets).
func LoadConfig(instanceName string) (*Config, error) {
	prefix := envPrefix(instanceName)
	cfg := &Config{
		URL:      strings.TrimRight(getEnv(prefix+"URL"), "/"),
		APIKey:   getEnvOrFile(prefix+"API_KEY", prefix+"API_KEY_FILE"),
		Zone:     getEnv(prefix + "ZONE"),
		ServerID: getEnv(prefix + "SERVER_ID"),
		TTL:      DefaultTTL,
	}
	if cfg.ServerID == "" {
		cfg.ServerID = DefaultServerID
	}
	if ttlStr := getEnv(prefix + "TTL"); ttlStr != "" {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL value %q: %w", ttlStr, err)
		}
		cfg.TTL = ttl
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration for %s: %w", instanceName, err)
	}
	return cfg, nil
}

// envPrefix converts an instance name to its env var prefix.
// Example: "my-pdns" -> "DNSWEAVER_MY_PDNS_".
func envPrefix(instanceName string) string {
	normalized := strings.ToUpper(instanceName)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	return "DNSWEAVER_" + normalized + "_"
}

func getEnv(key string) string { return os.Getenv(key) }

// getEnvOrFile reads directKey, or the contents of the file named by fileKey
// (Docker secrets pattern). The file takes precedence; its contents are
// whitespace-trimmed.
func getEnvOrFile(directKey, fileKey string) string {
	if filePath := os.Getenv(fileKey); filePath != "" {
		if content, err := os.ReadFile(filePath); err == nil {
			return strings.TrimSpace(string(content))
		}
	}
	return os.Getenv(directKey)
}
