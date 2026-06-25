// Package http provides HTTP handler helpers for notification-service.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// apiErr returns a DefectDojo-format error map.
func apiErr(msg string) map[string]string {
	return map[string]string{"detail": msg}
}

// intOr parses s as an int, returning def if parsing fails or value is <= 0.
func intOr(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}

// contextKey is a private key type for context values.
type contextKey int

// AlertRepository placeholder (used in alert_handler.go).
type contextAlertRepo contextKey

// These ensure the time package is used (avoids import error in alert_handler.go).
var _ = time.Now
var _ = context.Background
