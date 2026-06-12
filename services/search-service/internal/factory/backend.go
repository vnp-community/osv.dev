// Package factory provides backend selection for search-service.
package factory

import "os"

// BackendType identifies the search backend.
type BackendType string

const (
	BackendOpenSearch BackendType = "opensearch"
	BackendPostgres   BackendType = "postgres"
	BackendMongo      BackendType = "mongo"
	BackendAuto       BackendType = "auto" // tries OpenSearch, falls back to Postgres
)

// FromEnv reads SEARCH_BACKEND env var (default: "auto").
func FromEnv() BackendType {
	b := os.Getenv("SEARCH_BACKEND")
	switch BackendType(b) {
	case BackendOpenSearch, BackendPostgres, BackendMongo:
		return BackendType(b)
	default:
		return BackendAuto
	}
}
