package technitium

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// DefaultTTL is the default TTL for Technitium DNS records.
const DefaultTTL = 300

// DefaultHTTPSALPN is the default ALPN value for auto-created HTTPS companion records.
const DefaultHTTPSALPN = "h2"

// Config holds Technitium-specific configuration.
type Config struct {
	URL   string // Technitium API URL (e.g., http://dns:5380)
	Token string // API token
	Zone  string // DNS zone to manage
	TTL   int    // Record TTL (defaults to DefaultTTL)

	// AutoHTTPSRecords enables automatic companion HTTPS (SVCB Type 65) record creation.
	// When true (the default for Technitium), creating an A or CNAME record also creates a
	// companion HTTPS record (e.g., "HTTPS 1 . alpn=h2") to prevent ECH fallback errors
	// in split-horizon DNS environments. Deleting the parent record also removes the companion.
	// Set AUTO_HTTPS_RECORDS=false to disable.
	// See RFC 9460 and issue #158 for details.
	AutoHTTPSRecords bool

	// AutoHTTPSALPN is the ALPN protocol value for auto-created HTTPS companion records.
	// Defaults to "h2". Common values: "h2" (HTTP/2), "h3" (HTTP/3), "h2,h3" (both).
	AutoHTTPSALPN string
}

// Validate checks that all required configuration is present.
func (c *Config) Validate() error {
	var errs []string

	if c.URL == "" {
		errs = append(errs, "URL is required")
	} else {
		// Validate URL format and scheme
		parsed, err := url.Parse(c.URL)
		if err != nil {
			errs = append(errs, fmt.Sprintf("invalid URL: %v", err))
		} else if parsed.Scheme != "http" && parsed.Scheme != "https" {
			errs = append(errs, "URL must start with http:// or https://")
		} else if parsed.User != nil {
			errs = append(errs, "URL must not contain embedded credentials")
		}
	}
	if c.Token == "" {
		errs = append(errs, "TOKEN is required")
	}
	if c.Zone == "" {
		errs = append(errs, "ZONE is required")
	}
	if c.TTL < 0 {
		errs = append(errs, "TTL must be non-negative")
	}

	if len(errs) > 0 {
		return fmt.Errorf("technitium config validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// LoadConfig loads Technitium configuration from environment variables.
// Environment variable pattern: DNSWEAVER_{INSTANCE_NAME}_{SETTING}
//
// Instance names are normalized: lowercase with hyphens becomes uppercase with underscores.
// Example: "internal-dns" looks for DNSWEAVER_INTERNAL_DNS_*
//
// Supported settings:
//   - URL: Technitium API URL (required)
//   - TOKEN: API token (required, supports _FILE suffix for Docker secrets)
//   - ZONE: DNS zone to manage (required)
//   - TTL: Record TTL (optional, defaults to 300)
//   - AUTO_HTTPS_RECORDS: Enable companion HTTPS record creation (optional, defaults to true)
//   - AUTO_HTTPS_ALPN: ALPN value for HTTPS records (optional, defaults to "h2")
func LoadConfig(instanceName string) (*Config, error) {
	prefix := envPrefix(instanceName)

	config := &Config{
		URL:              getEnv(prefix + "URL"),
		Token:            getEnvOrFile(prefix+"TOKEN", prefix+"TOKEN_FILE"),
		Zone:             getEnv(prefix + "ZONE"),
		TTL:              DefaultTTL,
		AutoHTTPSRecords: true, // Enabled by default for Technitium
	}

	// Parse optional TTL
	if ttlStr := getEnv(prefix + "TTL"); ttlStr != "" {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL value %q: %w", ttlStr, err)
		}
		config.TTL = ttl
	}

	// Parse optional AutoHTTPSRecords (companion HTTPS record creation)
	// Default is true for Technitium; only override if explicitly set
	if autoHTTPS := getEnv(prefix + "AUTO_HTTPS_RECORDS"); autoHTTPS != "" {
		config.AutoHTTPSRecords = strings.EqualFold(autoHTTPS, "true") || autoHTTPS == "1"
	}

	// Parse optional AutoHTTPSALPN (default: "h2")
	config.AutoHTTPSALPN = DefaultHTTPSALPN
	if alpn := getEnv(prefix + "AUTO_HTTPS_ALPN"); alpn != "" {
		config.AutoHTTPSALPN = alpn
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration for %s: %w", instanceName, err)
	}

	return config, nil
}

// envPrefix converts an instance name to an environment variable prefix.
// Example: "internal-dns" → "DNSWEAVER_INTERNAL_DNS_"
func envPrefix(instanceName string) string {
	normalized := strings.ToUpper(instanceName)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	return "DNSWEAVER_" + normalized + "_"
}

// getEnv retrieves an environment variable value.
func getEnv(key string) string {
	return os.Getenv(key)
}

// getEnvOrFile retrieves a value from either a direct environment variable
// or a file path specified by the file key (Docker secrets pattern).
//
// If both are set, the file takes precedence.
// The file contents are trimmed of leading/trailing whitespace.
func getEnvOrFile(directKey, fileKey string) string {
	// Check for file-based secret first (Docker secrets pattern)
	if filePath := os.Getenv(fileKey); filePath != "" {
		content, err := os.ReadFile(filePath)
		if err == nil {
			return strings.TrimSpace(string(content))
		}
		// If file read fails, fall through to direct value
	}

	return os.Getenv(directKey)
}

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// IsConfigError returns true if the error is a configuration error.
func IsConfigError(err error) bool {
	var configErr *ConfigError
	return errors.As(err, &configErr)
}

// LoadConfigFromMap creates a Config from a map of key-value pairs.
// This is used by the provider registry to create instances from
// configuration that was already parsed from environment variables.
//
// Required keys: URL, TOKEN, ZONE
// Optional keys: TTL (defaults to 300)
func LoadConfigFromMap(instanceName string, configMap map[string]string) (*Config, error) {
	config := &Config{
		URL:              configMap["URL"],
		Token:            configMap["TOKEN"],
		Zone:             configMap["ZONE"],
		TTL:              DefaultTTL,
		AutoHTTPSRecords: true, // Enabled by default for Technitium
	}

	// Parse optional TTL
	if ttlStr, ok := configMap["TTL"]; ok && ttlStr != "" {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL value %q: %w", ttlStr, err)
		}
		config.TTL = ttl
	}

	// Parse optional AutoHTTPSRecords
	// Default is true for Technitium; only override if explicitly set
	if autoHTTPS, ok := configMap["AUTO_HTTPS_RECORDS"]; ok && autoHTTPS != "" {
		config.AutoHTTPSRecords = strings.EqualFold(autoHTTPS, "true") || autoHTTPS == "1"
	}

	// Parse optional AutoHTTPSALPN (default: "h2")
	config.AutoHTTPSALPN = DefaultHTTPSALPN
	if alpn, ok := configMap["AUTO_HTTPS_ALPN"]; ok && alpn != "" {
		config.AutoHTTPSALPN = alpn
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration for %s: %w", instanceName, err)
	}

	return config, nil
}
