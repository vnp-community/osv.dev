// Package domain — HTTP helpers for cve-search services.
package domain

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ErrorResponse is the canonical API error response format.
// {"error": {"code": "NOT_FOUND", "message": "CVE CVE-2021-44228 not found"}}
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains the machine-readable error code and human-readable message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON writes v as JSON with the given HTTP status code.
// Sets Content-Type: application/json automatically.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// WriteError maps domain sentinel errors to HTTP status codes and writes
// a structured JSON error response.
//
// Mapping:
//   - ErrNotFound     → 404 NOT_FOUND
//   - ErrInvalidInput → 400 INVALID_INPUT
//   - ErrUnauthorized → 401 UNAUTHORIZED
//   - ErrForbidden    → 403 FORBIDDEN
//   - ErrConflict     → 409 CONFLICT
//   - (other)         → 500 INTERNAL_ERROR
func WriteError(w http.ResponseWriter, err error) {
	code, status := "INTERNAL_ERROR", http.StatusInternalServerError

	switch {
	case errors.Is(err, ErrNotFound):
		code, status = "NOT_FOUND", http.StatusNotFound
	case errors.Is(err, ErrInvalidInput):
		code, status = "INVALID_INPUT", http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		code, status = "UNAUTHORIZED", http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		code, status = "FORBIDDEN", http.StatusForbidden
	case errors.Is(err, ErrConflict):
		code, status = "CONFLICT", http.StatusConflict
	}

	WriteJSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: err.Error(),
		},
	})
}
