# TASK-P1-003 — OAuth Credentials từ Environment Variables

**Bug:** MOCK-011  
**Priority:** 🔴 P1 — Feature không hoạt động  
**Effort:** ~30 phút  
**Service:** `identity-service`  
**Loại thay đổi:** Sửa embedded.go + Sửa OAuth provider + Tạo .env.example

---

## Mục tiêu

`identity-service/embedded.go` khởi tạo Google và GitHub OAuth providers với empty credentials `("", "", "")`. Mọi OAuth login sẽ thất bại với lỗi `invalid_client`. Cần đọc credentials từ environment variables.

---

## Preconditions

- [ ] Đọc `services/identity-service/embedded.go` — xác định dòng `NewGoogleProvider` và `NewGitHubProvider`
- [ ] Đọc provider files để xem cấu trúc:
  ```bash
  find services/identity-service -name "google*" -o -name "github*" | grep -v test
  ```
- [ ] Xác định field names trong struct provider (clientID, clientSecret, redirectURL)

---

## Steps

### Step 1 — Sửa embedded.go: đọc OAuth credentials từ env vars

Mở `services/identity-service/embedded.go`.

Tìm các dòng:
```go
googleProvider := oauth.NewGoogleProvider("", "", "")
githubProvider := oauth.NewGitHubProvider("", "", "")
```

Thay bằng:

```go
// FIX MOCK-011: đọc OAuth credentials từ environment variables
googleClientID     := os.Getenv("GOOGLE_CLIENT_ID")
googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
googleRedirectURL  := os.Getenv("GOOGLE_REDIRECT_URL")
if googleRedirectURL == "" {
    googleRedirectURL = "http://localhost:8080/api/v1/auth/callback?provider=google"
}

githubClientID     := os.Getenv("GITHUB_CLIENT_ID")
githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
githubRedirectURL  := os.Getenv("GITHUB_REDIRECT_URL")
if githubRedirectURL == "" {
    githubRedirectURL = "http://localhost:8080/api/v1/auth/callback?provider=github"
}

googleProvider := oauth.NewGoogleProvider(googleClientID, googleClientSecret, googleRedirectURL)
githubProvider := oauth.NewGitHubProvider(githubClientID, githubClientSecret, githubRedirectURL)

if googleClientID == "" {
    logger.Warn().Msg("identity-service: GOOGLE_CLIENT_ID not set, Google OAuth disabled")
}
if githubClientID == "" {
    logger.Warn().Msg("identity-service: GITHUB_CLIENT_ID not set, GitHub OAuth disabled")
}
```

> **Lưu ý**: `os` package cần được import. Kiểm tra xem đã có chưa.

### Step 2 — Thêm credentials validation trong OAuth provider

Tìm và mở Google OAuth provider file:
```bash
find services/identity-service -name "*.go" | xargs grep -l "NewGoogleProvider\|GoogleProvider" | head -3
```

Tìm method xử lý OAuth redirect/callback (thường là `OAuthRedirect`, `Redirect`, `AuthURL`):

```go
// FIX MOCK-011: check credentials trước khi redirect
func (p *GoogleProvider) OAuthRedirect(w http.ResponseWriter, r *http.Request) {
    if p.clientID == "" || p.clientSecret == "" {
        http.Error(w,
            `{"error":"oauth_not_configured","detail":"Google OAuth credentials not set. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables."}`,
            http.StatusServiceUnavailable)
        return
    }
    // ... phần còn lại giữ nguyên
}
```

Làm tương tự cho GitHub provider.

### Step 3 — Tạo .env.example

**File mới**: `services/identity-service/.env.example`

```bash
# =========================================
# Identity Service — Environment Variables
# =========================================

# Database
DATABASE_URL=postgres://osv:osv@localhost:5432/osv_db?sslmode=disable

# JWT
JWT_SECRET=change-me-to-a-secure-random-string-min-32-chars

# OAuth — Google (optional: chỉ cần nếu dùng Google social login)
# Lấy từ: https://console.cloud.google.com/apis/credentials
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URL=http://localhost:8080/api/v1/auth/callback?provider=google

# OAuth — GitHub (optional: chỉ cần nếu dùng GitHub social login)
# Lấy từ: https://github.com/settings/developers
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
GITHUB_REDIRECT_URL=http://localhost:8080/api/v1/auth/callback?provider=github

# LDAP (optional)
LDAP_URL=
LDAP_BIND_DN=
LDAP_BIND_PASSWORD=
LDAP_BASE_DN=
```

### Step 4 — Cập nhật docker-compose để inject OAuth env vars

Tìm file docker-compose đang dùng:
```bash
ls deploy/dev/docker-compose*.yml
```

Thêm OAuth env vars vào service `osv-monolith` hoặc `identity-service`:

```yaml
environment:
  # OAuth Google (optional)
  - GOOGLE_CLIENT_ID=${GOOGLE_CLIENT_ID:-}
  - GOOGLE_CLIENT_SECRET=${GOOGLE_CLIENT_SECRET:-}
  - GOOGLE_REDIRECT_URL=${GOOGLE_REDIRECT_URL:-http://localhost:8080/api/v1/auth/callback?provider=google}
  # OAuth GitHub (optional)  
  - GITHUB_CLIENT_ID=${GITHUB_CLIENT_ID:-}
  - GITHUB_CLIENT_SECRET=${GITHUB_CLIENT_SECRET:-}
  - GITHUB_REDIRECT_URL=${GITHUB_REDIRECT_URL:-http://localhost:8080/api/v1/auth/callback?provider=github}
```

---

## Acceptance Criteria

- [ ] Khi `GOOGLE_CLIENT_ID` không set → log warning "Google OAuth disabled" khi khởi động
- [ ] `GET /api/v1/auth/oauth/google` khi không có credentials → trả `503` JSON (không redirect)
- [ ] Khi set đúng credentials → OAuth redirect hoạt động bình thường
- [ ] `go build ./services/identity-service/...` — thành công
- [ ] File `.env.example` tồn tại với documentation đầy đủ

---

## Test Commands

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev
go build ./services/identity-service/...
go vet ./services/identity-service/...

# Verify env vars being read
grep -n "os.Getenv.*GOOGLE\|os.Getenv.*GITHUB" services/identity-service/embedded.go

# Test OAuth without credentials (cần service đang chạy)
curl -v http://localhost:8080/api/v1/auth/oauth/google
# Expect: HTTP 503 with JSON error
```
