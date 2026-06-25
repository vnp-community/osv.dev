// Package searchbycpe provides the use case for searching CVEs by CPE string.
package searchbycpe

import (
	"context"
	"fmt"
	"strings"

	repo "github.com/osv/data-service/internal/domain/repository"
)

// Input holds the parameters for a CPE-based CVE search.
type Input struct {
	// CPE string (CPE 2.2 URI or CPE 2.3 FS), e.g. "cpe:2.3:a:apache:log4j:2.14.1:*:*:*:*:*:*:*"
	CPE string

	// Lax: if true, match vendor+product only (ignore version component).
	Lax bool

	// StrictVendorProduct: use exact vendor+product match.
	StrictVendorProduct bool

	// Limit caps the number of results (0 = default 100).
	Limit int

	// Skip is the pagination offset (0 = first page).
	Skip int

	// Mode is a unified mode selector: "standard" | "lax" | "strict".
	// If Mode is set, it overrides the Lax/StrictVendorProduct booleans.
	// Backward compat: Lax=true and StrictVendorProduct=true still work.
	Mode string

	// Enrich options — parsed but not yet applied (Phase 1 MVP).
	EnrichRanking bool // ?enrich=ranking
	EnrichCAPEC   bool // ?enrich=capec
	EnrichVIA4    bool // ?enrich=via4
}

// SearchResult wraps CVE results with pagination metadata and optional notices.
type SearchResult struct {
	Results interface{} `json:"results"`
	Skip    int         `json:"skip"`
	Limit   int         `json:"limit"`
	Notices []string    `json:"notices,omitempty"`
}

// UseCase searches for CVEs matching a CPE string.
type UseCase struct {
	repo repo.MongoDBCVERepository
}

// New creates a SearchByCPE use case.
func New(r repo.MongoDBCVERepository) *UseCase {
	return &UseCase{repo: r}
}

// Execute searches for CVEs matching the given CPE.
// Supports mode=standard|lax|strict (with backward compat for lax=true, strict=true).
// Returns SearchResult with optional notices for lax mode.
func (uc *UseCase) Execute(ctx context.Context, in Input) (interface{}, error) {
	if in.CPE == "" {
		return nil, fmt.Errorf("cpe parameter is required")
	}

	// Apply mode override (unified param takes precedence over individual booleans)
	switch strings.ToLower(in.Mode) {
	case "lax":
		in.Lax = true
		in.StrictVendorProduct = false
	case "strict":
		in.StrictVendorProduct = true
		in.Lax = false
	case "standard", "":
		// keep existing Lax/StrictVendorProduct values (backward compat)
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	results, err := uc.repo.FindByCPE(ctx, repo.CVESearchOptions{
		CPE:                 in.CPE,
		Lax:                 in.Lax,
		StrictVendorProduct: in.StrictVendorProduct,
		Limit:               limit,
		Skip:                in.Skip,
	})
	if err != nil {
		return nil, err
	}

	response := &SearchResult{
		Results: results,
		Skip:    in.Skip,
		Limit:   limit,
	}

	// Add notice for Lax mode to inform callers about relaxed matching
	if in.Lax {
		response.Notices = []string{
			fmt.Sprintf("Lax mode active: matching vendor+product only for CPE '%s'. Version components are ignored.", in.CPE),
		}
	}

	return response, nil
}
