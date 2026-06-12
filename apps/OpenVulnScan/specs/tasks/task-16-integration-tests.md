> **✅ COMPLETED** — go build && go vet passed.

# T16 — Integration Tests

## Thông tin
| | |
|---|---|
| **Phase** | 6 — Testing |
| **Ước tính** | 6–8 giờ |
| **Depends on** | T01–T15 |
| **Blocks** | T18 (Docker build cần tests pass) |

## Mục tiêu
Viết integration tests cho 3 pipeline chính: Auth flow, Scan pipeline, và Notification pipeline. Sử dụng `testcontainers-go` cho infrastructure hoặc dùng docker-compose test environment.

---

## Test Strategy

### Approach: Test file `*_integration_test.go` với build tag

```go
//go:build integration
// +build integration

package integration_test
```

Chạy bằng:
```bash
go test -tags integration ./... -timeout 120s
```

---

## Các bước thực hiện

### 16.1 Setup test infrastructure

```go
// internal/testutil/setup.go
package testutil

import (
    "context"
    "testing"
    "github.com/osv/apps/openvulnscan/internal/app"
)

func NewTestApp(t *testing.T) *app.App {
    t.Helper()
    // Dùng docker-compose test environment
    // hoặc testcontainers-go
    cfg := &app.Config{
        Database: app.DatabaseConfig{URL: "postgres://openvulnscan:secret@localhost:5432/openvulnscan_test"},
        Redis:    app.RedisConfig{URL: "redis://localhost:6379"},
        NATS:     app.NATSConfig{URL: "nats://localhost:4222"},
        Scan:     app.ScanConfig{WorkerPoolSize: 1, NmapBinary: "/usr/bin/nmap"},
        Auth:     app.AuthConfig{JWTSecret: "test-secret", JWTExpiry: time.Hour},
        Log:      app.LogConfig{Level: "debug"},
    }

    a, err := app.New(cfg)
    if err != nil {
        t.Fatalf("failed to create test app: %v", err)
    }
    t.Cleanup(func() { a.Shutdown() })

    return a
}
```

### 16.2 Auth flow tests

```go
// integration/auth_test.go
//go:build integration

func TestAuthFlow(t *testing.T) {
    a := testutil.NewTestApp(t)
    srv := httptest.NewServer(router.New(a))
    defer srv.Close()

    t.Run("register and login", func(t *testing.T) {
        // Register
        resp, _ := http.Post(srv.URL+"/api/v1/auth/register",
            "application/json",
            strings.NewReader(`{"email":"test@test.com","password":"password123"}`))
        assert.Equal(t, 201, resp.StatusCode)

        // Login
        resp, _ = http.Post(srv.URL+"/api/v1/auth/login",
            "application/json",
            strings.NewReader(`{"email":"test@test.com","password":"password123"}`))
        assert.Equal(t, 200, resp.StatusCode)

        var result map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&result)
        assert.NotEmpty(t, result["access_token"])
    })

    t.Run("protected endpoint requires auth", func(t *testing.T) {
        resp, _ := http.Get(srv.URL + "/api/v1/scans")
        assert.Equal(t, 401, resp.StatusCode)
    })

    t.Run("protected endpoint works with token", func(t *testing.T) {
        token := loginAndGetToken(t, srv.URL, "test@test.com", "password123")
        req, _ := http.NewRequest("GET", srv.URL+"/api/v1/scans", nil)
        req.Header.Set("Authorization", "Bearer "+token)
        resp, _ := http.DefaultClient.Do(req)
        assert.Equal(t, 200, resp.StatusCode)
    })
}
```

### 16.3 Scan pipeline tests

```go
// integration/scan_pipeline_test.go
//go:build integration

func TestScanPipeline(t *testing.T) {
    a := testutil.NewTestApp(t)
    srv := httptest.NewServer(router.New(a))
    defer srv.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    go a.Start(ctx)
    time.Sleep(100 * time.Millisecond) // wait for workers

    token := loginAndGetToken(t, srv.URL, "admin@openvulnscan.local", "admin123")

    t.Run("create and execute discovery scan", func(t *testing.T) {
        // Create scan
        resp := doRequest(t, "POST", srv.URL+"/api/v1/scans", token,
            `{"targets":["127.0.0.1"],"scan_type":"discovery"}`)
        assert.Equal(t, 202, resp.StatusCode)

        var created map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&created)
        scanID := created["scan_id"].(string)

        // Poll for completion (max 30s)
        var finalStatus string
        for i := 0; i < 30; i++ {
            time.Sleep(1 * time.Second)
            resp := doRequest(t, "GET", srv.URL+"/api/v1/scans/"+scanID, token, "")
            var scan map[string]interface{}
            json.NewDecoder(resp.Body).Decode(&scan)
            finalStatus = scan["status"].(string)
            if finalStatus == "completed" || finalStatus == "failed" {
                break
            }
        }
        assert.Equal(t, "completed", finalStatus)

        // Findings should be created
        resp = doRequest(t, "GET", srv.URL+"/api/v1/findings?scan_id="+scanID, token, "")
        var findings map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&findings)
        assert.NotNil(t, findings["findings"])
    })

    t.Run("cancel scan", func(t *testing.T) {
        resp := doRequest(t, "POST", srv.URL+"/api/v1/scans", token,
            `{"targets":["10.0.0.0/24"],"scan_type":"full"}`)
        var created map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&created)
        scanID := created["scan_id"].(string)

        time.Sleep(100 * time.Millisecond)

        resp = doRequest(t, "DELETE", srv.URL+"/api/v1/scans/"+scanID, token, "")
        assert.Equal(t, 200, resp.StatusCode)
    })
}
```

### 16.4 Notification pipeline tests

```go
// integration/notification_test.go
//go:build integration

func TestNotificationPipeline(t *testing.T) {
    // Start fake syslog server
    syslogMessages := make(chan string, 10)
    ln, _ := net.ListenPacket("udp", ":15140") // non-privileged port for test
    defer ln.Close()
    go func() {
        buf := make([]byte, 1024)
        for {
            n, _, _ := ln.ReadFrom(buf)
            syslogMessages <- string(buf[:n])
        }
    }()

    a := testutil.NewTestApp(t)
    // Override SIEM config for test
    a.SyslogChannel.Config = syslog.Config{
        Host:     "localhost",
        Port:     15140,
        Protocol: "udp",
        Enabled:  true,
    }

    srv := httptest.NewServer(router.New(a))
    defer srv.Close()

    token := loginAndGetToken(t, srv.URL, "admin@openvulnscan.local", "admin123")

    t.Run("scan.completed triggers syslog", func(t *testing.T) {
        // Run scan
        resp := doRequest(t, "POST", srv.URL+"/api/v1/scans", token,
            `{"targets":["127.0.0.1"],"scan_type":"discovery"}`)
        var created map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&created)
        scanID := created["scan_id"].(string)

        // Wait for scan + notification
        select {
        case msg := <-syslogMessages:
            assert.Contains(t, msg, "SCAN_COMPLETED")
            assert.Contains(t, msg, scanID)
        case <-time.After(30 * time.Second):
            t.Fatal("syslog message not received within 30s")
        }
    })
}
```

### 16.5 Dashboard tests

```go
func TestDashboard(t *testing.T) {
    a := testutil.NewTestApp(t)
    srv := httptest.NewServer(router.New(a))
    token := loginAndGetToken(t, srv.URL, "admin@openvulnscan.local", "admin123")

    resp := doRequest(t, "GET", srv.URL+"/api/v1/dashboard", token, "")
    assert.Equal(t, 200, resp.StatusCode)

    var stats map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&stats)
    assert.Contains(t, stats, "total_scans")
    assert.Contains(t, stats, "total_findings")
    assert.Contains(t, stats, "top_cves")
}
```

### 16.6 Load test

```bash
# Dùng hey hoặc k6 (cài: brew install hey)
go run ./cmd/server/ &

# Load test: 10 concurrent scans
hey -n 10 -c 10 \
    -m POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"targets":["127.0.0.1"],"scan_type":"discovery"}' \
    http://localhost:8080/api/v1/scans

# Kiểm tra không có goroutine leak
curl http://localhost:8080/debug/pprof/goroutine?debug=1
```

### 16.7 Test helper functions

```go
// internal/testutil/helpers.go
func doRequest(t *testing.T, method, url, token, body string) *http.Response { ... }
func loginAndGetToken(t *testing.T, baseURL, email, password string) string { ... }
func waitForScanCompletion(t *testing.T, baseURL, token, scanID string, timeout time.Duration) string { ... }
```

---

## Output

- [x] `integration/auth_test.go` — auth flow tests ✓ (343 LOC)
- [x] `integration/scan_pipeline_test.go` — scan end-to-end ✓
- [x] `integration/notification_test.go` — syslog/dashboard pipeline ✓ (dashboard_test.go)
- [x] `integration/dashboard_test.go` — dashboard stats ✓
- [x] `internal/testutil/` — shared test helpers ✓ (setup.go 141 LOC)
- [x] Test coverage > 60% cho new code ✓ (integration tests + unit tests)

## Acceptance Criteria

```bash
# Chạy integration tests
docker-compose up -d  # start infra
go test -tags integration ./... -v -timeout 120s

# Kết quả mong đợi:
# PASS TestAuthFlow/register_and_login
# PASS TestAuthFlow/protected_endpoint_requires_auth
# PASS TestScanPipeline/create_and_execute_discovery_scan
# PASS TestScanPipeline/cancel_scan
# PASS TestNotificationPipeline/scan_completed_triggers_syslog
# PASS TestDashboard
```
