// Package dnsmasq implements the DNSWeaver provider interface for dnsmasq DNS server.
package dnsmasq

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DefaultTTL is the default TTL for dnsmasq DNS records.
// Note: dnsmasq doesn't use TTL for local records, but we track it for consistency.
const DefaultTTL = 300

// DefaultConfigDir is the default directory for dnsmasq configuration files.
const DefaultConfigDir = "/etc/dnsmasq.d"

// DefaultConfigFile is the default filename for dnsweaver-managed records.
const DefaultConfigFile = "dnsweaver.conf"

// DefaultReloadCommand is the default command to reload dnsmasq configuration.
const DefaultReloadCommand = "systemctl reload dnsmasq"

// DefaultSSHStrictHostKey is the default for SSH host key verification.
// Verification is on by default (fail closed): a remote SSH instance must
// either provide a known_hosts file or explicitly opt out by setting
// SSH_STRICT_HOST_KEY_CHECKING=false.
const DefaultSSHStrictHostKey = true

// Config holds dnsmasq-specific configuration.
type Config struct {
	ConfigDir     string // Directory for config files (e.g., /etc/dnsmasq.d)
	ConfigFile    string // Filename for dnsweaver records (e.g., dnsweaver.conf)
	ReloadCommand string // Command to reload dnsmasq (e.g., "systemctl reload dnsmasq")
	Zone          string // DNS zone for record filtering (optional)
	TTL           int    // Record TTL (for consistency with other providers)

	// SSH configuration for remote dnsmasq management (optional)
	SSHHost     string // SSH host (e.g., "pihole.local" or "192.168.1.100")
	SSHPort     int    // SSH port (default: 22)
	SSHUser     string // SSH username
	SSHKeyFile  string // Path to SSH private key file
	SSHPassword string // SSH password (alternative to key, not recommended)

	// SSH host key verification (optional, recommended for untrusted networks)
	SSHKnownHostsFile string // Path to an OpenSSH known_hosts file; enables host key verification
	SSHStrictHostKey  bool   // Require host key verification; fails if no known_hosts file is provided
}

// Validate checks that all required configuration is present.
func (c *Config) Validate() error {
	var errs []string

	if c.ConfigDir == "" {
		errs = append(errs, "CONFIG_DIR is required")
	}
	if c.ConfigFile == "" {
		errs = append(errs, "CONFIG_FILE is required")
	}
	if c.ReloadCommand == "" {
		errs = append(errs, "RELOAD_COMMAND is required")
	}
	if c.TTL < 0 {
		errs = append(errs, "TTL must be non-negative")
	}

	// SSH validation: if any SSH option is set, host and user are required
	if c.IsSSHEnabled() {
		if c.SSHHost == "" {
			errs = append(errs, "SSH_HOST is required when SSH is enabled")
		}
		if c.SSHUser == "" {
			errs = append(errs, "SSH_USER is required when SSH is enabled")
		}
		// Either key or password required (but key preferred)
		if c.SSHKeyFile == "" && c.SSHPassword == "" {
			errs = append(errs, "SSH_KEY_FILE or SSH_PASSWORD is required when SSH is enabled")
		}
		// Strict host key checking needs a known_hosts file to verify against.
		// Strict is the default, so guide the operator to either provide a
		// known_hosts file or explicitly opt out.
		if c.SSHStrictHostKey && c.SSHKnownHostsFile == "" {
			errs = append(errs, "SSH host key verification is enabled by default: set SSH_KNOWN_HOSTS_FILE to a known_hosts file, or set SSH_STRICT_HOST_KEY_CHECKING=false to disable verification (insecure)")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("dnsmasq config validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// IsSSHEnabled returns true if SSH configuration is provided.
func (c *Config) IsSSHEnabled() bool {
	return c.SSHHost != "" || c.SSHUser != "" || c.SSHKeyFile != "" || c.SSHPassword != ""
}

// ConfigFilePath returns the full path to the dnsweaver config file.
func (c *Config) ConfigFilePath() string {
	return c.ConfigDir + "/" + c.ConfigFile
}

// LoadConfig loads dnsmasq configuration from environment variables.
// Environment variable pattern: DNSWEAVER_{INSTANCE_NAME}_{SETTING}
//
// Instance names are normalized: lowercase with hyphens becomes uppercase with underscores.
// Example: "pihole-dns" looks for DNSWEAVER_PIHOLE_DNS_*
//
// Supported settings:
//   - CONFIG_DIR: Directory for config files (default: /etc/dnsmasq.d)
//   - CONFIG_FILE: Filename for dnsweaver records (default: dnsweaver.conf)
//   - RELOAD_COMMAND: Command to reload dnsmasq (default: systemctl reload dnsmasq)
//   - ZONE: DNS zone for record filtering (optional)
//   - TTL: Record TTL (optional, default: 300)
//   - SSH_HOST: Remote SSH host (optional, for remote management)
//   - SSH_PORT: SSH port (optional, default: 22)
//   - SSH_USER: SSH username (required if SSH_HOST set)
//   - SSH_KEY_FILE: Path to SSH private key (supports _FILE suffix for Docker secrets)
//   - SSH_PASSWORD: SSH password (not recommended, use SSH_KEY_FILE)
//   - SSH_KNOWN_HOSTS_FILE: Path to an OpenSSH known_hosts file (enables host key verification)
//   - SSH_STRICT_HOST_KEY_CHECKING: Require host key verification (default: true)
func LoadConfig(instanceName string) (*Config, error) {
	prefix := envPrefix(instanceName)

	config := &Config{
		ConfigDir:         getEnvWithDefault(prefix+"CONFIG_DIR", DefaultConfigDir),
		ConfigFile:        getEnvWithDefault(prefix+"CONFIG_FILE", DefaultConfigFile),
		ReloadCommand:     getEnvWithDefault(prefix+"RELOAD_COMMAND", DefaultReloadCommand),
		Zone:              getEnv(prefix + "ZONE"),
		TTL:               DefaultTTL,
		SSHHost:           getEnv(prefix + "SSH_HOST"),
		SSHUser:           getEnv(prefix + "SSH_USER"),
		SSHKeyFile:        getEnvOrFile(prefix+"SSH_KEY_FILE", prefix+"SSH_KEY_FILE_FILE"),
		SSHPassword:       getEnvOrFile(prefix+"SSH_PASSWORD", prefix+"SSH_PASSWORD_FILE"),
		SSHKnownHostsFile: getEnvOrFile(prefix+"SSH_KNOWN_HOSTS_FILE", prefix+"SSH_KNOWN_HOSTS_FILE_FILE"),
		SSHStrictHostKey:  parseBoolEnvDefault(prefix+"SSH_STRICT_HOST_KEY_CHECKING", DefaultSSHStrictHostKey),
	}

	// Parse optional TTL
	if ttlStr := getEnv(prefix + "TTL"); ttlStr != "" {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL value %q: %w", ttlStr, err)
		}
		config.TTL = ttl
	}

	// Parse optional SSH port
	if portStr := getEnv(prefix + "SSH_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SSH_PORT value %q: %w", portStr, err)
		}
		config.SSHPort = port
	} else if config.SSHHost != "" {
		config.SSHPort = 22 // Default SSH port
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
// Required keys: CONFIG_DIR, CONFIG_FILE, RELOAD_COMMAND
// Optional keys: ZONE, TTL, SSH_HOST, SSH_PORT, SSH_USER, SSH_KEY_FILE, SSH_PASSWORD,
// SSH_KNOWN_HOSTS_FILE, SSH_STRICT_HOST_KEY_CHECKING
func LoadConfigFromMap(instanceName string, configMap map[string]string) (*Config, error) {
	config := &Config{
		ConfigDir:         getMapWithDefault(configMap, "CONFIG_DIR", DefaultConfigDir),
		ConfigFile:        getMapWithDefault(configMap, "CONFIG_FILE", DefaultConfigFile),
		ReloadCommand:     getMapWithDefault(configMap, "RELOAD_COMMAND", DefaultReloadCommand),
		Zone:              configMap["ZONE"],
		TTL:               DefaultTTL,
		SSHHost:           configMap["SSH_HOST"],
		SSHUser:           configMap["SSH_USER"],
		SSHKeyFile:        configMap["SSH_KEY_FILE"],
		SSHPassword:       configMap["SSH_PASSWORD"],
		SSHKnownHostsFile: configMap["SSH_KNOWN_HOSTS_FILE"],
		SSHStrictHostKey:  parseBoolValueDefault(configMap["SSH_STRICT_HOST_KEY_CHECKING"], DefaultSSHStrictHostKey),
	}

	// Parse optional TTL
	if ttlStr, ok := configMap["TTL"]; ok && ttlStr != "" {
		ttl, err := strconv.Atoi(ttlStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL value %q: %w", ttlStr, err)
		}
		config.TTL = ttl
	}

	// Parse optional SSH port
	if portStr, ok := configMap["SSH_PORT"]; ok && portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SSH_PORT value %q: %w", portStr, err)
		}
		config.SSHPort = port
	} else if config.SSHHost != "" {
		config.SSHPort = 22
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration for %s: %w", instanceName, err)
	}

	return config, nil
}

// envPrefix converts an instance name to an environment variable prefix.
// Example: "pihole-dns" → "DNSWEAVER_PIHOLE_DNS_"
func envPrefix(instanceName string) string {
	normalized := strings.ToUpper(instanceName)
	normalized = strings.ReplaceAll(normalized, "-", "_")
	return "DNSWEAVER_" + normalized + "_"
}

// getEnv retrieves an environment variable value.
func getEnv(key string) string {
	return os.Getenv(key)
}

// getEnvWithDefault retrieves an environment variable value with a default.
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getMapWithDefault retrieves a map value with a default.
func getMapWithDefault(m map[string]string, key, defaultValue string) string {
	if value, ok := m[key]; ok && value != "" {
		return value
	}
	return defaultValue
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

// parseBoolEnvDefault reads an environment variable and parses it as a boolean,
// returning def when the variable is unset or empty.
func parseBoolEnvDefault(key string, def bool) bool {
	return parseBoolValueDefault(os.Getenv(key), def)
}

// parseBoolValueDefault parses a string as a boolean, returning def when the
// value is unset or empty. Recognizes "true"/"false" (case-insensitive); any
// other non-empty value falls back to def.
func parseBoolValueDefault(value string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return true
	case "false":
		return false
	default:
		return def
	}
}
