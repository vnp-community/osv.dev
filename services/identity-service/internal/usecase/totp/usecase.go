package totp

import (
    "context"
    "crypto/rand"
    "encoding/base32"
    "encoding/hex"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/pquerna/otp/totp"

    "github.com/osv/identity-service/internal/domain/entity"
)

const (
    issuer      = "OpenVulnScan"
    secretLength = 20 // bytes = 160-bit TOTP secret (RFC 6238)
)

var (
    ErrMFAAlreadyEnabled  = errors.New("MFA is already enabled for this user")
    ErrMFANotEnabled      = errors.New("MFA is not enabled for this user")
    ErrInvalidTOTPCode    = errors.New("invalid or expired TOTP code")
    ErrUserNotFound       = errors.New("user not found")
)

// UserRepository defines storage needed for MFA operations.
type UserRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
    UpdateMFA(ctx context.Context, userID uuid.UUID, enabled bool, encryptedSecret string) error
}

// SetupOutput contains the TOTP provisioning URI and backup codes.
type SetupOutput struct {
    Secret      string   // base32 secret (for display)
    OTPAuthURI  string   // otpauth:// URI for QR code
    BackupCodes []string // CR-001: one-time backup codes for account recovery
}

// UseCase handles TOTP MFA enrollment and validation.
type UseCase struct {
    userRepo UserRepository
    // In production, provide an encryptor for AES-256-GCM secret storage
}

// New creates a TOTPUseCase.
func New(userRepo UserRepository) *UseCase {
    return &UseCase{userRepo: userRepo}
}

// Setup generates a new TOTP secret and returns the provisioning URI.
// Does NOT enable MFA — user must confirm with a valid code first.
func (uc *UseCase) Setup(ctx context.Context, userID uuid.UUID) (*SetupOutput, error) {
    user, err := uc.userRepo.FindByID(ctx, userID)
    if err != nil {
        return nil, ErrUserNotFound
    }
    if user.MFAEnabled {
        return nil, ErrMFAAlreadyEnabled
    }

    // Generate random secret
    rawSecret := make([]byte, secretLength)
    if _, err := rand.Read(rawSecret); err != nil {
        return nil, fmt.Errorf("generate totp secret: %w", err)
    }
    secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(rawSecret)

    // Generate OTP Auth URI
    key, err := totp.Generate(totp.GenerateOpts{
        Issuer:      issuer,
        AccountName: user.Email,
        Secret:      rawSecret,
        Period:      30,
        Digits:      6,
    })
    if err != nil {
        return nil, fmt.Errorf("generate totp key: %w", err)
    }

    // Store secret (temporarily unconfirmed) — encrypted in production
    if err := uc.userRepo.UpdateMFA(ctx, userID, false, secret); err != nil {
        return nil, fmt.Errorf("store mfa secret: %w", err)
    }

    // CR-001: generate 8 backup codes (one-time use, shown only at setup)
    backupCodes, err := generateBackupCodes(8)
    if err != nil {
        return nil, fmt.Errorf("generate backup codes: %w", err)
    }

    return &SetupOutput{
        Secret:      secret,
        OTPAuthURI:  key.URL(),
        BackupCodes: backupCodes,
    }, nil
}

// generateBackupCodes returns n random 8-character hex codes.
func generateBackupCodes(n int) ([]string, error) {
    codes := make([]string, n)
    buf := make([]byte, 4) // 4 bytes = 8 hex chars
    for i := range codes {
        if _, err := rand.Read(buf); err != nil {
            return nil, err
        }
        codes[i] = hex.EncodeToString(buf)
    }
    return codes, nil
}

// Confirm validates the TOTP code and enables MFA for the user.
func (uc *UseCase) Confirm(ctx context.Context, userID uuid.UUID, code string) error {
    user, err := uc.userRepo.FindByID(ctx, userID)
    if err != nil {
        return ErrUserNotFound
    }
    if user.MFAEnabled {
        return ErrMFAAlreadyEnabled
    }
    if user.MFATOTPSecret == nil || *user.MFATOTPSecret == "" {
        return ErrMFANotEnabled
    }

    // Validate code against stored secret
    valid := totp.Validate(code, *user.MFATOTPSecret)
    if !valid {
        return ErrInvalidTOTPCode
    }

    // Enable MFA
    return uc.userRepo.UpdateMFA(ctx, userID, true, *user.MFATOTPSecret)
}

// Disable disables MFA after verifying current TOTP code.
func (uc *UseCase) Disable(ctx context.Context, userID uuid.UUID, code string) error {
    user, err := uc.userRepo.FindByID(ctx, userID)
    if err != nil {
        return ErrUserNotFound
    }
    if !user.MFAEnabled {
        return ErrMFANotEnabled
    }

    valid := totp.Validate(code, *user.MFATOTPSecret)
    if !valid {
        return ErrInvalidTOTPCode
    }

    return uc.userRepo.UpdateMFA(ctx, userID, false, "")
}

// Validate checks a TOTP code against the user's stored secret.
// Used by the login flow when MFAEnabled=true.
func (uc *UseCase) Validate(ctx context.Context, userID uuid.UUID, code string) error {
    user, err := uc.userRepo.FindByID(ctx, userID)
    if err != nil {
        return ErrUserNotFound
    }
    if !user.MFAEnabled {
        return ErrMFANotEnabled
    }

    // RFC 6238: 30s window, allow ±1 step for clock skew
    valid, err := totp.ValidateCustom(code, *user.MFATOTPSecret, time.Now().UTC(), totp.ValidateOpts{
        Period:    30,
        Skew:      1,
        Digits:    6,
        Algorithm: 0,
    })
    if err != nil || !valid {
        return ErrInvalidTOTPCode
    }
    return nil
}
