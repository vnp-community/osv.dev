package apikey

import "errors"

var (
    ErrNameRequired     = errors.New("API key name is required")
    ErrNameTooLong      = errors.New("API key name must be 100 characters or less")
    ErrInvalidKeyFormat = errors.New("invalid API key format: must start with 'ovs_'")
    ErrKeyNotFound      = errors.New("API key not found")
    ErrKeyRevoked       = errors.New("API key has been revoked")
    ErrKeyExpired       = errors.New("API key has expired")
    ErrPermissionEscalation = errors.New("API key cannot have permissions beyond user's role")
    ErrMaxKeysReached   = errors.New("maximum number of API keys per user reached (10)")
)
