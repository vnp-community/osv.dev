# Solution 001: Implement Identity MFA

**Status**: Proposed
**Target Service**: `identity-service`, `apps/osv` (Gateway)
**Related CR**: [CR-001-identity-mfa.md](../CR-001-identity-mfa.md)

## 1. Kiến trúc lưu trữ (Infrastructure Layer)
Cập nhật PostgreSQL Schema (`osv_identity`):
```sql
ALTER TABLE users 
ADD COLUMN mfa_enabled BOOLEAN DEFAULT FALSE,
ADD COLUMN mfa_secret VARCHAR(64);
```

## 2. Logic Xử Lý (Use Case Layer)
Bổ sung `MFAUseCase` trong `identity-service/internal/usecase/`:
Sử dụng thư viện `github.com/pquerna/otp/totp` để xử lý.

```go
type MFAUseCase struct {
    userRepo UserRepository
}

// SetupMFA sinh secret và URL cho mã QR
func (uc *MFAUseCase) SetupMFA(ctx context.Context, userID uuid.UUID) (*MFASetupResult, error) {
    user, _ := uc.userRepo.GetByID(ctx, userID)
    
    // Gen TOTP secret (Issuer = "OSV Platform")
    key, _ := totp.Generate(totp.GenerateOpts{
        Issuer:      "OSV Platform",
        AccountName: user.Email,
    })
    
    // Lưu secret vào db (chưa kích hoạt mfa_enabled)
    uc.userRepo.UpdateMFASecret(ctx, userID, key.Secret())
    
    return &MFASetupResult{
        Secret: key.Secret(),
        QRUrl:  key.URL(),
    }, nil
}

// ConfirmMFA xác thực mã OTP lần đầu và bật cờ mfa_enabled
func (uc *MFAUseCase) ConfirmMFA(ctx context.Context, userID uuid.UUID, code string) error {
    user, _ := uc.userRepo.GetByID(ctx, userID)
    valid := totp.Validate(code, user.MfaSecret)
    if !valid {
        return errors.New("invalid otp")
    }
    
    return uc.userRepo.EnableMFA(ctx, userID)
}
```

## 3. Cập nhật Luồng Đăng nhập (Login Flow)
Cập nhật `LoginUseCase` trong `identity-service`:
```go
func (uc *AuthUseCase) Login(ctx context.Context, req LoginRequest) (*LoginResult, error) {
    user, _ := uc.userRepo.FindByEmail(ctx, req.Email)
    // Check bcrypt password...
    
    if user.MfaEnabled {
        if req.MfaCode == "" {
            // Cần MFA nhưng không truyền lên, báo frontend
            return nil, ErrMFARequired
        }
        
        valid := totp.Validate(req.MfaCode, user.MfaSecret)
        if !valid {
            return nil, ErrInvalidMFA
        }
    }
    
    // Issue JWT...
}
```

## 4. Gateway Integration
*   `GET /api/v1/auth/mfa/setup` -> Forward to `identity-service` (Requires JWT)
*   `POST /api/v1/auth/mfa/confirm` -> Forward to `identity-service` (Requires JWT)
*   Nếu `LoginUseCase` trả về `ErrMFARequired`, HTTP handler sẽ trả HTTP 403:
    ```json
    { "mfa_required": true }
    ```
