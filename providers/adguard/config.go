// Package adguard implements the DNSWeaver provider interface for AdGuard Home DNS.
package adguard

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// DefaultTTL is the default TTL for AdGuard Home DNS records.
// Note: AdGuard Home rewrites don't have per-record TTL, but we track it for consistency.
const DefaultTTL = 300

// Config holds AdGuard Home-specific configuration.
type Config struct {
	// URL is the AdGuard Home web interface URL (e.g., "http://adguard.local:3000").
	URL string

	// Username for basic auth (AdGuard Home admin username).
	Username string

	// Password for basic auth (AdGuard Home admin password).
	Password string

	// Zone is the DNS zone for record filtering (optional).
	// When set, only records matching this zone suffix are returned by List.
	Zone string

	// TTL is the record TTL (for consistency with other providers).
	TTL int
}

// Validate checks that all required configuration is present.
func (c *Config) Validate() error {
	var errs []string

	if c.URL == "" {
		errs = append(errs, "URL is required")
	} else {
		parsed, err := url.Parse(c.URL)
		if err != nil {
			errs = append(errs, fmt.Sprintf("invalid URL: %v", err))
		} else if parsed.Scheme != "http" && parsed.Scheme != "https" {
			errs = append(errs, "URL must start with http:// or https://")
		} else if parsed.User != nil {
			errs = append(errs, "URL must not contain embedded credentials")
		}
	}

	if c.Username == "" {
		errs = append(errs, "USERNAME is required")
	}

	if c.Password == "" {
		errs = append(errs, "PASSWORD is required")
	}

	if c.TTL < 0 {
		errs = append(errs, "TTL must be non-negative")
	}

	if len(errs) > 0 {
		return fmt.Errorf("adguard config validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// LoadConfig loads AdGuard Home configuration from environment variables.
// Environment variable pattern: DNSWEAVER_{INSTANCE_NAME}_{SETTING}
//
// Instance names are normalized: lowercase with hyphens becomes uppercase with underscores.
// Example: "adguard-dns" looks for DNSWEAVER_ADGUARD_DNS_*
//
// Supported settings:
//   - URL: AdGuard Home web URL (e.g., "http://adguard.local:3000")
//   - USERNAME: Admin username
//   - PASSWORD: Admin password (supports _FILE suffix for Docker secrets)
//   - ZONE: DNS zone for record filtering (optional)
//   - TTL: Record TTL (optional, default: 300)
func LoadConfig(instanceName string) (*Config, error) {
	prefix := envPrefix(instanceName)

	config := &Config{
		URL:      getEnv(prefix + "URL"),
		Username: getEnv(prefix + "USERNAME"),
		Password: getEnvOrFile(prefix+"PASSWORD", prefix+"PASSWORD_FILE"),
		Zone:     getEnv(prefix + "ZONE"),
		TTL:      DefaultTTL,
	}

	if ttlStr := getEnv(prefix + "TTL"); ttlStr != "" {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL value %q: %w", ttlStr, err)
		}
		config.TTL = ttl
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration for %s: %w", instanceName, err)
	}

	return config, nil
}

// LoadConfigFromMap creates a Config from a map of key-value pairs.
// This is used by the provider registry to create instances from
// configuration that was already parsed from environment variables.
//
// Expected keys (case-insensitive):
//   - url: AdGuard Home web URL
//   - username: Admin username
//   - password: Admin password
//   - zone: DNS zone
//   - ttl: Record TTL
func LoadConfigFromMap(name string, m map[string]string) (*Config, error) {
	config := &Config{
		URL:      getMapValue(m, "url"),
		Username: getMapValue(m, "username"),
		Password: getMapValue(m, "password"),
		Zone:     getMapValue(m, "zone"),
		TTL:      DefaultTTL,
	}

	if ttlStr := getMapValue(m, "ttl"); ttlStr != "" {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL value %q: %w", ttlStr, err)
		}
		config.TTL = ttl
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration for %s: %w", name, err)
	}

	return config, nil
}

// envPrefix returns the environment variable prefix for a provider instance.
func envPrefix(instanceName string) string {
	normalized := strings.ToUpper(strings.ReplaceAll(instanceName, "-", "_"))
	return "DNSWEAVER_" + normalized + "_"
}

func getEnv(key string) string {
	return os.Getenv(key)
}

func getEnvOrFile(envKey, fileKey string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if filePath := os.Getenv(fileKey); filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
	return ""
}

// getMapValue returns a value from a map, case-insensitively.
func getMapValue(m map[string]string, key string) string {
	if v, ok := m[key]; ok {
		return v
	}
	if v, ok := m[strings.ToUpper(key)]; ok {
		return v
	}
	if v, ok := m[strings.ToLower(key)]; ok {
		return v
	}
	return ""
}
