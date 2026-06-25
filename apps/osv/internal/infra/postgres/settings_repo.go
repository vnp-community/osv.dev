// Package postgres — settings_repo.go
// Implements SettingsRepository backed by PostgreSQL platform_settings table.
// NOTE: types formerly imported from gateway/bff are now defined locally to avoid
// the dependency on apps/osv/internal/gateway (deleted in gateway consolidation).
package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Platform settings domain types ──────────────────────────────────────────
// These types mirror the old gateway/bff.PlatformSettings.
// They are kept here to avoid re-importing the deleted gateway package.

// SettingsRepository is the persistence interface for platform settings.
type SettingsRepository interface {
	Get(ctx context.Context) (*PlatformSettings, error)
	Patch(ctx context.Context, patch map[string]interface{}) error
}

// PlatformSettings is the full system-wide configuration.
type PlatformSettings struct {
	General  GeneralSettings  `json:"general"`
	SMTP     SMTPSettings     `json:"smtp"`
	Security SecuritySettings `json:"security"`
	AI       AISettings       `json:"ai"`
}

// GeneralSettings holds general platform config.
type GeneralSettings struct {
	PlatformName string `json:"platform_name"`
	Organization string `json:"organization"`
	SupportEmail string `json:"support_email"`
	Timezone     string `json:"timezone"`
	DateFormat   string `json:"date_format"`
}

// SMTPSettings holds email delivery config.
type SMTPSettings struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	FromName string `json:"from_name"`
	Password string `json:"password,omitempty"`
	UseTLS   bool   `json:"use_tls"`
}

// SecuritySettings holds security policy config.
type SecuritySettings struct {
	PasswordMinLength     int  `json:"password_min_length"`
	PasswordMaxAgeDays    int  `json:"password_max_age_days"`
	SessionTimeoutMinutes int  `json:"session_timeout_minutes"`
	MaxConcurrentSessions int  `json:"max_concurrent_sessions"`
	MFARequired           bool `json:"mfa_required"`
	AllowSMSOTP           bool `json:"allow_sms_otp"`
}

// AISettings holds AI provider config.
type AISettings struct {
	ActiveProviderID string       `json:"active_provider_id"`
	Providers        []AIProvider `json:"providers"`
}

// AIProvider represents one AI backend configuration.
type AIProvider struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	BaseURL string `json:"base_url,omitempty"`
}

// ─── Repository implementation ────────────────────────────────────────────────

// SettingsRepo implements SettingsRepository backed by PostgreSQL.
type SettingsRepo struct {
	db *pgxpool.Pool
}

// NewSettingsRepo creates a new SettingsRepo.
func NewSettingsRepo(db *pgxpool.Pool) *SettingsRepo {
	if db == nil {
		return nil
	}
	return &SettingsRepo{db: db}
}

var _ SettingsRepository = (*SettingsRepo)(nil)

// Get retrieves all platform settings, returning defaults when DB is unavailable.
func (r *SettingsRepo) Get(ctx context.Context) (*PlatformSettings, error) {
	if r == nil || r.db == nil {
		return newDefaultSettings(), nil
	}
	rows, err := r.db.Query(ctx, `SELECT key, value_json FROM platform_settings`)
	if err != nil {
		return newDefaultSettings(), nil
	}
	defer rows.Close()

	settings := newDefaultSettings()
	for rows.Next() {
		var key string
		var valueJson []byte
		if err := rows.Scan(&key, &valueJson); err != nil {
			continue
		}
		switch key {
		case "general":
			json.Unmarshal(valueJson, &settings.General) //nolint:errcheck
		case "smtp":
			json.Unmarshal(valueJson, &settings.SMTP) //nolint:errcheck
		case "notifications":
			if settings.SMTP.Host == "" {
				var legacy struct {
					SMTPHost string `json:"smtp_host"`
					SMTPPort int    `json:"smtp_port"`
					SMTPFrom string `json:"smtp_from"`
				}
				if json.Unmarshal(valueJson, &legacy) == nil {
					settings.SMTP.Host = legacy.SMTPHost
					settings.SMTP.Port = legacy.SMTPPort
					settings.SMTP.FromName = legacy.SMTPFrom
				}
			}
		case "ai":
			json.Unmarshal(valueJson, &settings.AI) //nolint:errcheck
		case "security":
			json.Unmarshal(valueJson, &settings.Security) //nolint:errcheck
		}
	}
	return settings, nil
}

// Patch updates platform settings in the DB.
func (r *SettingsRepo) Patch(ctx context.Context, patch map[string]interface{}) error {
	if r == nil || r.db == nil {
		return nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	validSections := map[string]bool{
		"general": true, "smtp": true, "notifications": true,
		"ai": true, "security": true,
	}
	for key, value := range patch {
		if !validSections[key] {
			continue
		}
		var existingJson []byte
		err := tx.QueryRow(ctx, `SELECT value_json FROM platform_settings WHERE key = $1`, key).Scan(&existingJson)
		existing := map[string]interface{}{}
		if err == nil {
			json.Unmarshal(existingJson, &existing) //nolint:errcheck
		}
		updates, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		for k, v := range updates {
			existing[k] = v
		}
		newJson, _ := json.Marshal(existing)
		_, err = tx.Exec(ctx, `
			INSERT INTO platform_settings (key, value_json, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (key) DO UPDATE SET value_json = $2, updated_at = NOW()
		`, key, newJson)
		if err != nil {
			continue
		}
	}
	return tx.Commit(ctx)
}

// newDefaultSettings returns sensible defaults when DB is not available.
func newDefaultSettings() *PlatformSettings {
	return &PlatformSettings{
		General: GeneralSettings{
			PlatformName: "OSV Platform",
			Timezone:     "UTC",
			DateFormat:   "YYYY-MM-DD",
		},
		SMTP: SMTPSettings{Port: 587, UseTLS: true},
		Security: SecuritySettings{
			PasswordMinLength:     12,
			PasswordMaxAgeDays:    90,
			SessionTimeoutMinutes: 60,
			MaxConcurrentSessions: 3,
		},
		AI: AISettings{
			ActiveProviderID: "ollama",
			Providers:        []AIProvider{},
		},
	}
}
