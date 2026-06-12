package importdb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/osv/data-service/internal/domain/repository"
)

// Input for ImportDB use case.
type Input struct {
	// FileBytes is the raw import file (JSON format).
	FileBytes []byte

	// VerifyChecksum enables SHA-256 checksum verification before import.
	// For full PGP signature verification, integrate ProtonMail/go-crypto
	// directly in the adapter layer.
	VerifyChecksum bool

	// ExpectedSHA256 is the expected SHA-256 hex string for FileBytes.
	// Required when VerifyChecksum is true.
	ExpectedSHA256 string
}

// Output summarizes the import operation.
type Output struct {
	RecordsImported int
	Message         string
}

// UseCase imports a CVE database file into the local database.
// Supports JSON format with optional SHA-256 checksum verification.
type UseCase struct {
	dbAdmin repository.DBAdminRepository
}

// New creates a new ImportDB use case.
func New(dbAdmin repository.DBAdminRepository) *UseCase {
	return &UseCase{dbAdmin: dbAdmin}
}

// Execute imports the database file.
func (uc *UseCase) Execute(ctx context.Context, in Input) (*Output, error) {
	if len(in.FileBytes) == 0 {
		return nil, fmt.Errorf("empty file data")
	}

	// ── Step 1: SHA256 Checksum Verification ──
	if in.VerifyChecksum {
		if in.ExpectedSHA256 == "" {
			return nil, fmt.Errorf("expected SHA256 checksum required but not provided")
		}
		h := sha256.Sum256(in.FileBytes)
		got := hex.EncodeToString(h[:])
		if got != in.ExpectedSHA256 {
			return nil, fmt.Errorf("sha256 mismatch: got %s, want %s", got, in.ExpectedSHA256)
		}
	}

	// ── Step 2: Import JSON ──
	if err := uc.dbAdmin.ImportJSON(ctx, in.FileBytes); err != nil {
		return nil, fmt.Errorf("import JSON: %w", err)
	}

	return &Output{
		Message: "import completed successfully",
	}, nil
}
