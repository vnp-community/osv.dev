// Package config provides storage backend selection configuration.
// Controls which storage implementation is used for each repository.
// Pattern: env-driven selector — existing Firestore implementation is default.
package config

import "os"

// AliasGroupBackend selects the persistence backend for AliasGroupRepository.
type AliasGroupBackend string

const (
	// AliasGroupBackendFirestore uses Google Cloud Firestore (legacy default).
	// This preserves all existing behaviour without code changes.
	AliasGroupBackendFirestore AliasGroupBackend = "firestore"

	// AliasGroupBackendPostgres uses PostgreSQL (recommended for local dev).
	// Requires migration 005_create_alias_groups.up.sql to be applied first.
	AliasGroupBackendPostgres AliasGroupBackend = "postgres"
)

// StorageConfig holds backend selection for data-service storage domains.
type StorageConfig struct {
	// AliasGroupBackend selects where alias groups are persisted.
	// Env: ALIAS_GROUP_BACKEND
	// Default: "postgres" (changed from "firestore" to avoid GCP dependency in dev).
	AliasGroupBackend AliasGroupBackend
}

// LoadStorageConfig loads StorageConfig from environment variables.
func LoadStorageConfig() StorageConfig {
	backend := AliasGroupBackend(envOr("ALIAS_GROUP_BACKEND", string(AliasGroupBackendPostgres)))
	return StorageConfig{
		AliasGroupBackend: backend,
	}
}

// envOr returns the value of the OS environment variable named key,
// or defaultVal if the variable is not set or empty.
// FIXED: previous implementation was a placeholder that always returned defaultVal.
func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
