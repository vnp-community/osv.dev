# TASK-007: JIRA-Service — Thêm /integrations/jira Routes

> **Bug**: BUG-013  
> **Solution**: SOL-007  
> **Service**: `services/jira-service`  
> **Priority**: 🟡 MEDIUM  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành:**
- ✅ Thêm `PUT /jira/config` alias cho `CreateOrUpdateConfig` trong router.go
- ✅ Thêm v2 CRUD handlers: `ListConfigs`, `GetConfigByID`, `UpdateConfigByID`, `DeleteConfigByID`
- ✅ Thêm jira-issues stub handlers: `ListIssues`, `CreateIssue`, `GetIssueByFinding`, `DeleteIssue`
- ✅ Mở rộng `JiraConfigRepository` interface với `List()` và `DeleteByID()`
- ✅ Register tất cả routes v2 trong router.go trước wildcard `/{id}`
- ✅ Build `go build ./...` thành công


## Phân Tích Thực Tế

**Gateway đã có** (router.go):
```go
mux.Handle("GET /api/v1/jira/config",       adminOnly(proxy.Forward("jira-service:8088")))
mux.Handle("POST /api/v1/jira/config",      adminOnly(proxy.Forward("jira-service:8088")))
mux.Handle("PUT /api/v1/jira/config",       adminOnly(proxy.Forward("jira-service:8088")))
mux.Handle("POST /api/v1/jira/config/test", adminOnly(proxy.Forward("jira-service:8088")))
```

Cần kiểm tra jira-service có handler cho các routes này không.

## Việc Cần Làm

### Bước 1: Kiểm tra cấu trúc jira-service

```bash
find services/jira-service -name "*.go" | head -20
find services/jira-service -name "*.go" | xargs grep -l "handler\|Handler\|router\|Router" 2>/dev/null | head -10
```

### Bước 2: Kiểm tra routes đang registered

```bash
find services/jira-service -name "router.go" | xargs cat 2>/dev/null | head -80
# hoặc
find services/jira-service -name "*.go" | xargs grep -n "jira/config\|HandleFunc\|Handle\|Route" 2>/dev/null | head -20
```

### Bước 3: Xác nhận xem thiếu route nào

Nếu jira-service chưa có routes `/api/v1/jira/config`:

```go
// services/jira-service/internal/delivery/http/config_handler.go (tạo mới)

package http

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
)

type JiraConfigHandler struct {
    configRepo JiraConfigRepository
    encryptor  Encryptor
    httpClient *http.Client
    log        zerolog.Logger
}

// GET /api/v1/jira/config
func (h *JiraConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
    config, err := h.configRepo.GetGlobal(r.Context())
    if err != nil {
        if isNotFound(err) {
            // Not configured yet — return empty
            respondJSON(w, http.StatusOK, map[string]interface{}{
                "enabled": false,
            })
            return
        }
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "enabled":          config.Enabled,
        "server_url":       config.ServerURL,
        "project_key":      config.ProjectKey,
        "api_token":        "***masked***",
        "auto_create":      config.AutoCreate,
        "default_priority": config.DefaultPriority,
    })
}

// PUT /api/v1/jira/config
func (h *JiraConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ServerURL       string `json:"server_url"`
        APIToken        string `json:"api_token"`
        ProjectKey      string `json:"project_key"`
        AutoCreate      bool   `json:"auto_create"`
        DefaultPriority string `json:"default_priority"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    if req.ServerURL == "" || req.ProjectKey == "" {
        respondError(w, http.StatusBadRequest, "server_url and project_key required")
        return
    }

    // Encrypt token
    encrypted, err := h.encryptor.Encrypt([]byte(req.APIToken))
    if err != nil {
        respondError(w, http.StatusInternalServerError, "encryption failed")
        return
    }

    config, err := h.configRepo.Upsert(r.Context(), JiraConfig{
        ServerURL:       req.ServerURL,
        EncryptedToken:  encrypted,
        ProjectKey:      req.ProjectKey,
        AutoCreate:      req.AutoCreate,
        DefaultPriority: req.DefaultPriority,
        Enabled:         true,
    })
    if err != nil {
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    _ = config
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "success":    true,
        "server_url": req.ServerURL,
        "project_key": req.ProjectKey,
    })
}

// POST /api/v1/jira/config/test
func (h *JiraConfigHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ServerURL string `json:"server_url,omitempty"`
        APIToken  string `json:"api_token,omitempty"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    testURL := req.ServerURL
    testToken := req.APIToken

    // Use stored config if not provided in request
    if testToken == "" {
        config, err := h.configRepo.GetGlobal(r.Context())
        if err != nil {
            respondError(w, http.StatusBadRequest, "no JIRA config found")
            return
        }
        decrypted, err := h.encryptor.Decrypt(config.EncryptedToken)
        if err != nil {
            respondError(w, http.StatusInternalServerError, "decryption failed")
            return
        }
        testToken = string(decrypted)
        testURL = config.ServerURL
    }

    // Call JIRA serverInfo endpoint
    infoURL := strings.TrimSuffix(testURL, "/") + "/rest/api/2/serverInfo"
    req2, err := http.NewRequestWithContext(r.Context(), "GET", infoURL, nil)
    if err != nil {
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "success": false,
            "error":   "Failed to create request: " + err.Error(),
        })
        return
    }
    req2.Header.Set("Authorization", "Bearer "+testToken)
    req2.Header.Set("Accept", "application/json")

    resp, err := h.httpClient.Do(req2)
    if err != nil {
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "success": false,
            "error":   "Connection failed: " + err.Error(),
        })
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        respondJSON(w, http.StatusOK, map[string]interface{}{
            "success": false,
            "error":   fmt.Sprintf("JIRA returned HTTP %d", resp.StatusCode),
        })
        return
    }

    var info map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&info)

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "success":      true,
        "jira_version": info["version"],
        "server_title": info["serverTitle"],
    })
}
```

### Bước 4: Register routes trong jira-service router

```bash
# Tìm router hoặc main của jira-service
find services/jira-service -name "router.go" -o -name "main.go" | xargs grep -n "Handle\|Route\|GET\|POST\|PUT" 2>/dev/null | head -20
```

Thêm vào router:
```go
r.Get("/api/v1/jira/config",       configHandler.GetConfig)      // GET
r.Post("/api/v1/jira/config",      configHandler.UpdateConfig)   // POST (create)
r.Put("/api/v1/jira/config",       configHandler.UpdateConfig)   // PUT (update)
r.Post("/api/v1/jira/config/test", configHandler.TestConnection) // Test
```

### Bước 5: Build & Test

```bash
cd services/jira-service && go build ./...
```

**Test**:
```bash
TOKEN="your_admin_token"
BASE="https://c12.openledger.vn"

# Get config
curl -s "$BASE/api/v1/jira/config" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 OK {enabled: false} or {enabled: true, server_url: "..."}

# Update config
curl -s -X PUT "$BASE/api/v1/jira/config" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "server_url": "https://jira.example.com",
    "api_token": "test-token",
    "project_key": "SEC",
    "auto_create": true
  }' | jq .
# Expected: 200 OK {success: true}

# Test connection
curl -s -X POST "$BASE/api/v1/jira/config/test" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}' | jq .
# Expected: 200 OK {success: false, error: "..."}  (nếu không có JIRA server thực)
```

## Acceptance Criteria

- [x] `GET /api/v1/jira/config` → `200 OK`
- [x] `PUT /api/v1/jira/config` → `200 OK`
- [x] `POST /api/v1/jira/config/test` → `200 OK` (success: true hoặc false, không 404/405)
- [x] `go build ./...` không lỗi
- [x] API token không bao giờ được trả về plaintext
