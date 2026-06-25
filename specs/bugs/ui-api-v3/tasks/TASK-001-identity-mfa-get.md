# TASK-001: Identity-Service — Fix MFA GET /mfa/setup

> **Bug**: BUG-001  
> **Solution**: SOL-002  
> **Service**: `services/identity-service`  
> **File chính**: `adapter/handler/http/router.go`  
> **Priority**: 🔴 HIGH  
> **Status**: `[x] DONE`

## Phân Tích Thực Tế

**Vấn đề**: Gateway forward `GET /api/v1/auth/mfa/setup` (sau rewrite thành `/api/v1/auth/totp/setup`) nhưng identity-service router chỉ có:
```go
// adapter/handler/http/router.go
r.Post("/totp/setup", totpH.Setup)   // POST — KHÔNG có GET
r.Post("/totp/verify", totpH.Verify)
```

Frontend gọi `GET /api/v1/auth/mfa/setup` để lấy QR code + secret (setup TOTP). Server hiện chỉ có `POST /auth/totp/setup` → `405 Method Not Allowed`.

**Kiểm tra TOTP usecase**:
```bash
find services/identity-service -name "*.go" | xargs grep -l "totp\|TOTP" 2>/dev/null
cat services/identity-service/internal/infrastructure/crypto/apikey_totp.go
cat services/identity-service/internal/usecase/totp/  # nếu có
```

## Việc Cần Làm

### Bước 1: Verify TOTP Handler có method Setup (GET) hay không

```bash
grep -n "func.*Setup\|func.*Verify\|func.*Generate" \
  services/identity-service/adapter/handler/http/auth_handler.go \
  services/identity-service/adapter/handler/http/*.go 2>/dev/null
```

### Bước 2: Nếu TOTP handler chỉ có POST — thêm GET handler

File: `services/identity-service/adapter/handler/http/totp_handler.go` (tạo mới nếu chưa có, hoặc sửa file hiện tại)

```go
// TOTPHandler handles TOTP setup and verification.
// Inject totpUC từ usecase/totp package
type TOTPHandler struct {
    totpUC TOTPUseCase
    log    zerolog.Logger
}

// GET /api/v1/auth/totp/setup — generate TOTP secret + QR code
// Gateway rewrites: GET /api/v1/auth/mfa/setup → GET /api/v1/auth/totp/setup
func (h *TOTPHandler) GetSetupInfo(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("X-User-ID")
    if userID == "" {
        writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "missing user identity"))
        return
    }

    secret, qrURL, err := h.totpUC.GenerateSecret(r.Context(), userID)
    if err != nil {
        h.log.Error().Err(err).Str("user_id", userID).Msg("TOTP GenerateSecret failed")
        writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", "failed to generate TOTP secret"))
        return
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "secret":  secret,
        "qr_url":  qrURL,
        "algorithm": "SHA1",
        "digits":    6,
        "period":    30,
    })
}

// POST /api/v1/auth/totp/setup — already exists (keep)
func (h *TOTPHandler) Setup(w http.ResponseWriter, r *http.Request) { ... }

// POST /api/v1/auth/totp/verify — already exists (keep)
func (h *TOTPHandler) Verify(w http.ResponseWriter, r *http.Request) { ... }
```

### Bước 3: Kiểm tra TOTP UseCase có `GenerateSecret`

```bash
find services/identity-service -path "*/usecase/totp*" -name "*.go" | xargs cat 2>/dev/null | head -60
```

Nếu chưa có `GenerateSecret`, implement:

```go
// services/identity-service/internal/usecase/totp/usecase.go

func (uc *UseCase) GenerateSecret(ctx context.Context, userID string) (secret, qrURL string, err error) {
    // Check if user already has TOTP
    existing, _ := uc.repo.GetTOTP(ctx, userID)
    if existing != nil && existing.Enabled {
        return "", "", ErrTOTPAlreadyEnabled
    }

    // Generate TOTP secret (RFC 6238)
    key, err := totp.Generate(totp.GenerateOpts{
        Issuer:      "OSV Platform",
        AccountName: userID,
        SecretSize:  20,
    })
    if err != nil {
        return "", "", fmt.Errorf("generate totp key: %w", err)
    }

    // Encrypt secret before storing (AES-256-GCM)
    encryptedSecret, err := uc.crypto.Encrypt([]byte(key.Secret()))
    if err != nil {
        return "", "", fmt.Errorf("encrypt totp secret: %w", err)
    }

    // Store as pending (not yet enabled)
    if err := uc.repo.StorePendingTOTP(ctx, userID, encryptedSecret); err != nil {
        return "", "", fmt.Errorf("store totp: %w", err)
    }

    return key.Secret(), key.URL(), nil
}
```

### Bước 4: Register GET route trong router

File: `services/identity-service/adapter/handler/http/router.go`

```go
// Thay đổi:
r.Post("/totp/setup", totpH.Setup)
r.Post("/totp/verify", totpH.Verify)

// Thành:
r.Get("/totp/setup", totpH.GetSetupInfo)   // THÊM — GET để lấy QR code
r.Post("/totp/setup", totpH.Setup)          // Giữ — POST để enable (optional alternative)  
r.Post("/totp/verify", totpH.Verify)        // Giữ nguyên
r.Delete("/totp", totpH.Disable)            // Giữ nguyên
```

### Bước 5: Verify

```bash
# Build
cd services/identity-service && go build ./...

# Test via test suite
cd tests/client && python3 -c "
import requests, os
token = os.environ.get('OSV_TOKEN')
r = requests.get('https://c12.openledger.vn/api/v1/auth/mfa/setup',
    headers={'Authorization': f'Bearer {token}'})
print(r.status_code, r.json())
"
```

**Expected**: `200 OK` với `{secret, qr_url}`

## Acceptance Criteria

- [x] `GET /api/v1/auth/mfa/setup` → `200 OK` với `{secret, qr_url}`  
- [x] `POST /api/v1/auth/mfa/confirm` → vẫn hoạt động như cũ (`200 OK`)  
- [x] `go build ./...` không có lỗi  
- [x] Không ảnh hưởng đến các auth routes khác  
