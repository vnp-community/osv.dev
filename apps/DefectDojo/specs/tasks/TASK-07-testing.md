# TASK-07: Testing

**Phase**: 7 — Quality Assurance  
**Ước tính**: 16 giờ  
**Phụ thuộc**: TASK-04, TASK-05, TASK-06  
**Output**: Unit tests, integration tests, E2E tests, test helpers

---

## Mục tiêu

Đảm bảo chất lượng qua 3 tầng test:
- **Unit tests**: Test từng runner/handler/component riêng lẻ
- **Integration tests**: Test interaction giữa services (in-process)
- **E2E tests**: Test full API flows (DefectDojo API v2 compatibility)

---

## T-07.1: Test Infrastructure Setup

**File**: `apps/DefectDojo/internal/testutil/helpers.go`  
**Ước tính**: 2h

```go
// Package testutil provides test helpers for the DefectDojo monolith.
package testutil

import (
    "context"
    "testing"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/nats-io/nats.go"
    "github.com/nats-io/nats-server/v2/server"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc/test/bufconn"
)

// TestDB creates an in-memory test database (or connects to test PG).
func TestDB(t *testing.T) *pgxpool.Pool {
    t.Helper()
    url := testenv("TEST_POSTGRES_URL", "postgres://defectdojo:test@localhost:5432/defectdojo_test?sslmode=disable")
    pool, err := pgxpool.New(context.Background(), url)
    require.NoError(t, err)
    t.Cleanup(func() { pool.Close() })
    return pool
}

// TestNATS starts an embedded NATS server for testing.
func TestNATS(t *testing.T) *nats.Conn {
    t.Helper()
    opts := server.Options{
        Port:      -1,     // Random port
        JetStream: true,
        StoreDir:  t.TempDir(),
    }
    s, err := server.NewServer(&opts)
    require.NoError(t, err)
    go s.Start()
    require.True(t, s.ReadyForConnections(2*time.Second))
    t.Cleanup(s.Shutdown)

    nc, err := nats.Connect(s.ClientURL())
    require.NoError(t, err)
    t.Cleanup(nc.Close)
    return nc
}

// TestRedis starts a miniredis for testing.
func TestRedis(t *testing.T) *redis.Client {
    t.Helper()
    // Use miniredis or test Redis
    return redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
}

// TestBufConn creates a bufconn listener for gRPC testing.
func TestBufConn() *bufconn.Listener {
    return bufconn.Listen(1 << 20)
}

// WaitForRunner waits until a runner's Health() returns nil.
func WaitForRunner(t *testing.T, ctx context.Context, runner interface {
    Health(context.Context) error
}, timeout time.Duration) {
    t.Helper()
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if err := runner.Health(ctx); err == nil {
            return
        }
        time.Sleep(50 * time.Millisecond)
    }
    t.Fatal("runner did not become healthy within timeout")
}

func testenv(key, defaultVal string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultVal
}
```

**Tasks**:
- [ ] TestDB với cleanup
- [ ] Embedded NATS với JetStream
- [ ] TestBufConn helper
- [ ] WaitForRunner polling helper
- [ ] `go build ./internal/testutil/...` pass

---

## T-07.2: Registry Unit Tests

**File**: `apps/DefectDojo/internal/app/registry/registry_test.go`  
**Ước tính**: 1.5h

### Test cases:
- [ ] `TestRegistry_RegisterAndStart` — Register 3 runners, start, verify all StateRunning
- [ ] `TestRegistry_StopSpecific` — Stop one runner, others still running
- [ ] `TestRegistry_StopAll` — All runners stop gracefully
- [ ] `TestRegistry_PanicRecovery` — Panicking runner → StateFailed, others unaffected
- [ ] `TestRegistry_ContextCancel` — Parent ctx cancel → all runners stop
- [ ] `TestRegistry_HealthAll` — Mix of healthy/unhealthy runners

```go
func TestRegistry_PanicRecovery(t *testing.T) {
    reg := registry.New()
    panicRunner := &mockPanicRunner{name: "panic-svc"}
    normalRunner := &mockNormalRunner{name: "normal-svc"}

    reg.Register(panicRunner)
    reg.Register(normalRunner)

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    reg.Start(ctx)
    time.Sleep(200 * time.Millisecond)

    statuses := reg.Status()
    assert.Equal(t, registry.StateFailed, statuses["panic-svc"])
    assert.Equal(t, registry.StateRunning, statuses["normal-svc"])
}
```

---

## T-07.3: Config Unit Tests

**File**: `apps/DefectDojo/internal/config/config_test.go`  
**Ước tính**: 0.5h

- [ ] `TestLoad_Success` — All required vars set
- [ ] `TestLoad_MissingPostgresURL` — Returns error
- [ ] `TestLoad_MissingJWTSecret` — Returns error
- [ ] `TestLoad_JWTSecretTooShort` — Returns error (< 32 chars)
- [ ] `TestLoad_Defaults` — HTTP_PORT=8080, NATS_URL=nats://localhost:4222

---

## T-07.4: Events Unit Tests

**File**: `apps/DefectDojo/internal/events/publisher_test.go`  
**Ước tính**: 1h

- [ ] `TestPublisher_FindingCreated_PublishesToTwoSubjects` — Critical finding → both SubjFindingCreated and SubjFindingCritical
- [ ] `TestPublisher_FindingCreated_NonCritical_OneSubject` — Medium finding → only SubjFindingCreated
- [ ] `TestConsumer_Subscribe_DeliveryAndAck` — Publish event, consumer receives and processes
- [ ] `TestConsumer_NakOnError` — Handler returns error → message is redelivered

---

## T-07.5: Gateway Handler Unit Tests

**File**: `apps/DefectDojo/internal/gateway/handlers/finding_test.go`  
**Ước tính**: 2h

```go
func TestFindingHandler_List_Filtering(t *testing.T) {
    mockClient := &mockFindingServiceClient{}
    handler := &FindingHandler{client: mockClient}

    req := httptest.NewRequest("GET", "/api/v2/findings/?severity=Critical&active=true&limit=50", nil)
    req = req.WithContext(context.WithValue(req.Context(), claimsKey, &UserClaims{UserID: "user1"}))
    rr := httptest.NewRecorder()

    handler.List(rr, req)

    assert.Equal(t, http.StatusOK, rr.Code)

    var resp DDPaginatedResponse
    json.Unmarshal(rr.Body.Bytes(), &resp)
    assert.Greater(t, resp.Count, 0)
    assert.NotNil(t, resp.Results)

    // Verify mock was called with correct params
    call := mockClient.lastListCall
    assert.Equal(t, []string{"Critical"}, call.Severity)
    assert.True(t, call.ActiveOnly)
    assert.Equal(t, int32(50), call.Limit)
}
```

**Test cases cho finding handlers**:
- [ ] List findings với filters
- [ ] Get finding by ID (found/not found)
- [ ] Create finding
- [ ] Bulk update
- [ ] Close finding (state transition)
- [ ] Notes CRUD
- [ ] Pagination (next/previous URLs)
- [ ] 401 when no auth header
- [ ] 403 when insufficient permissions

---

## T-07.6: Auth Middleware Unit Tests

**File**: `apps/DefectDojo/internal/gateway/middleware_test.go`  
**Ước tính**: 1h

- [ ] `TestAuthMiddleware_ValidJWT` — Passes through
- [ ] `TestAuthMiddleware_ValidAPIKey` — Passes through (ovs_ prefix)
- [ ] `TestAuthMiddleware_NoHeader` — Returns 401
- [ ] `TestAuthMiddleware_ExpiredJWT` — Returns 401
- [ ] `TestAuthMiddleware_InvalidAPIKey` — Returns 401
- [ ] `TestAuthMiddleware_ClaimsInContext` — UserID available downstream

---

## T-07.7: Integration Tests — Scan Import Flow

**File**: `apps/DefectDojo/tests/integration/scan_import_test.go`  
**Tag**: `//go:build integration`  
**Ước tính**: 3h

```go
//go:build integration

package integration

import (
    "testing"

    "github.com/defectdojo/apps/defectdojo/internal/testutil"
)

// TestScanImportFlow tests the full scan import pipeline:
// POST /import-scan-results → scan-service → finding-service → notifications
func TestScanImportFlow(t *testing.T) {
    // Setup full app in test mode
    app := testutil.NewTestApp(t)
    app.Start(t)

    client := testutil.NewAPIClient(t, app.BaseURL())

    // 1. Create product
    product := client.MustCreateProduct(t, "Test Product", "test-product-type-id")

    // 2. Create engagement
    engagement := client.MustCreateEngagement(t, product.ID, "Test Engagement")

    // 3. Import Trivy scan
    scanFile := loadTestFixture(t, "trivy_scan.json")
    importResp := client.MustImportScan(t, "Trivy Scan", scanFile, engagement.ID)

    // 4. Wait for processing
    testutil.WaitFor(t, 10*time.Second, func() bool {
        findings := client.MustListFindings(t, map[string]string{"engagement": engagement.ID})
        return findings.Count > 0
    })

    // 5. Verify findings created
    findings := client.MustListFindings(t, map[string]string{"engagement": engagement.ID})
    assert.Greater(t, findings.Count, 0)

    // 6. Verify notification was sent (check mock notification channel)
    assert.True(t, app.NotificationsSent() > 0)

    // 7. Verify SLA dates set
    for _, f := range findings.Results {
        if f.Severity == "Critical" {
            assert.NotNil(t, f.SLAExpirationDate)
        }
    }
}
```

**Integration test cases**:
- [ ] Scan import flow (Trivy) → findings created → SLA set → notification sent
- [ ] Reimport: existing findings deduped, closed findings reopened
- [ ] Close finding → notification triggered → JIRA updated (mock)
- [ ] SLA breach: advance time → SLA checker fires → alert sent
- [ ] Risk acceptance: finding marked accepted → excluded from active
- [ ] Report generation: request → stream findings → PDF rendered

---

## T-07.8: E2E Tests — DefectDojo Compatibility

**File**: `apps/DefectDojo/tests/e2e/compatibility_test.go`  
**Tag**: `//go:build e2e`  
**Ước tính**: 2h

```bash
# Test using DefectDojo Python client
# Verifies API is 100% compatible

# Run against local app
go run ./cmd/defectdojo/ &
sleep 5

python3 tests/e2e/test_dd_compat.py http://localhost:8080
```

**Python test script** (`tests/e2e/test_dd_compat.py`):
```python
#!/usr/bin/env python3
"""Test DefectDojo API compatibility using official Python client."""

import sys
import requests

BASE = sys.argv[1]

def test_auth():
    r = requests.post(f"{BASE}/api/v2/api-token-auth/", json={
        "username": "admin", "password": "admin"
    })
    assert r.status_code == 200, f"Auth failed: {r.text}"
    return r.json()["token"]

def test_pagination(token):
    r = requests.get(f"{BASE}/api/v2/findings/?limit=10&offset=0", headers={
        "Authorization": f"Token {token}"
    })
    assert r.status_code == 200
    data = r.json()
    assert "count" in data
    assert "next" in data
    assert "previous" in data
    assert "results" in data
    print("✅ Pagination format correct")

def main():
    token = test_auth()
    print("✅ Authentication working")
    test_pagination(token)
    print("All E2E tests passed!")

main()
```

**E2E test cases**:
- [ ] Authentication endpoints (login, refresh, logout)
- [ ] Pagination format (count, next, previous, results)
- [ ] Filtering (severity, active, product, engagement)
- [ ] Date format (ISO 8601)
- [ ] Error format `{"detail": "..."}` 
- [ ] Import scan với real scan files (Trivy, Semgrep, Snyk)
- [ ] Report download returns valid content

---

## T-07.9: Test Fixtures

**Directory**: `apps/DefectDojo/tests/fixtures/`  
**Ước tính**: 1h

```bash
mkdir -p tests/fixtures/scans
# Copy sample scan files từ django-DefectDojo test fixtures
cp /Users/binhnt/Lab/sec/cve/django-DefectDojo/unittests/scans/trivy/*.json \
   tests/fixtures/scans/
cp /Users/binhnt/Lab/sec/cve/django-DefectDojo/unittests/scans/semgrep/*.json \
   tests/fixtures/scans/
cp /Users/binhnt/Lab/sec/cve/django-DefectDojo/unittests/scans/snyk/*.json \
   tests/fixtures/scans/
```

- [ ] Trivy scan fixtures (JSON)
- [ ] Semgrep scan fixtures (JSON)
- [ ] Snyk scan fixtures (JSON)
- [ ] SARIF fixtures
- [ ] CycloneDX SBOM fixtures

---

## Definition of Done — TASK-07

- [ ] T-07.1 Test helpers (DB, NATS, bufconn)
- [ ] T-07.2 Registry unit tests (100% coverage)
- [ ] T-07.3 Config unit tests
- [ ] T-07.4 Events unit tests (publisher + consumer)
- [ ] T-07.5 Gateway handler unit tests
- [ ] T-07.6 Auth middleware unit tests
- [ ] T-07.7 Integration tests (scan import flow) — requires Docker infra
- [ ] T-07.8 E2E compatibility tests
- [ ] T-07.9 Test fixtures from DefectDojo
- [ ] `go test ./... -timeout 60s` pass (unit tests)
- [ ] `go test ./tests/integration/... -tags=integration` pass
- [ ] Coverage ≥ 70% cho internal/app/registry, internal/events, internal/gateway
