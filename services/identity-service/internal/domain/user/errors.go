package user

import "errors"

var (
    ErrEmailRequired    = errors.New("email is required")
    ErrEmailInvalid     = errors.New("email format is invalid")
    ErrEmailTaken       = errors.New("email is already registered")
    ErrUsernameRequired = errors.New("username is required")
    ErrUsernameLength   = errors.New("username must be between 3 and 50 characters")
    ErrUsernameTaken    = errors.New("username is already taken")
    ErrPasswordTooShort = errors.New("password must be at least 12 characters")
    ErrUserNotFound     = errors.New("user not found")
    ErrUserLocked       = errors.New("account is locked due to too many failed login attempts")
    ErrUserInactive     = errors.New("account is inactive")
    ErrInvalidCredentials = errors.New("invalid email or password")
    ErrMFARequired      = errors.New("MFA code is required")
    ErrMFAInvalidCode   = errors.New("invalid or expired MFA code")
    ErrMFAAlreadyEnabled = errors.New("MFA is already enabled")
    ErrMFANotEnabled    = errors.New("MFA is not enabled")
)
