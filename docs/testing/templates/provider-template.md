# Provider Testing Template

Standardized test checklist for DNS provider implementations. Every provider must pass these baseline tests before shipping.

> **Philosophy:** *"Does what it says on the tin."* — If a provider claims to support Create, it must handle create in all documented scenarios.

## Test Categories

### 1. Connectivity

| # | Test | Description | Priority |
|---|------|-------------|----------|
| P1 | Ping success | `Ping()` returns nil against a reachable server | Required |
| P2 | Ping unreachable | `Ping()` returns error for non-routable address | Required |
| P3 | Ping canceled context | `Ping()` returns error when context is already canceled | Required |
| P4 | Ping timeout | `Ping()` returns error when server doesn't respond in time | Required |
| P5 | Auth failure | `Ping()` returns error with bad credentials/API key | Recommended |

### 2. Record CRUD

| # | Test | Description | Priority |
|---|------|-------------|----------|
| C1 | Create A record | Round-trip: create, query, verify rdata | Required |
| C2 | Create AAAA record | Round-trip with IPv6 address | Required |
| C3 | Create CNAME record | Round-trip with target FQDN | Required |
| C4 | Create TXT record | Round-trip with arbitrary text | Required |
| C5 | Create MX record | Round-trip with priority | Recommended |
| C6 | Create SRV record | Round-trip with priority, weight, port | Recommended |
| C7 | Delete specific record | Create, delete, verify gone | Required |
| C8 | Delete all by type | Create multiple, delete all of type, verify | Required |
| C9 | Update target | Create, update rdata, verify old gone + new present | Required |
| C10 | Update TTL | Create, update TTL only, verify | Recommended |
| C11 | List records | Verify List() returns seeded records | Required |
| C12 | List empty zone | List() on empty zone returns empty slice, not error | Required |

### 3. Error Handling

| # | Test | Description | Priority |
|---|------|-------------|----------|
| E1 | Out-of-zone create | Create record outside provider's zone → error | Required |
| E2 | Out-of-zone delete | Delete record outside provider's zone → error | Required |
| E3 | Network error | Correct error type when server is unreachable | Required |
| E4 | Auth error | Correct error type for bad credentials | Recommended |
| E5 | Context cancellation | Operations respect context cancellation mid-flight | Required |
| E6 | Invalid record data | Reject malformed IPs, empty names, etc. | Required |

### 4. Ownership

| # | Test | Description | Priority |
|---|------|-------------|----------|
| O1 | Create ownership TXT | Provider creates `_dnsweaver.<hostname>` TXT record | Required |
| O2 | Query ownership | Ownership record is visible in List() | Required |
| O3 | Delete ownership | Ownership record is removed on cleanup | Required |
| O4 | Ownership format | TXT value matches `heritage=dnsweaver[,instance=<id>]` | Required |

### 5. Edge Cases

| # | Test | Description | Priority |
|---|------|-------------|----------|
| X1 | Multiple records same name | Two A records for one hostname | Recommended |
| X2 | Long TXT value | Near 255-char string limit | Recommended |
| X3 | Full lifecycle | Create → query → update → query → delete → query | Required |
| X4 | Idempotent create | Creating same record twice doesn't error or duplicate | Recommended |
| X5 | Delete non-existent | Deleting a record that doesn't exist → graceful handling | Recommended |

## Implementation Pattern

```go
//go:build integration

package myprovider_test

import (
    "context"
    "os"
    "testing"
    "time"
)

func testProvider(t *testing.T) *Provider {
    t.Helper()
    apiURL := os.Getenv("MYPROVIDER_TEST_URL")
    apiKey := os.Getenv("MYPROVIDER_TEST_API_KEY")
    zone := os.Getenv("MYPROVIDER_TEST_ZONE")
    if apiURL == "" || zone == "" {
        t.Skip("MYPROVIDER_TEST_URL and MYPROVIDER_TEST_ZONE must be set")
    }
    // ... create and return provider
}

func TestIntegration_CreateAndQueryA(t *testing.T) {
    p := testProvider(t)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    // ... test logic
}
```

## Using the Conformance Suite

The shared test harness provides standard conformance tests:

```go
func TestConformance(t *testing.T) {
    p := testProvider(t)
    testutil.RunProviderConformance(t, p)
}

func TestCRUDConformance(t *testing.T) {
    p := testProvider(t)
    testutil.RunProviderCRUDConformance(t, p)
}
```

## Environment Variables

Each provider should document its required test environment variables at the top of the integration test file:

```go
// Required:
//   MYPROVIDER_TEST_URL    - API endpoint
//   MYPROVIDER_TEST_ZONE   - Test zone (must allow updates)
//
// Optional:
//   MYPROVIDER_TEST_API_KEY - API key (if auth required)
```
