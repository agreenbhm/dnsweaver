# Core Component Testing Template

Standardized test checklist for reconciler and core engine components. These tests validate the central orchestration logic that ties providers and sources together.

> **Philosophy:** The reconciler is the brain of dnsweaver. It must handle every combination of provider state, source output, and failure mode gracefully.

## Test Categories

### 1. Reconciliation

| # | Test | Description | Priority |
|---|------|-------------|----------|
| R1 | Basic reconcile | Source hostnames → DNS records created | Required |
| R2 | No-op reconcile | All records already correct → no mutations | Required |
| R3 | Target change | Source target changes → record updated | Required |
| R4 | Hostname removed | Source no longer reports hostname → orphan cleanup | Required |
| R5 | New hostname added | Source adds new hostname → record created | Required |
| R6 | Multi-provider | Hostnames routed to correct providers by domain match | Required |
| R7 | Disabled reconciler | `SetEnabled(false)` → reconcile returns immediately | Required |

### 2. Provider Matching

| # | Test | Description | Priority |
|---|------|-------------|----------|
| M1 | Exact domain match | `app.example.com` matches `example.com.` provider | Required |
| M2 | Wildcard match | `*.example.com` provider matches subdomains | Required |
| M3 | Overlapping domains | Most-specific provider wins for overlapping zones | Required |
| M4 | No matching provider | Hostname with no provider match → skipped with warning | Required |
| M5 | Multiple providers same domain | Both providers receive the record | Recommended |

### 3. Orphan Cleanup

| # | Test | Description | Priority |
|---|------|-------------|----------|
| O1 | Managed mode | Orphan with ownership → deleted | Required |
| O2 | Authoritative mode | All orphans deleted regardless of ownership | Required |
| O3 | Additive mode | Orphans never deleted | Required |
| O4 | Circuit breaker | >50% orphan ratio (with count >1) → abort deletion | Required |
| O5 | No false positives | Active hostnames are never orphaned | Required |

### 4. State Recovery

| # | Test | Description | Priority |
|---|------|-------------|----------|
| S1 | Cold start | First reconcile with no previous state → creates all | Required |
| S2 | Provider restart | Provider returns to healthy → records reconciled | Required |
| S3 | Ownership recovery | `RecoverOwnership()` reclaims existing records | Recommended |
| S4 | Concurrent reconcile | Parallel `Reconcile()` calls are safe (race-free) | Required |
| S5 | SetEnabled during reconcile | Toggling enabled mid-reconcile is safe | Required |

### 5. Failure Handling

| # | Test | Description | Priority |
|---|------|-------------|----------|
| F1 | Provider List fails | Cache build fails → error returned, no mutations | Required |
| F2 | Provider Create fails | Create error → reported in result, others continue | Required |
| F3 | Provider Delete fails | Delete error → reported in result, others continue | Required |
| F4 | All providers fail | All listers fail → error returned | Required |
| F5 | Partial failure | One provider fails, others succeed → partial success | Required |
| F6 | Context cancellation | Canceled context → clean return with error | Required |
| F7 | Error messages | Errors include provider name, hostname, operation | Required |

## Implementation Pattern

```go
package reconciler_test

import (
    "context"
    "testing"

    "github.com/maxfield-allison/dnsweaver/internal/reconciler"
    "github.com/maxfield-allison/dnsweaver/internal/testutil"
)

func TestReconcile_BasicCreateFlow(t *testing.T) {
    prov := testutil.NewMockProvider("test", "example.com.")
    src := testutil.NewMockSource()
    src.SetWorkloads(
        testutil.SimpleWorkload("web", "web.example.com", "192.0.2.1"),
    )

    r := newTestReconciler(t, prov, src)
    result, err := r.Reconcile(context.Background())
    testutil.RequireNoError(t, err)

    // Verify A record created
    testutil.AssertRecordExists(t, prov.Created(), "web.example.com", "A")
}
```

## Result Inspection

The `Result` struct provides detailed inspection of what happened:

```go
result, err := r.Reconcile(ctx)

// Check outcomes
result.HasErrors()         // Any errors occurred
result.Actions()           // All actions taken
result.Created()           // Records created
result.Deleted()           // Records deleted
result.Updated()           // Records updated
result.Skipped()           // Records skipped (no change needed)
result.Failed()            // Actions that failed
```

## Edge Case Priorities

Pay special attention to:

1. **Hostname normalization** — trailing dots, case folding
2. **RFC limits** — label length (63), total hostname (253)
3. **SRV records** — zero priority/weight are valid
4. **Duplicate handling** — duplicate hostnames across workloads, duplicate records from providers
