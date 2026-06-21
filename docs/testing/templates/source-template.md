# Source Testing Template

Standardized test checklist for workload discovery sources. Every source must pass these baseline tests before shipping.

> **Philosophy:** Sources translate infrastructure state into DNS intent. Tests must verify that translation is accurate, complete, and resilient to real-world variability.

## Test Categories

### 1. Discovery

| # | Test | Description | Priority |
|---|------|-------------|----------|
| D1 | List workloads | `List()` returns all matching containers/services | Required |
| D2 | Empty result | `List()` returns empty slice (not nil/error) when nothing matches | Required |
| D3 | Filter by label | Only containers with expected labels are returned | Required |
| D4 | Multiple workloads | Multiple matching containers produce correct hostname list | Required |
| D5 | Workload metadata | Returned workloads include name, hostnames, record hints | Required |

### 2. Label Parsing

| # | Test | Description | Priority |
|---|------|-------------|----------|
| L1 | Traefik `Host()` rule | Parse `Host(\`app.example.com\`)` correctly | Required |
| L2 | Multiple hosts | Parse `Host(\`a.com\`) \|\| Host(\`b.com\`)` → two hostnames | Required |
| L3 | Native labels | Parse `dnsweaver.hostname=app.example.com` | Required |
| L4 | Record type hints | Parse `dnsweaver.type=CNAME` or `dnsweaver.type=A` | Required |
| L5 | Target override | Parse `dnsweaver.target=192.0.2.1` override | Recommended |
| L6 | SRV hints | Parse SRV-specific labels (port, weight, priority) | Recommended |
| L7 | Missing labels | Container without DNS labels → excluded from results | Required |
| L8 | Malformed labels | Invalid label syntax → skip with warning, don't error | Required |

### 3. Container/Service Lifecycle

| # | Test | Description | Priority |
|---|------|-------------|----------|
| S1 | Running container | Running container is discovered | Required |
| S2 | Stopped container | Stopped/exited container is excluded | Required |
| S3 | Paused container | Paused container handling (include or exclude per config) | Recommended |
| S4 | Replicated service | All replicas contribute same hostname | Recommended |
| S5 | Service with no tasks | Service with 0 running tasks → excluded | Recommended |

### 4. Watch Mode

| # | Test | Description | Priority |
|---|------|-------------|----------|
| W1 | Event detection | Source detects container start/stop events | Required |
| W2 | Event channel | Events are delivered to the watcher channel | Required |
| W3 | Context cancellation | Watch stops cleanly on context cancel | Required |
| W4 | Reconnection | Watch recovers after API disconnect | Recommended |

### 5. Error Handling

| # | Test | Description | Priority |
|---|------|-------------|----------|
| E1 | API unreachable | `List()` returns error, not panic | Required |
| E2 | Auth failure | Clear error for bad credentials | Required |
| E3 | Partial failure | Some containers fail to inspect → others still returned | Recommended |
| E4 | Context timeout | Operations respect context timeout | Required |

## Implementation Pattern

```go
package mysource_test

import (
    "context"
    "testing"

    "github.com/maxfield-allison/dnsweaver/internal/testutil"
)

func TestList_RunningContainers(t *testing.T) {
    // Create mock workloads
    workloads := []testutil.MockWorkload{
        testutil.DockerWorkload("web", "running",
            testutil.WithTraefikHost("web.example.com"),
        ),
        testutil.DockerWorkload("api", "running",
            testutil.WithNativeHostname("api.example.com"),
        ),
    }

    src := newTestSource(t, workloads)
    ctx := context.Background()

    result, err := src.List(ctx)
    testutil.RequireNoError(t, err)
    testutil.AssertLen(t, result, 2)
}
```

## Hostname Extraction Testing

Sources must correctly extract hostnames from various label formats. Test all supported formats:

```go
func TestHostnameExtraction(t *testing.T) {
    tests := []struct {
        name     string
        labels   map[string]string
        wantHost []string
    }{
        {
            name:     "traefik host rule",
            labels:   map[string]string{"traefik.http.routers.app.rule": "Host(`app.example.com`)"},
            wantHost: []string{"app.example.com"},
        },
        {
            name:     "native label",
            labels:   map[string]string{"dnsweaver.hostname": "app.example.com"},
            wantHost: []string{"app.example.com"},
        },
        {
            name:     "no dns labels",
            labels:   map[string]string{"unrelated": "value"},
            wantHost: nil,
        },
    }
    // ... run table-driven tests
}
```
