package exportdb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/osv/data-service/internal/domain/repository"
)

// Input for ExportDB use case.
type Input struct {
	// Format selects export format: "json" (default).
	Format string

	// Year filters CVEs by year (0 = all years).
	Year int

	// IncludeChecksum appends SHA-256 checksum to response.
	IncludeChecksum bool
}

// Output holds the exported database file.
type Output struct {
	// Bytes is the exported data.
	Bytes []byte

	// SHA256 is the hex checksum of Bytes (if IncludeChecksum was set).
	SHA256 string

	// ContentType is the MIME type of Bytes (e.g. "application/json").
	ContentType string
}

// UseCase exports the local CVE database to a portable format.
type UseCase struct {
	dbAdmin repository.DBAdminRepository
}

// New creates a new ExportDB use case.
func New(dbAdmin repository.DBAdminRepository) *UseCase {
	return &UseCase{dbAdmin: dbAdmin}
}

// Execute exports the database.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	if in.Format == "" {
		in.Format = "json"
	}
	if in.Format != "json" {
		return nil, fmt.Errorf("unsupported export format: %q (only \"json\" supported)", in.Format)
	}

	// ── Step 1: Export JSON ──
	data, err := uc.dbAdmin.ExportJSON(ctx, in.Year)
	if err != nil {
		return nil, fmt.Errorf("export JSON: %w", err)
	}

	out := &Output{
		Bytes:       data,
		ContentType: "application/json",
	}

	// ── Step 2: Compute checksum ──
	if in.IncludeChecksum {
		h := sha256.Sum256(data)
		out.SHA256 = hex.EncodeToString(h[:])
	}

	return out, nil
}
