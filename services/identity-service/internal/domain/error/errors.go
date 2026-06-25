// Package domainerr defines sentinel errors for the auth service domain.
package domainerr

import "errors"

var (
	// ErrEmailAlreadyExists is returned when registering a duplicate email.
	ErrEmailAlreadyExists = errors.New("email already exists")

	// ErrUsernameAlreadyExists is returned when registering a duplicate username.
	ErrUsernameAlreadyExists = errors.New("username already exists")

	// ErrInvalidCredentials is returned when email or password is incorrect.
	ErrInvalidCredentials = errors.New("invalid email or password")

	// ErrAccountLocked is returned when too many failed login attempts occur.
	ErrAccountLocked = errors.New("account locked due to too many failed attempts")

	// ErrAccountInactive is returned when a deactivated user tries to log in.
	ErrAccountInactive = errors.New("account is not active")

	// ErrMFARequired is returned when MFA is enabled but no code was provided.
	ErrMFARequired = errors.New("MFA code required")

	// ErrInvalidMFACode is returned when the TOTP code is incorrect.
	ErrInvalidMFACode = errors.New("invalid MFA code")

	// ErrTokenRevoked is returned when a refresh token has been explicitly revoked.
	ErrTokenRevoked = errors.New("token has been revoked")

	// ErrTokenExpired is returned when a token's expiry time has passed.
	ErrTokenExpired = errors.New("token has expired")

	// ErrInvalidToken is returned for malformed or signature-invalid tokens.
	ErrInvalidToken = errors.New("invalid or malformed token")

	// ErrAPIKeyNotFound is returned when an API key doesn't exist or is revoked.
	ErrAPIKeyNotFound = errors.New("API key not found or revoked")

	// ErrAPIKeyExpired is returned when the API key's expiry has passed.
	ErrAPIKeyExpired = errors.New("API key has expired")

	// ErrUserNotFound is returned when no user matches a lookup.
	ErrUserNotFound = errors.New("user not found")

	// ErrSessionNotFound is returned when no session matches a refresh token.
	ErrSessionNotFound = errors.New("session not found")

	// ErrOAuthProviderError is returned when an OAuth provider API call fails.
	ErrOAuthProviderError = errors.New("OAuth provider error")

	// ErrTOTPNotSetup is returned when verifying TOTP but no pending secret exists.
	ErrTOTPNotSetup = errors.New("TOTP setup not initiated")

	// ErrInvalidTOTPCode is returned when the TOTP code does not match the secret.
	ErrInvalidTOTPCode = errors.New("invalid TOTP code")

	// ErrMFANotEnabled is returned when disabling TOTP but MFA is already off.
	ErrMFANotEnabled = errors.New("MFA is not enabled for this account")
)
