package domain_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osv/shared/pkg/domain"
)

func TestWriteError_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	domain.WriteError(w, fmt.Errorf("CVE not found: %w", domain.ErrNotFound))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"NOT_FOUND"`)
}

func TestWriteError_Unauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	domain.WriteError(w, domain.ErrUnauthorized)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"UNAUTHORIZED"`)
}

func TestWriteError_InvalidInput(t *testing.T) {
	w := httptest.NewRecorder()
	domain.WriteError(w, fmt.Errorf("bad cpe param: %w", domain.ErrInvalidInput))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"INVALID_INPUT"`)
}

func TestWriteError_Forbidden(t *testing.T) {
	w := httptest.NewRecorder()
	domain.WriteError(w, domain.ErrForbidden)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWriteError_Conflict(t *testing.T) {
	w := httptest.NewRecorder()
	domain.WriteError(w, domain.ErrConflict)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestWriteError_Internal(t *testing.T) {
	w := httptest.NewRecorder()
	domain.WriteError(w, fmt.Errorf("db timeout"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), `"INTERNAL_ERROR"`)
}

func TestWriteError_WrappedErrorsIsWorks(t *testing.T) {
	// errors.Is must work through wrapping
	wrapped := fmt.Errorf("deeper: %w", fmt.Errorf("wrap: %w", domain.ErrNotFound))
	w := httptest.NewRecorder()
	domain.WriteError(w, wrapped)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWriteJSON_SetsContentType(t *testing.T) {
	w := httptest.NewRecorder()
	domain.WriteJSON(w, http.StatusOK, map[string]string{"key": "value"})
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, w.Code)
}
