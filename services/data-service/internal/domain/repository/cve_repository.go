// Package repository defines the CVE repository interface for cve-service.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/osv/data-service/internal/domain/entity"
)

// ErrNotFound is returned when a CVE document is not found.
var ErrNotFound = errors.New("not found")

// CVESearchOptions controls CVE search behavior for CPE-based queries.
type CVESearchOptions struct {
	// CPE is the CPE string to search for (2.2 URI or 2.3 FS).
	CPE string

	// Lax: if true, match by vendor+product only (ignore version).
	Lax bool

	// StrictVendorProduct: exact vendor+product match (no regex).
	StrictVendorProduct bool

	// Limit caps the number of results. 0 = no limit.
	Limit int

	// Skip is the pagination offset (number of documents to skip). Default 0.
	Skip int
}

// MongoDBCVERepository defines the MongoDB data access interface for cve-search CVE documents.
// This is separate from CVERepository (PostgreSQL-backed binary tool interface).
type MongoDBCVERepository interface {
	// FindByID looks up a single CVE by its ID (e.g. "CVE-2021-44228").
	FindByID(ctx context.Context, id string) (*entity.CVE, error)

	// FindByCPE searches for CVEs matching a CPE string.
	FindByCPE(ctx context.Context, opts CVESearchOptions) ([]*entity.CVE, error)

	// FindLast returns the n most recently modified CVEs.
	FindLast(ctx context.Context, n int) ([]*entity.CVE, error)

	// FindRecent returns CVEs modified after since, sorted by modified desc.
	FindRecent(ctx context.Context, since time.Time, limit int) ([]*entity.CVE, error)
}
