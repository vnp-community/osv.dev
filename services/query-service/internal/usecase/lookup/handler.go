// Package query contains application-layer query handlers.
// get_vuln_by_id returns a single vulnerability by ID, performing alias resolution
// and hydrating the full OSV record from GCS on cache miss.
package get_vuln_by_id

import (
	"context"
	"fmt"

	stderrors "errors"
	pkgerrors "github.com/osv/shared/pkg/errors"
	"github.com/osv/shared/pkg/osvschema"
	"github.com/osv/query-service/internal/domain/entity"
	"github.com/osv/query-service/internal/domain/repository"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

// AliasResolver resolves a vulnerability ID to its canonical form.
type AliasResolver interface {
	ResolveAlias(ctx context.Context, id string) (canonicalID string, aliases []string, err error)
}

// BlobStore fetches the full OSV JSON from GCS.
type BlobStore interface {
	GetByGCSPath(ctx context.Context, path string) (*osvschema.Vulnerability, error)
}

// Query requests a single vulnerability by any known ID.
type Query struct {
	ID string
}

// Result holds the resolved vulnerability with full OSV data.
type Result struct {
	Vulnerability *entity.VulnerabilityReadModel
	CanonicalID   string
	Aliases       []string
}

// Handler handles GetVulnByID queries.
type Handler struct {
	reader   repository.VulnerabilityReader
	cache    repository.VulnerabilityCache
	blob     BlobStore
	aliases  AliasResolver
	tracer   trace.Tracer
	log      zerolog.Logger
}

// NewHandler creates a new GetVulnByID handler.
func NewHandler(
	reader repository.VulnerabilityReader,
	cache repository.VulnerabilityCache,
	blob BlobStore,
	aliases AliasResolver,
	tracer trace.Tracer,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		reader:  reader,
		cache:   cache,
		blob:    blob,
		aliases: aliases,
		tracer:  tracer,
		log:     log,
	}
}

// Handle resolves and returns a single vulnerability.
// Flow: alias resolve → cache check → Firestore → GCS hydrate → cache set
func (h *Handler) Handle(ctx context.Context, q Query) (*Result, error) {
	ctx, span := h.tracer.Start(ctx, "GetVulnByID")
	defer span.End()

	// 1. Resolve alias → canonical ID
	canonicalID, aliases, err := h.aliases.ResolveAlias(ctx, q.ID)
	if err != nil {
		// Alias service unavailable — fall back to direct lookup
		h.log.Warn().Err(err).Str("id", q.ID).Msg("alias resolution failed, using raw id")
		canonicalID = q.ID
	}

	// 2. Cache check (L1 + L2)
	cacheKey := "vuln:" + canonicalID
	if cached, err := h.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		return &Result{
			Vulnerability: cached,
			CanonicalID:   canonicalID,
			Aliases:       aliases,
		}, nil
	}

	// 3. Firestore lookup
	vuln, err := h.reader.GetByID(ctx, canonicalID)
	if err != nil {
		if stderrors.Is(err, pkgerrors.ErrNotFound) {
			return nil, pkgerrors.ErrNotFound
		}
		return nil, fmt.Errorf("get vuln %s: %w", canonicalID, err)
	}

	// 4. Hydrate full OSV record from GCS (if GCS path available)
	if vuln.GCSPath != "" && vuln.FullRecord == nil {
		full, err := h.blob.GetByGCSPath(ctx, vuln.GCSPath)
		if err != nil {
			h.log.Warn().Err(err).Str("gcs_path", vuln.GCSPath).Msg("GCS hydrate failed, returning partial record")
		} else {
			vuln.FullRecord = full
		}
	}

	// 5. Cache the result (TTL managed by cache adapter)
	if cacheErr := h.cache.Set(ctx, cacheKey, vuln, 0); cacheErr != nil {
		h.log.Warn().Err(cacheErr).Msg("cache set failed")
	}

	return &Result{
		Vulnerability: vuln,
		CanonicalID:   canonicalID,
		Aliases:       aliases,
	}, nil
}
