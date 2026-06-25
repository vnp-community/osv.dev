package session

import "errors"

var (
    ErrSessionNotFound      = errors.New("session not found")
    ErrSessionRevoked       = errors.New("session has been revoked")
    ErrSessionExpired       = errors.New("session has expired")
    ErrRefreshTokenReuse    = errors.New("refresh token reuse detected — all sessions revoked")
)
