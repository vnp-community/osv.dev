// Package http — settings_handler.go
// TASK-HC-009: Platform settings HTTP handlers backed by PostgreSQL platform_settings table.
// Provides GET /api/v1/admin/settings and PUT/PATCH /api/v1/admin/settings.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PlatformSettingsRepo reads/writes settings from the platform_settings table.
type PlatformSettingsRepo struct {
	pool *pgxpool.Pool
}

// NewPlatformSettingsRepo creates a settings repository.
func NewPlatformSettingsRepo(pool *pgxpool.Pool) *PlatformSettingsRepo {
	return &PlatformSettingsRepo{pool: pool}
}

// GetAll returns all settings grouped by section.
func (r *PlatformSettingsRepo) GetAll(ctx context.Context) (map[string]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT key, value, section FROM platform_settings ORDER BY section, key
	`)
	if err != nil {
		return nil, fmt.Errorf("platform_settings.GetAll: %w", err)
	}
	defer rows.Close()

	grouped := make(map[string]map[string]interface{})
	for rows.Next() {
		var key, section string
		var rawVal []byte
		if err := rows.Scan(&key, &rawVal, &section); err != nil {
			continue
		}
		if grouped[section] == nil {
			grouped[section] = make(map[string]interface{})
		}
		// key format: "section.field" → strip section prefix
		fieldName := key
		if len(section)+1 < len(key) && key[:len(section)+1] == section+"." {
			fieldName = key[len(section)+1:]
		}
		var v interface{}
		if err := json.Unmarshal(rawVal, &v); err != nil {
			v = string(rawVal) // raw fallback
		}
		grouped[section][fieldName] = v
	}
	return grouped, rows.Err()
}

// Upsert saves a single setting key-value pair.
func (r *PlatformSettingsRepo) Upsert(ctx context.Context, key string, value interface{}, updatedBy *string) error {
	rawVal, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("platform_settings.Upsert marshal: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO platform_settings (key, value, updated_at)
		VALUES ($1, $2::jsonb, NOW())
		ON CONFLICT (key) DO UPDATE
		  SET value = EXCLUDED.value,
		      updated_at = NOW(),
		      updated_by = $3
	`, key, rawVal, updatedBy)
	return err
}

// ── HTTP Handlers ─────────────────────────────────────────────────────────────

// SettingsHandler provides /api/v1/admin/settings endpoints.
type SettingsHandler struct {
	repo *PlatformSettingsRepo
}

// NewSettingsHandler creates a SettingsHandler.
func NewSettingsHandler(repo *PlatformSettingsRepo) *SettingsHandler {
	return &SettingsHandler{repo: repo}
}

// GetSettings handles GET /api/v1/admin/settings
// [FIX TASK-HC-009] Reads from platform_settings table — not hardcoded.
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "settings repository not configured")
		return
	}

	settings, err := h.repo.GetAll(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// UpdateSettings handles PUT or PATCH /api/v1/admin/settings
// [FIX TASK-HC-009] Persists settings to platform_settings table.
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "settings repository not configured")
		return
	}

	var payload map[string]map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	userID := r.Header.Get("X-User-ID")
	var updatedBy *string
	if userID != "" {
		updatedBy = &userID
	}

	for section, fields := range payload {
		for field, value := range fields {
			key := section + "." + field
			if err := h.repo.Upsert(r.Context(), key, value, updatedBy); err != nil {
				writeJSONError(w, http.StatusInternalServerError,
					fmt.Sprintf("failed to save setting %s: %v", key, err))
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "settings updated"})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
