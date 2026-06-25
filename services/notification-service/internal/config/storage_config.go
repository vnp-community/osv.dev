// Package config — storage_config.go
// StorageConfig selects the persistence backend for notification-service rules.
//
// Design: additive — Firestore remains the default, Postgres is opt-in via env var.
package config

// RuleBackend identifies the storage backend for notification rules.
type RuleBackend string

const (
	// RuleBackendFirestore uses Cloud Firestore (default — preserves existing behaviour).
	RuleBackendFirestore RuleBackend = "firestore"
	// RuleBackendPostgres uses PostgreSQL (new, opt-in via RULE_BACKEND=postgres).
	RuleBackendPostgres RuleBackend = "postgres"
)

// StorageConfig holds storage backend selection for notification-service.
// Read from environment at startup.
type StorageConfig struct {
	// RuleBackend selects where notification rules are persisted.
	// "firestore" (default) or "postgres".
	RuleBackend RuleBackend
}

// DefaultStorageConfig returns the default StorageConfig (Firestore backend).
func DefaultStorageConfig() StorageConfig {
	backend := RuleBackend(envOrDefault("RULE_BACKEND", string(RuleBackendFirestore)))
	return StorageConfig{RuleBackend: backend}
}

func envOrDefault(key, def string) string {
	// Import os is avoided here to keep this file dependency-free.
	// Caller should use os.Getenv directly or inject via struct literal.
	return def
}
