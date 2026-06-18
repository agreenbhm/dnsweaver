package ovh

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// DefaultTTL is the default TTL for OVH DNS records.
// OVH's minimum non-zero TTL is 60 seconds; 0 means "use the zone default".
const DefaultTTL = 3600

// DefaultEndpoint is the OVH API region used when none is configured.
const DefaultEndpoint = "ovh-eu"

// endpoints maps OVH API region identifiers to their base URLs.
// See https://api.ovh.com/ for the full list of regions.
var endpoints = map[string]string{
	"ovh-eu":        "https://eu.api.ovh.com/1.0",
	"ovh-ca":        "https://ca.api.ovh.com/1.0",
	"ovh-us":        "https://api.us.ovhcloud.com/1.0",
	"kimsufi-eu":    "https://eu.api.kimsufi.com/1.0",
	"kimsufi-ca":    "https://ca.api.kimsufi.com/1.0",
	"soyoustart-eu": "https://eu.api.soyoustart.com/1.0",
	"soyoustart-ca": "https://ca.api.soyoustart.com/1.0",
}

// Config holds OVH-specific configuration.
//
// OVH authenticates API calls with a triple of credentials created via the
// OVH API token wizard (https://api.ovh.com/createToken/): an application key,
// an application secret, and a consumer key. Each request is signed with these
// values plus the OVH server time.
type Config struct {
	ApplicationKey    string // X-Ovh-Application header
	ApplicationSecret string // used to sign requests (secret)
	ConsumerKey       string // X-Ovh-Consumer header (secret)
	Endpoint          string // API region (e.g. "ovh-eu"); defaults to DefaultEndpoint
	Zone              string // DNS zone name (e.g. "example.com")
	TTL               int    // Record TTL (defaults to DefaultTTL; 0 = zone default)
}

// EndpointURL returns the API base URL for the configured endpoint region.
func (c *Config) EndpointURL() string {
	return endpoints[c.Endpoint]
}

// Validate checks that all required configuration is present and well-formed.
func (c *Config) Validate() error {
	var errs []string

	if c.ApplicationKey == "" {
		errs = append(errs, "APPLICATION_KEY is required")
	}
	if c.ApplicationSecret == "" {
		errs = append(errs, "APPLICATION_SECRET is required")
	}
	if c.ConsumerKey == "" {
		errs = append(errs, "CONSUMER_KEY is required")
	}
	if c.Zone == "" {
		errs = append(errs, "ZONE is required")
	}
	if c.Endpoint == "" {
		errs = append(errs, "ENDPOINT is required")
	} else if _, ok := endpoints[c.Endpoint]; !ok {
		errs = append(errs, fmt.Sprintf("ENDPOINT %q is not a known OVH region (valid: %s)", c.Endpoint, validEndpoints()))
	}
	if c.TTL < 0 {
		errs = append(errs, "TTL must be non-negative")
	}
	// OVH minimum TTL is 60 seconds; 0 means "use the zone default".
	if c.TTL > 0 && c.TTL < 60 {
		errs = append(errs, "TTL must be at least 60 seconds (or 0 for the zone default)")
	}

	if len(errs) > 0 {
		return fmt.Errorf("ovh config validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// validEndpoints returns a sorted, comma-separated list of known endpoints
// for use in error messages.
func validEndpoints() string {
	keys := make([]string, 0, len(endpoints))
	for k := range endpoints {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// LoadConfig loads OVH configuration from environment variables.
// Environment variable pattern: DNSWEAVER_{INSTANCE_NAME}_{SETTING}
//
// Instance names are normalized: lowercase with hyphens becomes uppercase with underscores.
// Example: "public-dns" looks for DNSWEAVER_PUBLIC_DNS_*
//
// Supported settings:
//   - APPLICATION_KEY: OVH application key (required, supports _FILE suffix)
//   - APPLICATION_SECRET: OVH application secret (required, supports _FILE suffix)
//   - CONSUMER_KEY: OVH consumer key (required, supports _FILE suffix)
//   - ZONE: DNS zone name (required)
//   - ENDPOINT: API region (optional, defaults to "ovh-eu")
//   - TTL: Record TTL (optional, defaults to 3600; 0 = zone default)
func LoadConfig(instanceName string) (*Config, error) {
	prefix := envPrefix(instanceName)

	config := &Config{
		ApplicationKey:    getEnvOrFile(prefix+"APPLICATION_KEY", prefix+"APPLICATION_KEY_FILE"),
		ApplicationSecret: getEnvOrFile(prefix+"APPLICATION_SECRET", prefix+"APPLICATION_SECRET_FILE"),
		ConsumerKey:       getEnvOrFile(prefix+"CONSUMER_KEY", prefix+"CONSUMER_KEY_FILE"),
		Zone:              getEnv(prefix + "ZONE"),
		Endpoint:          DefaultEndpoint,
		TTL:               DefaultTTL,
	}

	if endpoint := getEnv(prefix + "ENDPOINT"); endpoint != "" {
		config.Endpoint = endpoint
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

// LoadConfigFromMap creates a Config from a configuration map.
// This is used by the Factory to parse provider-specific configuration.
//
// Supported keys mirror LoadConfig (without the _FILE suffix, which is
// resolved upstream by the config loader):
//   - APPLICATION_KEY, APPLICATION_SECRET, CONSUMER_KEY (required)
//   - ZONE (required)
//   - ENDPOINT (optional, defaults to "ovh-eu")
//   - TTL (optional, defaults to 3600)
func LoadConfigFromMap(instanceName string, config map[string]string) (*Config, error) {
	cfg := &Config{
		ApplicationKey:    config["APPLICATION_KEY"],
		ApplicationSecret: config["APPLICATION_SECRET"],
		ConsumerKey:       config["CONSUMER_KEY"],
		Zone:              config["ZONE"],
		Endpoint:          DefaultEndpoint,
		TTL:               DefaultTTL,
	}

	if endpoint := config["ENDPOINT"]; endpoint != "" {
		cfg.Endpoint = endpoint
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

// envPrefix converts an instance name to an environment variable prefix.
// Example: "public-dns" → "DNSWEAVER_PUBLIC_DNS_"
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
// If the file key is set and readable, the file contents take precedence.
// The file contents are trimmed of leading/trailing whitespace.
func getEnvOrFile(directKey, fileKey string) string {
	if filePath := os.Getenv(fileKey); filePath != "" {
		content, err := os.ReadFile(filePath)
		if err == nil {
			return strings.TrimSpace(string(content))
		}
		// If file read fails, fall through to direct value.
	}

	return os.Getenv(directKey)
}
