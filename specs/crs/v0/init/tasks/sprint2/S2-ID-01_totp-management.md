# S2-ID-01 — Thêm TOTP Management UC + Handler (identity-service)


## ✅ Execution Status: COMPLETED
## Metadata
- **Task ID**: S2-ID-01
- **Service**: identity-service
- **Sprint**: 2 (P1)
- **Ước tính**: 3 giờ
- **Dependencies**: Không có (độc lập)
- **Spec nguồn**: `specs/develop/01_identity-service-upgrade.md` § "P1 — Thiếu: TOTP Management"

## Context

```bash
# Đọc existing TOTP logic (đây là crypto helper, không phải UC):
cat services/identity-service/internal/infrastructure/crypto/apikey_totp.go

# Đọc login UC để biết MFA flow:
cat services/identity-service/internal/usecase/login/login.go | grep -A5 -i "mfa\|totp"

# Đọc user entity để biết MFA fields:
cat services/identity-service/internal/domain/entity/user.go | grep -i "mfa\|totp\|secret"

# Đọc router để biết cách thêm routes:
cat services/identity-service/internal/adapter/handler/http/router.go

# Đọc existing handler để biết response pattern:
cat services/identity-service/internal/adapter/handler/http/auth_handler.go
```

## Goal

Thêm 3 TOTP management use cases (setup, verify, disable) và HTTP handler.
`infrastructure/crypto/apikey_totp.go` GIỮ NGUYÊN — UC sẽ gọi crypto helper.

## Files to Create

### File 1: `services/identity-service/internal/usecase/totp/setup.go`

```go
package totp

import (
	"context"
	"encoding/base32"
	"crypto/rand"
	"fmt"

	"github.com/google/uuid"
	"github.com/osv/identity-service/internal/domain/repository"
)

const (
	totpIssuer  = "OSV Security Platform"
	totpKeySize = 20  // 160 bits
)

// SetupRequest defines input for TOTP setup.
type SetupRequest struct {
	UserID uuid.UUID
}

// SetupResponse contains TOTP setup data to present to user.
type SetupResponse struct {
	Secret     string `json:"secret"`      // Base32 encoded secret
	QRCodeURL  string `json:"qr_code_url"` // otpauth:// URI for QR generation
	ManualCode string `json:"manual_code"` // 4-digit groups for manual entry
}

// SetupUseCase generates a new TOTP secret for a user.
type SetupUseCase struct {
	userRepo repository.UserRepository
}

// NewSetupUseCase creates a new SetupUseCase.
func NewSetupUseCase(userRepo repository.UserRepository) *SetupUseCase {
	return &SetupUseCase{userRepo: userRepo}
}

// Execute generates a TOTP secret (does NOT enable MFA yet — requires Verify step).
func (uc *SetupUseCase) Execute(ctx context.Context, req SetupRequest) (*SetupResponse, error) {
	// Get user to find their email (used as TOTP account name)
	user, err := uc.userRepo.FindByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	// Generate random secret
	secretBytes := make([]byte, totpKeySize)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	secret := base32.StdEncoding.EncodeToString(secretBytes)

	// Build otpauth URI (compatible with Google Authenticator, Authy, etc.)
	qrURL := fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		totpIssuer, user.Email, secret, totpIssuer,
	)

	// Store pending secret in user record (not yet active until Verify)
	if err := uc.userRepo.StorePendingTOTPSecret(ctx, req.UserID, secret); err != nil {
		return nil, fmt.Errorf("store pending secret: %w", err)
	}

	return &SetupResponse{
		Secret:    secret,
		QRCodeURL: qrURL,
		ManualCode: formatManualCode(secret),
	}, nil
}

// formatManualCode formats the Base32 secret in groups of 4 for readability.
func formatManualCode(secret string) string {
	if len(secret) < 4 {
		return secret
	}
	var groups []string
	for i := 0; i < len(secret); i += 4 {
		end := i + 4
		if end > len(secret) {
			end = len(secret)
		}
		groups = append(groups, secret[i:end])
	}
	return strings.Join(groups, " ")
}
```

### File 2: `services/identity-service/internal/usecase/totp/verify.go`

```go
package totp

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"  // or use existing crypto package

	"github.com/osv/identity-service/internal/domain/repository"
	domainerr "github.com/osv/identity-service/internal/domain/error"
)

// VerifyRequest confirms TOTP setup is working and activates MFA.
type VerifyRequest struct {
	UserID uuid.UUID
	Code   string  // 6-digit TOTP code from authenticator app
}

// VerifyUseCase confirms TOTP and enables MFA on the account.
type VerifyUseCase struct {
	userRepo repository.UserRepository
}

// NewVerifyUseCase creates a new VerifyUseCase.
func NewVerifyUseCase(userRepo repository.UserRepository) *VerifyUseCase {
	return &VerifyUseCase{userRepo: userRepo}
}

// Execute verifies the TOTP code and activates MFA.
func (uc *VerifyUseCase) Execute(ctx context.Context, req VerifyRequest) error {
	// Get user with pending TOTP secret
	user, err := uc.userRepo.FindByID(ctx, req.UserID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	// Get pending secret
	pendingSecret, err := uc.userRepo.GetPendingTOTPSecret(ctx, req.UserID)
	if err != nil || pendingSecret == "" {
		return domainerr.ErrTOTPNotSetup
	}

	// Validate TOTP code against pending secret
	// Use existing infrastructure/crypto/apikey_totp.go or pquerna/otp
	valid := totp.Validate(req.Code, pendingSecret)
	if !valid {
		return domainerr.ErrInvalidTOTPCode
	}

	// Activate MFA: set MFAEnabled=true, set MFASecret, clear pending
	if err := uc.userRepo.ActivateTOTP(ctx, req.UserID, pendingSecret); err != nil {
		return fmt.Errorf("activate TOTP: %w", err)
	}

	return nil
}
```

### File 3: `services/identity-service/internal/usecase/totp/disable.go`

```go
package totp

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/osv/identity-service/internal/domain/repository"
	"github.com/osv/identity-service/internal/infrastructure/crypto"
	domainerr "github.com/osv/identity-service/internal/domain/error"
)

// DisableRequest disables TOTP/MFA for a user.
type DisableRequest struct {
	UserID          uuid.UUID
	CurrentPassword string  // Require password confirmation for security
}

// DisableUseCase disables TOTP/MFA on an account.
type DisableUseCase struct {
	userRepo repository.UserRepository
	hasher   crypto.PasswordHasher  // reuse existing crypto
}

// NewDisableUseCase creates a new DisableUseCase.
func NewDisableUseCase(userRepo repository.UserRepository, hasher crypto.PasswordHasher) *DisableUseCase {
	return &DisableUseCase{userRepo: userRepo, hasher: hasher}
}

// Execute disables MFA after verifying current password.
func (uc *DisableUseCase) Execute(ctx context.Context, req DisableRequest) error {
	user, err := uc.userRepo.FindByID(ctx, req.UserID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	// Require password confirmation (security check)
	if err := uc.hasher.Verify(user.PasswordHash, req.CurrentPassword); err != nil {
		return domainerr.ErrInvalidCredentials
	}

	if !user.MFAEnabled {
		return domainerr.ErrMFANotEnabled
	}

	// Disable MFA
	if err := uc.userRepo.DisableTOTP(ctx, req.UserID); err != nil {
		return fmt.Errorf("disable TOTP: %w", err)
	}

	return nil
}
```

### File 4: `services/identity-service/internal/adapter/handler/http/totp_handler.go`

```go
package http

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/osv/identity-service/internal/usecase/totp"
)

// TOTPHandler handles TOTP management HTTP requests.
type TOTPHandler struct {
	setupUC   *totp.SetupUseCase
	verifyUC  *totp.VerifyUseCase
	disableUC *totp.DisableUseCase
	log       zerolog.Logger
}

// NewTOTPHandler creates a new TOTPHandler.
func NewTOTPHandler(
	setupUC *totp.SetupUseCase,
	verifyUC *totp.VerifyUseCase,
	disableUC *totp.DisableUseCase,
	log zerolog.Logger,
) *TOTPHandler {
	return &TOTPHandler{
		setupUC:   setupUC,
		verifyUC:  verifyUC,
		disableUC: disableUC,
		log:       log,
	}
}

// Setup handles POST /api/v1/auth/totp/setup
func (h *TOTPHandler) Setup(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)  // từ JWT context
	if userID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	resp, err := h.setupUC.Execute(r.Context(), totp.SetupRequest{UserID: userID})
	if err != nil {
		h.log.Error().Err(err).Msg("totp.Setup")
		http.Error(w, "failed to setup TOTP", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Verify handles POST /api/v1/auth/totp/verify
func (h *TOTPHandler) Verify(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.verifyUC.Execute(r.Context(), totp.VerifyRequest{
		UserID: userID,
		Code:   req.Code,
	}); err != nil {
		http.Error(w, "TOTP verification failed", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Disable handles DELETE /api/v1/auth/totp
func (h *TOTPHandler) Disable(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.disableUC.Execute(r.Context(), totp.DisableRequest{
		UserID:          userID,
		CurrentPassword: req.Password,
	}); err != nil {
		http.Error(w, "failed to disable TOTP", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

## Files to Extend

### Extend: `services/identity-service/internal/adapter/handler/http/router.go`

```go
// Tìm cuối router setup, thêm TOTP routes:
// (Giữ NGUYÊN toàn bộ routes cũ)

totpHandler := http.NewTOTPHandler(setupTOTPUC, verifyTOTPUC, disableTOTPUC, log)

r.Post("/api/v1/auth/totp/setup", totpHandler.Setup)    // NEW
r.Post("/api/v1/auth/totp/verify", totpHandler.Verify)  // NEW
r.Delete("/api/v1/auth/totp", totpHandler.Disable)      // NEW
```

### Extend: `services/identity-service/internal/domain/repository/repositories.go`

```go
// Thêm methods vào UserRepository interface:
type UserRepository interface {
    // ... existing methods giữ nguyên ...

    // TOTP management (NEW)
    StorePendingTOTPSecret(ctx context.Context, userID uuid.UUID, secret string) error
    GetPendingTOTPSecret(ctx context.Context, userID uuid.UUID) (string, error)
    ActivateTOTP(ctx context.Context, userID uuid.UUID, secret string) error
    DisableTOTP(ctx context.Context, userID uuid.UUID) error
}
```

### Extend: `services/identity-service/internal/adapter/repository/postgres/user_repo.go`

Implement 4 new methods (thêm vào cuối file):

```go
func (r *userRepo) StorePendingTOTPSecret(ctx context.Context, userID uuid.UUID, secret string) error {
    _, err := r.db.Exec(ctx,
        `UPDATE users SET pending_totp_secret = $2 WHERE id = $1`,
        userID, secret,
    )
    return err
}

func (r *userRepo) GetPendingTOTPSecret(ctx context.Context, userID uuid.UUID) (string, error) {
    var secret string
    err := r.db.QueryRow(ctx,
        `SELECT COALESCE(pending_totp_secret, '') FROM users WHERE id = $1`,
        userID,
    ).Scan(&secret)
    return secret, err
}

func (r *userRepo) ActivateTOTP(ctx context.Context, userID uuid.UUID, secret string) error {
    _, err := r.db.Exec(ctx,
        `UPDATE users SET mfa_enabled = TRUE, mfa_secret = $2, pending_totp_secret = NULL WHERE id = $1`,
        userID, secret,
    )
    return err
}

func (r *userRepo) DisableTOTP(ctx context.Context, userID uuid.UUID) error {
    _, err := r.db.Exec(ctx,
        `UPDATE users SET mfa_enabled = FALSE, mfa_secret = NULL WHERE id = $1`,
        userID,
    )
    return err
}
```

## Migration

```sql
-- migrations/002_totp_pending.sql  ← NEW
ALTER TABLE auth.users
    ADD COLUMN IF NOT EXISTS pending_totp_secret VARCHAR(100);
```

## Verification

```bash
cd services/identity-service && go build ./...

# Test TOTP flow:
# 1. POST /api/v1/auth/totp/setup → get QR code URL
# 2. Open QR in authenticator app
# 3. POST /api/v1/auth/totp/verify với code từ app
# 4. Try login → should require TOTP code
# 5. DELETE /api/v1/auth/totp với password → MFA disabled

go get github.com/pquerna/otp  # nếu chưa có
```

## Notes

- Nếu codebase dùng existing `infrastructure/crypto/apikey_totp.go`, import và gọi đó thay vì `pquerna/otp`
- `StorePendingTOTPSecret` lưu secret chưa activate — tránh bật MFA cho user khi họ chưa verify app
- `mfa_secret` column cần có trong `auth.users` — kiểm tra migration 001 xem đã có chưa
