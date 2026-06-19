# Testing Guide

dnsweaver maintains a comprehensive test suite organized by component type. This guide covers test conventions, the shared test harness, and standardized templates for writing new tests.

## Quick Reference

```bash
# Run all unit tests
make test

# Run with coverage
make test-cover

# Run short tests only
make test-short

# Run integration tests (requires env vars)
make test-integration
```

## Test Organization

| Directory | Scope | Build Tag |
|-----------|-------|-----------|
| `internal/*/` | Unit tests (internal packages) | — |
| `pkg/*/` | Unit tests (public API) | — |
| `providers/*/` | Provider unit tests (HTTP mocks) | — |
| `sources/*/` | Source unit tests | — |
| `pkg/dnsupdate/` | RFC 2136 integration tests | `integration` |

## Shared Test Harness

The `internal/testutil` package provides reusable test infrastructure:

### Record Builders

```go
import "github.com/maxfield-allison/dnsweaver/internal/testutil"

rec := testutil.ARecord("app.example.com", "192.0.2.1")
txt := testutil.TXTRecord("app.example.com", "heritage=dnsweaver")
own := testutil.OwnershipRecord("app.example.com")
srv := testutil.SRVRecord("_http._tcp.app.example.com", "app.example.com", 80)
```

### Mock Provider

```go
mock := testutil.NewMockProvider("test-provider", "example.com.")
mock.AddRecords(
    testutil.ARecord("app.example.com", "192.0.2.1"),
)
mock.SetCreateFunc(func(ctx context.Context, rec provider.Record) error {
    return nil // custom behavior
})
```

### Assertions

```go
testutil.RequireNoError(t, err)
testutil.RequireError(t, err)
testutil.RequireErrorContains(t, err, "connection refused")
testutil.AssertEqual(t, got, want)
testutil.AssertLen(t, records, 3)
testutil.AssertRecordExists(t, records, "app.example.com", "A")
```

### Conformance Suite

```go
// Run standard behavioral tests for any Provider implementation
testutil.RunProviderConformance(t, myProvider)

// Run CRUD round-trip tests
testutil.RunProviderCRUDConformance(t, myProvider)
```

### Mock Server (HTTP)

```go
server := testutil.NewMockServer(t)
server.Handle("/api/zones/list", testutil.JSONResponse(200, zonesPayload))
defer server.Close()
// Use server.URL as the provider API endpoint
```

## Writing Tests

### Conventions

1. **Table-driven tests** for input/output variations
2. **Subtests** (`t.Run`) for logical grouping
3. **`t.Helper()`** in all helper functions
4. **`t.Cleanup()`** over `defer` for resource cleanup
5. **`t.Parallel()`** where safe (no shared mutable state)
6. **No sleep-based waits** — use channels, contexts, or polling

### File Naming

| Pattern | Purpose |
|---------|---------|
| `*_test.go` | Standard unit tests |
| `*_integration_test.go` | Integration tests (with build tag) |
| `edge_*_test.go` | Edge case tests |
| `testutil_test.go` | Package-private test helpers |

### Build Tags

Integration tests that require external infrastructure use build tags:

```go
//go:build integration

package mypackage_test
```

Run with: `go test -tags=integration ./...`

## Templates

Standardized checklists for each component type. Use these when adding a new provider, source, or core component:

- [Provider Testing Template](templates/provider-template.md)
- [Source Testing Template](templates/source-template.md)
- [Core Component Testing Template](templates/core-template.md)
- [Configuration Testing Template](templates/config-template.md)
- [Observability Testing Template](templates/observability-template.md)
