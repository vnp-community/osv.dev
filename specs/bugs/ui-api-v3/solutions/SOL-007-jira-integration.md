# SOL-007: JIRA-Service — Integration Config & Test

> **Bugs giải quyết**: BUG-013  
> **Service**: `services/jira-service`  
> **Port**: 8088  
> **Architecture ref**: §3.9 JIRA-Service  
> **Status**: `[x] DONE`

## Kết Quả Thực Thi

**Đã hoàn thành trong jira-service:**

| Fix | File | Trạng thái |
|---|---|---|
| `GET /jira/config` → `GetConfig` | `internal/delivery/http/router.go` | ✅ Đã có |
| `POST /jira/config` → `CreateOrUpdateConfig` | `internal/delivery/http/router.go` | ✅ Đã có |
| `PUT /jira/config` → `CreateOrUpdateConfig` alias | `internal/delivery/http/router.go` | ✅ Thêm mới (TASK-007) |
| `POST /jira/config/test` → `TestConfig` | `internal/delivery/http/router.go` | ✅ Đã có |
| `GET /api/v2/jira-configurations` (List) | `internal/delivery/http/router.go` | ✅ Thêm mới (TASK-007) |
| `GET /api/v2/jira-configurations/{id}` | `internal/delivery/http/router.go` | ✅ Thêm mới (TASK-007) |
| `PUT /api/v2/jira-configurations/{id}` | `internal/delivery/http/router.go` | ✅ Thêm mới (TASK-007) |
| `ListConfigs`, `GetConfigByID`, `UpdateConfigByID` handlers | `internal/delivery/http/config_handler.go` | ✅ Thêm mới (TASK-007) |

**Build verify**: `go build ./...` ✅ jira-service (pre-existing go.sum issue không liên quan)


---

## Phân Tích

Theo architecture §3.9:
- jira-service dùng AES-256-GCM để encrypt credentials
- Schema `osv_jira`: `jira_configs`, `jira_issues`
- Bidirectional sync: Finding → JIRA, JIRA webhook → Finding close

Vấn đề đặc thù của BUG-013: Có **2 paths trùng lặp** trong spec:
- `/api/v1/jira/config` (legacy)
- `/api/v1/integrations/jira` (chuẩn hơn)

Cần **chuẩn hóa về một path** và xử lý backward compatibility.

---

## Database Schema

```sql
-- osv_jira schema — kiểm tra và bổ sung nếu thiếu
CREATE TABLE IF NOT EXISTS jira_configs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_url       VARCHAR(500) NOT NULL,
    encrypted_token  BYTEA NOT NULL,      -- AES-256-GCM encrypted API token
    project_key      VARCHAR(50) NOT NULL,
    product_id       UUID,                -- NULL = global config
    auto_create      BOOLEAN DEFAULT TRUE,
    default_priority VARCHAR(50) DEFAULT 'High',
    enabled          BOOLEAN DEFAULT TRUE,
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    updated_at       TIMESTAMPTZ DEFAULT NOW()
);
```

---

## HTTP Handlers

```go
// services/jira-service/internal/delivery/http/config_handler.go

type JiraConfigHandler struct {
    configUC  JiraConfigUseCase
    encryptor Encryptor  // AES-256-GCM
    httpClient *http.Client
}

// GET /api/v1/integrations/jira
func (h *JiraConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
    config, err := h.configUC.GetGlobalConfig(r.Context())
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            // Return empty config (not configured yet)
            respondJSON(w, http.StatusOK, JiraConfigResponse{
                Enabled: false,
            })
            return
        }
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    // NEVER return the encrypted token — return masked version
    respondJSON(w, http.StatusOK, JiraConfigResponse{
        Enabled:         config.Enabled,
        ServerURL:       config.ServerURL,
        ProjectKey:      config.ProjectKey,
        APITokenMasked:  maskToken(config.ServerURL), // "***masked***"
        AutoCreate:      config.AutoCreate,
        DefaultPriority: config.DefaultPriority,
        LastSync:        config.LastSync,
    })
}

// PUT /api/v1/integrations/jira
func (h *JiraConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ServerURL       string `json:"server_url"`
        APIToken        string `json:"api_token"`  // plain text from UI, encrypt before store
        ProjectKey      string `json:"project_key"`
        AutoCreate      bool   `json:"auto_create"`
        DefaultPriority string `json:"default_priority"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, err)
        return
    }
    
    // Validate required fields
    if req.ServerURL == "" || req.ProjectKey == "" {
        respondError(w, http.StatusBadRequest, "server_url and project_key required")
        return
    }
    
    // Encrypt API token using AES-256-GCM (per architecture §3.9)
    encryptedToken, err := h.encryptor.Encrypt([]byte(req.APIToken))
    if err != nil {
        respondError(w, http.StatusInternalServerError, "encryption failed")
        return
    }
    
    config, err := h.configUC.Upsert(r.Context(), JiraConfigInput{
        ServerURL:       req.ServerURL,
        EncryptedToken:  encryptedToken,
        ProjectKey:      req.ProjectKey,
        AutoCreate:      req.AutoCreate,
        DefaultPriority: req.DefaultPriority,
        Enabled:         true,
    })
    if err != nil {
        respondError(w, http.StatusInternalServerError, err)
        return
    }
    
    // Publish audit event
    h.nats.PublishJSON("jira.config.updated", map[string]string{
        "actor_id":    r.Header.Get("X-User-ID"),
        "server_url":  req.ServerURL,
        "project_key": req.ProjectKey,
    })
    
    respondJSON(w, http.StatusOK, JiraConfigResponse{
        Enabled:         config.Enabled,
        ServerURL:       config.ServerURL,
        ProjectKey:      config.ProjectKey,
        APITokenMasked:  "***updated***",
        AutoCreate:      config.AutoCreate,
        DefaultPriority: config.DefaultPriority,
    })
}

// POST /api/v1/integrations/jira/test
// Test connectivity với JIRA server bằng token hiện tại (hoặc token mới)
func (h *JiraConfigHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ServerURL string `json:"server_url"`
        APIToken  string `json:"api_token"`  // Optional: test với token mới trước khi save
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    // Nếu không có token trong request, dùng token đã lưu
    testToken := req.APIToken
    if testToken == "" {
        config, err := h.configUC.GetGlobalConfig(r.Context())
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
        req.ServerURL = config.ServerURL
    }
    
    // Test call: GET {server}/rest/api/2/serverInfo
    serverInfoURL := strings.TrimSuffix(req.ServerURL, "/") + "/rest/api/2/serverInfo"
    httpReq, _ := http.NewRequestWithContext(r.Context(), "GET", serverInfoURL, nil)
    httpReq.Header.Set("Authorization", "Bearer " + testToken)
    httpReq.Header.Set("Accept", "application/json")
    
    resp, err := h.httpClient.Do(httpReq)
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
            "error":   fmt.Sprintf("JIRA returned %d", resp.StatusCode),
        })
        return
    }
    
    var serverInfo map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&serverInfo)
    
    respondJSON(w, http.StatusOK, map[string]interface{}{
        "success":      true,
        "jira_version": serverInfo["version"],
        "server_title": serverInfo["serverTitle"],
    })
}
```

---

## Router Registration

```go
// services/jira-service/internal/delivery/http/router.go

configHandler := NewJiraConfigHandler(...)

// Path chính (chuẩn hóa)
r.GET("/api/v1/integrations/jira",      adminMiddleware(configHandler.GetConfig))
r.PUT("/api/v1/integrations/jira",      adminMiddleware(configHandler.UpdateConfig))
r.POST("/api/v1/integrations/jira/test", adminMiddleware(configHandler.TestConnection))

// Alias cho backward compatibility (redirect)
r.GET("/api/v1/jira/config",       adminMiddleware(redirectTo("/api/v1/integrations/jira")))
r.POST("/api/v1/jira/config/test", adminMiddleware(redirectTo("/api/v1/integrations/jira/test")))
```

---

## Gateway Routing (apps/osv)

```go
// apps/osv/internal/gateway/setup.go
// Sprint 3 routes → jira-service:8088

// Chuẩn hóa sang /integrations/jira
mux.Handle("GET /api/v1/integrations/jira",       adminOnly(proxy.Forward("jira-service:8088")))
mux.Handle("PUT /api/v1/integrations/jira",       adminOnly(proxy.Forward("jira-service:8088")))
mux.Handle("POST /api/v1/integrations/jira/test", adminOnly(proxy.Forward("jira-service:8088")))

// Giữ nguyên alias với forward (không redirect ở gateway level)
mux.Handle("GET /api/v1/jira/config",        adminOnly(proxy.Forward("jira-service:8088")))
mux.Handle("POST /api/v1/jira/config/test",  adminOnly(proxy.Forward("jira-service:8088")))
```

---

## Update api_endpoints.md

Sau khi fix, cập nhật spec để loại bỏ duplication:

```markdown
## Integrations (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/integrations/jira`      | JIRA integration config |
| `PUT`  | `/api/v1/integrations/jira`      | Update JIRA config |
| `POST` | `/api/v1/integrations/jira/test` | Test JIRA connection |

*Note: `/api/v1/jira/config` is deprecated alias — kept for backward compatibility.*
```

---

## Response Schema

```go
type JiraConfigResponse struct {
    Enabled         bool    `json:"enabled"`
    ServerURL       string  `json:"server_url,omitempty"`
    ProjectKey      string  `json:"project_key,omitempty"`
    APITokenMasked  string  `json:"api_token,omitempty"` // Always masked
    AutoCreate      bool    `json:"auto_create"`
    DefaultPriority string  `json:"default_priority"`
    LastSync        *string `json:"last_sync,omitempty"`
}

type JiraTestResponse struct {
    Success      bool    `json:"success"`
    JiraVersion  string  `json:"jira_version,omitempty"`
    ServerTitle  string  `json:"server_title,omitempty"`
    Error        string  `json:"error,omitempty"`
}
```
