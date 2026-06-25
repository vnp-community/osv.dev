# SOL-V6-006: Fix OAuth Configuration & Report Download

**Bugs:** BUG-V6-024, BUG-V6-025, BUG-V6-026 (OAuth), BUG-V6-027 (Scan Import), BUG-V6-028 (Report Download)  
**Tasks:** TASK-V6-006, TASK-V6-007  
**Services:** `auth-service` / `identity-service`, `scan-service`, `finding-service`  
**Kiến trúc tham chiếu:** `02-technical-design.md §14.1` (auth-service OAuth2), `§6` (scan import pipeline), `§5` (report service)

---

## Part A: OAuth Configuration (BUG-V6-024, 025, 026)

### Root Cause

Theo `02-technical-design.md §14.1`, auth-service hỗ trợ OAuth2 flow:
```go
// Login flow hỗ trợ: Local, LDAP, và OAuth2 (Google, GitHub)
// GET /auth/oauth/{provider} → redirect to provider authorization URL
// GET /auth/callback         → exchange code → issue JWT
```

Lỗi 400 thay vì 302 nghĩa là OAuth provider credentials (`CLIENT_ID`, `CLIENT_SECRET`) chưa được cấu hình trong server environment.

### Fix 1: Environment Configuration

**File:** `deploy/dev/configs/auth-service.yaml` (hoặc identity-service config)

```yaml
oauth:
  # Google OAuth2
  google:
    enabled: false          # Set true khi có credentials
    client_id: ""           # GOOGLE_CLIENT_ID
    client_secret: ""       # GOOGLE_CLIENT_SECRET
    redirect_url: "https://c12.openledger.vn/api/v1/auth/callback?provider=google"
    scopes:
      - "email"
      - "profile"
  
  # GitHub OAuth2
  github:
    enabled: false          # Set true khi có credentials
    client_id: ""           # GITHUB_CLIENT_ID
    client_secret: ""       # GITHUB_CLIENT_SECRET
    redirect_url: "https://c12.openledger.vn/api/v1/auth/callback?provider=github"
    scopes:
      - "user:email"
      - "read:user"
```

**.env additions:**
```bash
GOOGLE_OAUTH_ENABLED=false
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

GITHUB_OAUTH_ENABLED=false
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
```

### Fix 2: Graceful Error khi OAuth chưa cấu hình

**Vấn đề:** Server trả 400 không có body → FE không biết lý do.  
**Fix:** Trả về JSON error rõ ràng.

```go
// services/identity-service/internal/delivery/http/oauth_handler.go

type OAuthConfig struct {
    Google OAuthProviderConfig `env:",prefix=GOOGLE_"`
    GitHub OAuthProviderConfig `env:",prefix=GITHUB_"`
}

type OAuthProviderConfig struct {
    Enabled      bool   `env:"OAUTH_ENABLED"`
    ClientID     string `env:"CLIENT_ID"`
    ClientSecret string `env:"CLIENT_SECRET"`
    RedirectURL  string `env:"REDIRECT_URL"`
}

// GET /auth/oauth/google
func (h *OAuthHandler) InitiateGoogle(w http.ResponseWriter, r *http.Request) {
    if !h.cfg.Google.Enabled || h.cfg.Google.ClientID == "" {
        // Trả về error rõ ràng thay vì 400 không có body
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "error":   "OAUTH_NOT_CONFIGURED",
            "message": "Google OAuth is not configured on this server",
            "hint":    "Contact your administrator to enable Google OAuth",
        })
        return
    }

    // Generate state token (CSRF protection)
    state := generateSecureState()
    h.stateStore.Set(r.Context(), state, 10*time.Minute)

    // Build authorization URL và redirect
    authURL := h.googleConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
    http.Redirect(w, r, authURL, http.StatusFound)  // 302
}

// GET /auth/oauth/github
func (h *OAuthHandler) InitiateGitHub(w http.ResponseWriter, r *http.Request) {
    if !h.cfg.GitHub.Enabled || h.cfg.GitHub.ClientID == "" {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "error":   "OAUTH_NOT_CONFIGURED",
            "message": "GitHub OAuth is not configured on this server",
        })
        return
    }

    state := generateSecureState()
    h.stateStore.Set(r.Context(), state, 10*time.Minute)

    authURL := h.githubConfig.AuthCodeURL(state)
    http.Redirect(w, r, authURL, http.StatusFound)  // 302
}

// GET /auth/callback?provider=google&code=...&state=...
func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
    provider := r.URL.Query().Get("provider")
    code := r.URL.Query().Get("code")
    state := r.URL.Query().Get("state")

    // Validate state (CSRF)
    if !h.stateStore.Validate(r.Context(), state) {
        writeError(w, http.StatusBadRequest, "invalid or expired OAuth state")
        return
    }

    // Exchange code for token
    var userInfo OAuthUserInfo
    var err error
    switch provider {
    case "google":
        userInfo, err = h.exchangeGoogle(r.Context(), code)
    case "github":
        userInfo, err = h.exchangeGitHub(r.Context(), code)
    default:
        writeError(w, http.StatusBadRequest, "unknown OAuth provider")
        return
    }
    if err != nil {
        h.log.Error().Err(err).Str("provider", provider).Msg("OAuth exchange failed")
        writeError(w, http.StatusBadRequest, "OAuth authentication failed")
        return
    }

    // Find or create user
    user, err := h.findOrCreateOAuthUser(r.Context(), userInfo, provider)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to process OAuth user")
        return
    }

    // Issue JWT
    token, err := h.jwtService.Sign(user)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "token generation failed")
        return
    }

    // Redirect to frontend với token
    frontendURL := fmt.Sprintf("%s/auth/callback?token=%s",
        h.cfg.FrontendBaseURL, token)
    http.Redirect(w, r, frontendURL, http.StatusFound)
}
```

### Fix 3: Gateway — Đảm bảo /auth/callback KHÔNG có auth middleware

```go
// apps/osv/internal/gateway/router.go

// OAuth routes — PUBLIC (no auth middleware)
mux.Handle("GET /api/v1/auth/oauth/google",  proxy.Forward(identitySvc))  // Không có authMiddleware
mux.Handle("GET /api/v1/auth/oauth/github",  proxy.Forward(identitySvc))  // Không có authMiddleware
mux.Handle("GET /api/v1/auth/callback",      proxy.Forward(identitySvc))  // Không có authMiddleware
```

> ⚠️ **CRITICAL:** `/auth/callback` phải là Public route (không có JWT middleware).  
> Hiện tại đang trả 401 vì middleware yêu cầu Bearer token — đây là bug config, không phải logic.

---

## Part B: Scan Import — 501 Not Implemented (BUG-V6-027)

**Service:** `scan-service` (:8084)  
**Reference:** `02-technical-design.md §6.2` — Import Pipeline đã được design đầy đủ (12 steps)

### Phân tích

Handler đã tồn tại (trả 501), có nghĩa là skeleton implementation đã có nhưng logic chưa complete. `02-technical-design.md §6.2` mô tả `ImportUseCase` với 12 steps.

### Fix — Implement Import Handler

```go
// services/scan-service/internal/delivery/http/import_handler.go

// POST /scans/import → import scan results từ file
func (h *ScanHandler) ImportScan(w http.ResponseWriter, r *http.Request) {
    userID := extractUserID(r)
    
    // Parse multipart form (max 50MB)
    r.ParseMultipartForm(50 << 20)
    
    file, header, err := r.FormFile("file")
    if err != nil {
        // Try JSON body fallback
        var jsonReq struct {
            Type string `json:"type"`
            Data string `json:"data"`
        }
        if jsonErr := json.NewDecoder(r.Body).Decode(&jsonReq); jsonErr != nil {
            writeError(w, http.StatusBadRequest, "provide file (multipart) or JSON body with type+data")
            return
        }
        // Process JSON import
        result, err := h.importUC.ImportFromData(r.Context(), usecase.ImportRequest{
            ToolName: jsonReq.Type,
            Data:     []byte(jsonReq.Data),
            UserID:   userID,
        })
        if err != nil {
            handleImportError(w, err)
            return
        }
        writeJSON(w, http.StatusAccepted, result)
        return
    }
    defer file.Close()

    // Parse tool name từ form hoặc file extension
    toolName := r.FormValue("type")
    if toolName == "" {
        toolName = detectToolByFilename(header.Filename)
    }
    if toolName == "" {
        writeError(w, http.StatusBadRequest, "cannot detect scan tool type, provide 'type' field")
        return
    }

    productID := parseOptionalUUID(r.FormValue("product_id"))
    testID := parseOptionalUUID(r.FormValue("test_id"))

    // Execute 12-step import pipeline (02-technical-design.md §6.2)
    result, err := h.importUC.Import(r.Context(), usecase.ImportRequest{
        File:      file,
        ToolName:  toolName,
        ProductID: productID,
        TestID:    testID,
        UserID:    userID,
        MaxSize:   50 << 20,
    })
    if err != nil {
        handleImportError(w, err)
        return
    }

    writeJSON(w, http.StatusAccepted, map[string]interface{}{
        "import_id":  result.ImportID,
        "status":     "processing",
        "created":    result.Created,
        "duplicates": result.Duplicates,
        "total":      result.Total,
        "scan_id":    result.ScanID,
    })
}

func handleImportError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, usecase.ErrUnsupportedTool):
        writeError(w, http.StatusBadRequest, err.Error())
    case errors.Is(err, usecase.ErrFileTooLarge):
        writeError(w, http.StatusRequestEntityTooLarge, "file exceeds 50MB limit")
    case errors.Is(err, usecase.ErrParseError):
        writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("parse error: %s", err))
    default:
        writeError(w, http.StatusInternalServerError, "import failed")
    }
}
```

---

## Part C: Report Download — 503 Storage Not Configured (BUG-V6-028)

**Service:** `finding-service` (:8085) — report-service embedded  
**Reference:** `01-architecture.md §3.5, §2.1` — MinIO artifact storage

### Root Cause

```
GET /reports/{id}/download → 503
{"detail":"report download not available: storage backend not configured"}
```

MinIO (S3-compatible storage) chưa được cấu hình cho môi trường development.  
`01-architecture.md` định nghĩa: `MinIO artifact storage` cho report files.

### Fix 1: MinIO Configuration

**File:** `deploy/dev/configs/finding-service.yaml`

```yaml
storage:
  backend: "minio"         # hoặc "local" cho dev
  minio:
    endpoint: "minio:9000"
    access_key: "minioadmin"
    secret_key: "minioadmin"
    bucket: "osv-reports"
    secure: false           # true khi dùng HTTPS
  
  # Alternative: local filesystem (dev only)
  local:
    path: "/tmp/osv-reports"
```

**File:** `deploy/dev/docker-compose.server.yaml` — thêm MinIO service

```yaml
services:
  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio_data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 30s

volumes:
  minio_data:
```

### Fix 2: Fallback Strategy — Local File Storage cho Dev

```go
// services/finding-service/internal/infra/storage/storage.go

type StorageBackend interface {
    Save(ctx context.Context, key string, data []byte, contentType string) error
    GetURL(ctx context.Context, key string, ttl time.Duration) (string, error)
    GetStream(ctx context.Context, key string) (io.ReadCloser, error)
}

// Factory: chọn backend dựa trên config
func NewStorageBackend(cfg StorageConfig) StorageBackend {
    switch cfg.Backend {
    case "minio":
        return NewMinIOStorage(cfg.MinIO)
    case "local":
        return NewLocalStorage(cfg.Local.Path)
    default:
        // Log warning và dùng local fallback
        log.Warn().Msg("no storage backend configured, using local fallback")
        return NewLocalStorage("/tmp/osv-reports")
    }
}
```

### Fix 3: Report Download Handler — Graceful handling khi storage chưa ready

```go
// services/finding-service/internal/delivery/http/report_handler.go

// GET /reports/{id}/download
func (h *ReportHandler) Download(w http.ResponseWriter, r *http.Request) {
    reportID, err := uuid.Parse(r.PathValue("id"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "invalid report ID")
        return
    }

    report, err := h.reportRepo.GetByID(r.Context(), reportID)
    if err != nil {
        if errors.Is(err, domain.ErrNotFound) {
            writeError(w, http.StatusNotFound, "report not found")
            return
        }
        writeError(w, http.StatusInternalServerError, "internal error")
        return
    }

    // Check report status
    if report.Status != "completed" {
        writeError(w, http.StatusConflict,
            fmt.Sprintf("report not ready (status: %s)", report.Status))
        return
    }

    // Try to get download URL
    downloadURL, err := h.storage.GetURL(r.Context(), report.StorageKey, 15*time.Minute)
    if err != nil {
        // Nếu storage không available: trả lỗi có thể retry
        h.log.Warn().Err(err).Str("report_id", reportID.String()).
            Msg("storage unavailable for report download")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Header().Set("Retry-After", "60")
        json.NewEncoder(w).Encode(map[string]string{
            "error":   "STORAGE_UNAVAILABLE",
            "message": "Report storage is temporarily unavailable",
            "hint":    "Please configure MINIO_ENDPOINT in server config",
        })
        return
    }

    // Redirect to presigned URL hoặc stream file
    if isPresignedURL(downloadURL) {
        http.Redirect(w, r, downloadURL, http.StatusFound)
    } else {
        // Stream directly
        stream, _ := h.storage.GetStream(r.Context(), report.StorageKey)
        defer stream.Close()
        
        w.Header().Set("Content-Type", mimeTypeForFormat(report.Format))
        w.Header().Set("Content-Disposition",
            fmt.Sprintf(`attachment; filename="%s.%s"`, report.Name, report.Format))
        io.Copy(w, stream)
    }
}
```

---

## Summary

| Bug | Fix | Service |
|-----|-----|---------|
| BUG-V6-024,025 | Add OAuth config + graceful 503 response | identity-service |
| BUG-V6-026 | Remove auth middleware from /auth/callback | apps/osv gateway |
| BUG-V6-027 | Implement scan import pipeline (§6.2) | scan-service |
| BUG-V6-028 | Configure MinIO + local storage fallback | finding-service |

---

## Verification

```bash
# OAuth — expect 503 với JSON error (không phải 400 empty)
curl -v https://c12.openledger.vn/api/v1/auth/oauth/google 2>&1 | grep -A5 "< HTTP"
# Expected: HTTP/1.1 503 {"error":"OAUTH_NOT_CONFIGURED",...}

# /auth/callback — expect 400 (missing params), NOT 401
curl -v https://c12.openledger.vn/api/v1/auth/callback 2>&1 | grep "< HTTP"
# Expected: HTTP/1.1 400

# Scan import
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -F "type=nmap" \
  -F "file=@scan_results.xml" \
  https://c12.openledger.vn/api/v1/scans/import
# Expected: 202

# Report download (với MinIO configured)
curl -H "Authorization: Bearer $TOKEN" \
  https://c12.openledger.vn/api/v1/reports/$REPORT_ID/download
# Expected: 302 redirect to presigned URL, OR 200 with file stream
```
