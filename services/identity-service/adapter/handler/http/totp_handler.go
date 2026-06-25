// Package http — totp_handler.go
// TOTPHandler handles TOTP management HTTP endpoints for the identity-service.
//
// Routes (add to router.go):
//   POST   /api/v1/auth/totp/setup   → Setup()   — generate QR + pending secret
//   POST   /api/v1/auth/totp/verify  → Verify()  — validate code + activate MFA
//   DELETE /api/v1/auth/totp         → Disable()  — disable MFA (requires password)
package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	domainerr "github.com/osv/identity-service/internal/domain/error"
	"github.com/osv/identity-service/internal/usecase/totp"
)

// TOTPHandler handles TOTP management HTTP requests.
type TOTPHandler struct {
	uc  *totp.UseCase
	log zerolog.Logger
}

// NewTOTPHandler creates a new TOTPHandler.
func NewTOTPHandler(uc *totp.UseCase, log zerolog.Logger) *TOTPHandler {
	return &TOTPHandler{
		uc:  uc,
		log: log,
	}
}

// Setup handles POST /api/v1/auth/totp/setup
// Returns: { "qr_code_url": "otpauth://...", "secret": "BASE32...", "backup_codes": ["..."] }
func (h *TOTPHandler) Setup(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromHeader(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resp, err := h.uc.Setup(r.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Str("user_id", userID.String()).Msg("totp.Setup")
		writeError(w, http.StatusInternalServerError, "failed to setup TOTP")
		return
	}

	// CR-001: expose backup_codes — shown ONCE at setup
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"qr_code_url":  resp.OTPAuthURI,
		"secret":       resp.Secret,
		"backup_codes": resp.BackupCodes,
	})
}

// Verify handles POST /api/v1/auth/totp/verify
// Body: { "code": "123456" }
func (h *TOTPHandler) Verify(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromHeader(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		writeError(w, http.StatusBadRequest, "invalid request: 'code' required")
		return
	}

	err := h.uc.Confirm(r.Context(), userID, req.Code)
	if err != nil {
		if errors.Is(err, domainerr.ErrTOTPNotSetup) {
			writeError(w, http.StatusBadRequest, "TOTP setup not initiated")
		} else if errors.Is(err, domainerr.ErrInvalidTOTPCode) {
			writeError(w, http.StatusBadRequest, "invalid TOTP code")
		} else {
			h.log.Error().Err(err).Msg("totp.Verify")
			writeError(w, http.StatusInternalServerError, "TOTP verification failed")
		}
		return
	}

	writeJSON(w, http.StatusNoContent, nil)
}

// Disable handles DELETE /api/v1/auth/totp
// Body: { "code": "current-totp-code" }
func (h *TOTPHandler) Disable(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromHeader(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		writeError(w, http.StatusBadRequest, "invalid request: 'code' required")
		return
	}

	err := h.uc.Disable(r.Context(), userID, req.Code)
	if err != nil {
		if errors.Is(err, domainerr.ErrMFANotEnabled) {
			writeError(w, http.StatusBadRequest, "MFA is not enabled")
		} else if errors.Is(err, domainerr.ErrInvalidCredentials) {
			writeError(w, http.StatusForbidden, "invalid password")
		} else {
			h.log.Error().Err(err).Msg("totp.Disable")
			writeError(w, http.StatusInternalServerError, "failed to disable TOTP")
		}
		return
	}

	writeJSON(w, http.StatusNoContent, nil)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// userIDFromHeader extracts the user ID from the X-User-ID header (set by gateway JWT middleware).
func userIDFromHeader(r *http.Request) (uuid.UUID, bool) {
	raw := r.Header.Get("X-User-ID")
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}
