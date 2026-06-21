# Observability Testing Template

Standardized test checklist for health endpoints, Prometheus metrics, and structured logging. Every deployable component must pass these baseline observability tests.

> **Philosophy:** If it runs, it should emit metrics. If it's important, it should have alerts. Tests must verify this is actually happening.

## Test Categories

### 1. Health Endpoints

| # | Test | Description | Priority |
|---|------|-------------|----------|
| H1 | Liveness probe | `/healthz` returns 200 when process is running | Required |
| H2 | Readiness probe | `/readyz` returns 200 when providers are reachable | Required |
| H3 | Readiness failure | `/readyz` returns 503 when a provider is down | Required |
| H4 | Startup probe | `/healthz` returns 200 after initial reconcile | Required |
| H5 | Response format | Health endpoints return JSON with component status | Recommended |

### 2. Prometheus Metrics

| # | Test | Description | Priority |
|---|------|-------------|----------|
| M1 | Reconcile counter | `dnsweaver_reconcile_total` increments on each cycle | Required |
| M2 | Error counter | `dnsweaver_reconcile_errors_total` increments on failure | Required |
| M3 | Duration histogram | `dnsweaver_reconcile_duration_seconds` observed each cycle | Required |
| M4 | Records created | `dnsweaver_records_created_total` tracks creates by provider | Required |
| M5 | Records deleted | `dnsweaver_records_deleted_total` tracks deletes by provider | Required |
| M6 | Records skipped | `dnsweaver_records_skipped_total` tracks no-ops | Recommended |
| M7 | Active hostnames | `dnsweaver_active_hostnames` gauge reflects current state | Recommended |
| M8 | Provider health | `dnsweaver_provider_healthy` gauge per provider | Recommended |
| M9 | Metrics endpoint | `/metrics` returns valid Prometheus text format | Required |
| M10 | Label cardinality | Metric labels are bounded (no unbounded hostname labels) | Required |

### 3. Structured Logging

| # | Test | Description | Priority |
|---|------|-------------|----------|
| G1 | JSON format | Log output is valid JSON (in production mode) | Required |
| G2 | Log levels | Debug, Info, Warn, Error levels used appropriately | Required |
| G3 | Context fields | Logs include provider, hostname, action where relevant | Required |
| G4 | No secrets | API keys, passwords, tokens never appear in logs | Required |
| G5 | Error context | Error logs include enough context to diagnose issues | Required |

### 4. Dry-Run Observability

| # | Test | Description | Priority |
|---|------|-------------|----------|
| R1 | Action fields populated | Dry-run results include Provider, Hostname, Type, Target | Required |
| R2 | No mutations | Dry-run does not create/delete/update any real records | Required |
| R3 | Orphan reporting | Dry-run reports planned orphan cleanups | Required |
| R4 | Target change | Dry-run detects and reports target IP changes | Required |
| R5 | Runtime toggle | `SetDryRun(true/false)` takes effect on next reconcile | Required |

## Implementation Pattern

### Metric Testing with `prometheus/testutil`

```go
package reconciler_test

import (
    "testing"

    "github.com/prometheus/client_golang/prometheus/testutil"

    "github.com/maxfield-allison/dnsweaver/internal/metrics"
)

func TestMetrics_ReconcileCounter(t *testing.T) {
    // Reset metrics before test
    metrics.ReconcileTotal.Reset()

    // ... run reconcile ...

    count := testutil.ToFloat64(metrics.ReconcileTotal)
    if count != 1 {
        t.Errorf("ReconcileTotal = %v, want 1", count)
    }
}

func TestMetrics_DurationObserved(t *testing.T) {
    metrics.ReconcileDuration.Reset()

    // ... run reconcile ...

    count := testutil.CollectAndCount(metrics.ReconcileDuration)
    if count == 0 {
        t.Error("ReconcileDuration not observed")
    }
}
```

### Health Endpoint Testing

```go
func TestHealthz(t *testing.T) {
    srv := setupTestServer(t)
    resp, err := http.Get(srv.URL + "/healthz")
    if err != nil {
        t.Fatalf("GET /healthz: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        t.Errorf("GET /healthz = %d, want 200", resp.StatusCode)
    }
}
```

### Dry-Run Result Inspection

```go
func TestDryRun_ActionFieldsPopulated(t *testing.T) {
    r := newTestReconciler(t, prov, src)
    r.SetDryRun(true)

    result, err := r.Reconcile(context.Background())
    testutil.RequireNoError(t, err)

    for _, action := range result.Actions() {
        if action.Provider == "" {
            t.Error("dry-run action missing Provider")
        }
        if action.Hostname == "" {
            t.Error("dry-run action missing Hostname")
        }
    }
}
```

## Metric Naming Conventions

All dnsweaver metrics follow Prometheus naming conventions:

| Pattern | Example | Type |
|---------|---------|------|
| `dnsweaver_<noun>_total` | `dnsweaver_reconcile_total` | Counter |
| `dnsweaver_<noun>_<unit>` | `dnsweaver_reconcile_duration_seconds` | Histogram |
| `dnsweaver_<noun>` | `dnsweaver_active_hostnames` | Gauge |

Labels should be bounded:
- `provider` — instance name (bounded by config)
- `action` — create, delete, update, skip (bounded enum)
- `status` — success, error (bounded enum)
- **Never** use hostnames as label values (unbounded cardinality)
