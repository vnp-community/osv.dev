// Package ingest — usecase.go
// OSV Ingest Pipeline: validate → normalize → upsert → publish event.
//
// ADDITIVE: existing syncsource/, converter/ unchanged.
// This pipeline is called by HTTP handler (POST /api/v1/ingest/osv)
// or batch job (GCS bucket trigger).
package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/data-service/internal/domain/entity"
	natspkg "github.com/osv/data-service/internal/infra/messaging/nats"
	"github.com/osv/shared/pkg/osvschema"
)

// IngestRepository is the minimal interface required by the ingest pipeline.
// mongoCVERepo implements this; MongoDBCVERepository interface is not modified.
type IngestRepository interface {
	FindByID(ctx context.Context, id string) (*entity.CVE, error)
	Upsert(ctx context.Context, rawRecord []byte, id, source string) error
}

// UseCase is the OSV ingest pipeline.
type UseCase struct {
	repo      IngestRepository
	publisher *natspkg.CVEEventPublisher
	log       zerolog.Logger
}

// NewUseCase creates a new ingest UseCase.
func NewUseCase(
	repo IngestRepository,
	publisher *natspkg.CVEEventPublisher,
	log zerolog.Logger,
) *UseCase {
	return &UseCase{
		repo:      repo,
		publisher: publisher,
		log:       log,
	}
}

// Execute processes a single raw OSV JSON record.
// Steps: parse → validate → normalize ID → check modified → upsert → publish.
func (uc *UseCase) Execute(ctx context.Context, req IngestRequest) (*IngestResult, error) {
	// Step 1: Parse JSON into OSV schema
	var osv osvschema.Vulnerability
	if err := json.Unmarshal(req.RawRecord, &osv); err != nil {
		return nil, &IngestError{Source: req.Source, Err: fmt.Errorf("parse: %w", err)}
	}

	// Step 2: Validate required fields
	if err := validateOSV(&osv); err != nil {
		return nil, &IngestError{Source: req.Source, Err: fmt.Errorf("validate: %w", err)}
	}

	// Step 3: Normalize ID — prefer CVE-XXXX-XXXX from aliases
	normalizedID := normalizeID(osv.ID, osv.Aliases)

	// Step 4: Check if the record already exists (decide action)
	action := "created"
	existing, err := uc.repo.FindByID(ctx, normalizedID)
	if err == nil && existing != nil {
		// Skip if our record is not newer than what we already have
		if !osv.Modified.After(existing.Modified) {
			return &IngestResult{CVEID: normalizedID, Action: "skipped", Source: req.Source}, nil
		}
		action = "updated"
	}

	// Step 5: Upsert the raw record into MongoDB
	if err := uc.repo.Upsert(ctx, req.RawRecord, normalizedID, req.Source); err != nil {
		return nil, &IngestError{Source: req.Source, Err: fmt.Errorf("upsert: %w", err)}
	}

	uc.log.Info().
		Str("cve_id", normalizedID).
		Str("source", req.Source).
		Str("action", action).
		Msg("ingest: record processed")

	// Step 6: Publish NATS event (non-blocking, best-effort)
	if uc.publisher != nil {
		switch action {
		case "created":
			uc.publisher.PublishImported(ctx, natspkg.CVEImportedEvent{
				ID:        normalizedID,
				Source:    req.Source,
				SourceURL: req.SourceURL,
				SyncedAt:  time.Now().UTC(),
			})
		case "updated":
			uc.publisher.PublishUpdated(ctx, natspkg.CVEUpdatedEvent{
				ID:        normalizedID,
				Source:    req.Source,
				UpdatedAt: time.Now().UTC(),
			})
		}
	}

	return &IngestResult{CVEID: normalizedID, Action: action, Source: req.Source}, nil
}

// ExecuteBatch processes multiple OSV records.
// Errors are collected per-record; processing continues even on failure.
func (uc *UseCase) ExecuteBatch(ctx context.Context, reqs []IngestRequest) ([]IngestResult, []error) {
	results := make([]IngestResult, 0, len(reqs))
	var errs []error

	for _, req := range reqs {
		result, err := uc.Execute(ctx, req)
		if err != nil {
			errs = append(errs, err)
			uc.log.Warn().Err(err).Str("source", req.Source).Msg("ingest batch: record skipped")
			continue
		}
		results = append(results, *result)
	}

	uc.log.Info().
		Int("processed", len(results)).
		Int("errors", len(errs)).
		Msg("ingest: batch complete")

	return results, errs
}

// ── helpers ──────────────────────────────────────────────────────────────────

// validateOSV performs minimal OSV schema validation.
func validateOSV(v *osvschema.Vulnerability) error {
	if v.ID == "" {
		return fmt.Errorf("missing required field: id")
	}
	if v.Modified.IsZero() {
		return fmt.Errorf("missing required field: modified")
	}
	return nil
}

// normalizeID prefers a CVE-XXXX-XXXXX alias when available.
// Falls back to the original OSV/GHSA ID if no CVE alias exists.
func normalizeID(id string, aliases []string) string {
	for _, alias := range aliases {
		if len(alias) > 4 && alias[:4] == "CVE-" {
			return alias
		}
	}
	return id
}
