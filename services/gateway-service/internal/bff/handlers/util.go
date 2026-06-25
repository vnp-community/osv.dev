// Package http — shared utilities for BFF handlers.
// proxyJSON, writeAPIError, respondJSON used across all CR-UI-xxx handlers.
package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// APIError is the standard error envelope (CR-UI README §Chuẩn lỗi).
type APIError struct {
	Error   string      `json:"error"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

// writeAPIError writes a standard JSON error response.
func writeAPIError(w http.ResponseWriter, status int, code, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIError{
		Error:   code,
		Message: message,
		Details: details,
	})
}

// respondJSON writes a JSON response with given status and body.
func respondJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// proxyJSON sends a JSON request to the given URL and returns the raw response.
// Caller is responsible for closing resp.Body.
func proxyJSON(client *http.Client, url, method string, body interface{}) (*http.Response, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, code int, message string) {
	respondJSON(w, code, map[string]string{"error": message})
}

// mustJSON marshals v to JSON, returning empty bytes on error.
func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
