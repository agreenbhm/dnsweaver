# Configuration Testing Template

Standardized test checklist for configuration loading, validation, and secrets handling. Applies to both the top-level app config and per-provider/source configs.

> **Philosophy:** Configuration errors should fail fast, fail loud, and tell the operator exactly what's wrong.

## Test Categories

### 1. Environment Variables

| # | Test | Description | Priority |
|---|------|-------------|----------|
| V1 | All required vars | Config loads successfully with all required vars set | Required |
| V2 | Missing required var | Clear error naming the missing variable | Required |
| V3 | Empty string var | Empty string treated as unset for required fields | Required |
| V4 | Default values | Omitted optional vars use documented defaults | Required |
| V5 | Boolean parsing | `"true"`, `"1"`, `"yes"` all accepted | Recommended |
| V6 | Duration parsing | `"10s"`, `"5m"`, `"1h"` parsed correctly | Recommended |
| V7 | Integer parsing | Invalid integer → clear error | Recommended |
| V8 | Case sensitivity | Env var names are case-sensitive (standard behavior) | Required |

### 2. YAML / File Config

| # | Test | Description | Priority |
|---|------|-------------|----------|
| Y1 | Valid YAML | Complete config file loads without error | Required |
| Y2 | Minimal YAML | Only required fields → loads with defaults | Required |
| Y3 | Invalid YAML | Malformed YAML → parse error, not panic | Required |
| Y4 | Unknown fields | Extra YAML fields → warning or ignored (not error) | Recommended |
| Y5 | Type mismatch | String where int expected → clear error | Required |
| Y6 | File not found | Missing config file → clear error with path | Required |

### 3. Validation

| # | Test | Description | Priority |
|---|------|-------------|----------|
| D1 | Zone format | Zone must end with `.` → error if missing | Required |
| D2 | Server format | Server address validated (host:port) | Required |
| D3 | TSIG completeness | If any TSIG field set, all required TSIG fields must be set | Required |
| D4 | TSIG algorithm | Unsupported algorithm → error listing valid options | Required |
| D5 | Timeout range | Negative timeout → error | Required |
| D6 | Instance naming | Instance names validated (alphanumeric, hyphens) | Recommended |
| D7 | Duplicate instances | Two instances with same name → error | Recommended |
| D8 | Domain pattern | Provider domain patterns validated | Recommended |

### 4. Secrets

| # | Test | Description | Priority |
|---|------|-------------|----------|
| S1 | File-based secrets | `_FILE` suffix reads from file path | Recommended |
| S2 | Secret trimming | Trailing newlines stripped from file secrets | Recommended |
| S3 | Secret not logged | Config `String()`/log output redacts secrets | Required |
| S4 | Missing secret file | `_FILE` pointing to non-existent file → clear error | Recommended |

### 5. Multi-Instance

| # | Test | Description | Priority |
|---|------|-------------|----------|
| I1 | Multiple providers | `DNSWEAVER_INSTANCES=a,b` loads both | Required |
| I2 | Different types | Mixed provider types in one config | Required |
| I3 | Independent configs | Each instance has its own config namespace | Required |
| I4 | Instance list parsing | Whitespace, trailing commas handled | Recommended |

## Implementation Pattern

```go
package config_test

import (
    "testing"

    "gitlab.bluewillows.net/root/dnsweaver/internal/config"
)

func TestLoad_MinimalConfig(t *testing.T) {
    t.Setenv("DNSWEAVER_INSTANCES", "test")
    t.Setenv("DNSWEAVER_TEST_TYPE", "technitium")
    t.Setenv("DNSWEAVER_TEST_URL", "http://dns.example.com")
    t.Setenv("DNSWEAVER_TEST_API_KEY", "secret")
    t.Setenv("DNSWEAVER_TEST_ZONE", "example.com.")
    t.Setenv("DNSWEAVER_TEST_DOMAINS", "example.com")

    cfg, err := config.Load()
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if len(cfg.Instances) != 1 {
        t.Fatalf("expected 1 instance, got %d", len(cfg.Instances))
    }
}

func TestLoad_MissingRequired(t *testing.T) {
    t.Setenv("DNSWEAVER_INSTANCES", "test")
    // Missing TYPE
    _, err := config.Load()
    if err == nil {
        t.Fatal("expected error for missing type")
    }
}
```

## Validation Error Quality

Config errors should be:

1. **Specific** — Name the field and what's wrong
2. **Aggregated** — Report all validation errors at once, not just the first
3. **Actionable** — Include expected format or valid options
4. **Non-panicking** — Return errors, never crash

```
// Good error:
"dnsupdate config validation failed: zone must end with a dot (e.g., 'example.com.')"

// Bad error:
"invalid config"
```
