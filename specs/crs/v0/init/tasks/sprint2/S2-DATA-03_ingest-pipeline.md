# S2-DATA-03 — Thêm OSV Ingest Pipeline (data-service)


## ✅ Execution Status: COMPLETED
## Metadata
- **Task ID**: S2-DATA-03
- **Service**: data-service
- **Sprint**: 2 (P1)
- **Ước tính**: 3-4 giờ
- **Dependencies**: S1-DATA-02 (CVE event publisher)
- **Spec nguồn**: `specs/develop/02_data-service-upgrade.md` § "P1 — Thêm: OSV Ingest Pipeline"

## Context

```bash
# Đọc converter/domain để biết OSV record structure:
cat services/data-service/internal/converter/nvd/converter.go | head -50
cat services/data-service/internal/converter/cve5/converter.go | head -50

# Đọc shared/pkg OSV schema:
ls services/shared/pkg/osvschema/
cat services/shared/pkg/osvschema/*.go 2>/dev/null | head -80

# Đọc existing sync usecase:
cat services/data-service/internal/usecase/syncsource/sync_source.go

# Đọc MongoDB repo:
cat services/data-service/internal/infra/mongo/cve_repo.go
```

## Goal

Tạo `usecase/ingest/` — pipeline nhận 1 raw OSV record byte slice, validate, normalize, upsert vào MongoDB, và publish NATS event.

## Files to Create

### File 1: `services/data-service/internal/usecase/ingest/dto.go`

```go
package ingest

import "time"

// IngestRequest is the input to the ingest pipeline.
type IngestRequest struct {
	RawRecord []byte  // Raw JSON bytes of OSV record
	Source    string  // Source identifier: "nvd", "ghsa", "pypi", "go-vulndb", etc.
	SourceURL string  // URL where record was fetched from (for audit)
	FetchedAt time.Time
}

// IngestResult is the output of the ingest pipeline.
type IngestResult struct {
	CVEID    string `json:"cve_id"`    // Normalized CVE/GHSA ID
	Action   string `json:"action"`    // "created" | "updated" | "skipped"
	Source   string `json:"source"`
}

// IngestError captures per-record errors.
type IngestError struct {
	Source string
	Err    error
}

func (e *IngestError) Error() string {
	return fmt.Sprintf("ingest[%s]: %v", e.Source, e.Err)
}
```

### File 2: `services/data-service/internal/usecase/ingest/usecase.go`

```go
package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/osv/data-service/internal/domain/repository"
	nats_infra "github.com/osv/data-service/internal/infra/messaging/nats"
	"github.com/osv/shared/pkg/osvschema"
)

// UseCase is the OSV ingest pipeline.
// Accepts raw OSV JSON → validate → normalize → upsert → publish event.
type UseCase struct {
	mongoRepo repository.MongoDBCVERepository  // existing interface
	publisher *nats_infra.CVEEventPublisher    // from S1-DATA-02
	log       zerolog.Logger
}

// NewUseCase creates a new ingest UseCase.
func NewUseCase(
	mongoRepo repository.MongoDBCVERepository,
	publisher *nats_infra.CVEEventPublisher,
	log zerolog.Logger,
) *UseCase {
	return &UseCase{
		mongoRepo: mongoRepo,
		publisher: publisher,
		log:       log,
	}
}

// Execute processes a single OSV record.
func (uc *UseCase) Execute(ctx context.Context, req IngestRequest) (*IngestResult, error) {
	// Step 1: Parse JSON
	var osvRecord osvschema.Vulnerability  // Use shared OSV schema
	if err := json.Unmarshal(req.RawRecord, &osvRecord); err != nil {
		return nil, &IngestError{Source: req.Source, Err: fmt.Errorf("parse: %w", err)}
	}

	// Step 2: Basic validation
	if err := validateOSVRecord(&osvRecord); err != nil {
		return nil, &IngestError{Source: req.Source, Err: fmt.Errorf("validate: %w", err)}
	}

	// Step 3: Normalize ID (prefer CVE-XXXX-XXXXX format if available)
	normalizedID := normalizeID(osvRecord.ID, osvRecord.Aliases)

	// Step 4: Check if exists (for action determination)
	existing, _ := uc.mongoRepo.FindByID(ctx, normalizedID)
	action := "created"
	if existing != nil {
		// Compare modified dates to decide if update is needed
		if !osvRecord.Modified.After(existing.Modified) {
			return &IngestResult{CVEID: normalizedID, Action: "skipped", Source: req.Source}, nil
		}
		action = "updated"
	}

	// Step 5: Upsert into MongoDB
	if err := uc.mongoRepo.Upsert(ctx, req.RawRecord, normalizedID, req.Source); err != nil {
		return nil, &IngestError{Source: req.Source, Err: fmt.Errorf("upsert: %w", err)}
	}

	uc.log.Info().
		Str("cve_id", normalizedID).
		Str("source", req.Source).
		Str("action", action).
		Msg("ingest: record processed")

	// Step 6: Publish NATS event (non-blocking)
	if uc.publisher != nil {
		switch action {
		case "created":
			uc.publisher.PublishCreated(ctx, nats_infra.CVECreatedEvent{
				ID:       normalizedID,
				Source:   req.Source,
				SyncedAt: time.Now(),
			})
		case "updated":
			uc.publisher.PublishUpdated(ctx, nats_infra.CVEUpdatedEvent{
				ID:        normalizedID,
				UpdatedAt: time.Now(),
			})
		}
	}

	return &IngestResult{CVEID: normalizedID, Action: action, Source: req.Source}, nil
}

// ExecuteBatch processes multiple OSV records (used for bulk sync jobs).
func (uc *UseCase) ExecuteBatch(ctx context.Context, reqs []IngestRequest) ([]IngestResult, []error) {
	var results []IngestResult
	var errs []error

	for _, req := range reqs {
		result, err := uc.Execute(ctx, req)
		if err != nil {
			errs = append(errs, err)
			uc.log.Warn().Err(err).Msg("ingest batch: record skipped")
			continue
		}
		results = append(results, *result)
	}

	return results, errs
}

// validateOSVRecord performs basic OSV schema validation.
func validateOSVRecord(v *osvschema.Vulnerability) error {
	if v.ID == "" {
		return fmt.Errorf("missing id field")
	}
	if v.Modified.IsZero() {
		return fmt.Errorf("missing modified field")
	}
	return nil
}

// normalizeID returns the CVE ID if present in aliases, otherwise the original ID.
func normalizeID(id string, aliases []string) string {
	// Prefer CVE-XXXX-XXXXX format
	for _, alias := range aliases {
		if len(alias) > 4 && alias[:4] == "CVE-" {
			return alias
		}
	}
	return id
}
```

### File 3: `services/data-service/internal/infra/messaging/nats/ingest_subscriber.go`

```go
package nats

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"

	"github.com/osv/data-service/internal/usecase/ingest"
)

// IngestTask represents a request to ingest an OSV record (received via NATS).
type IngestTask struct {
	Source    string `json:"source"`
	SourceURL string `json:"source_url"`
	Record    []byte `json:"record"`  // raw OSV JSON
}

// IngestSubscriber listens for ingest tasks on NATS and triggers the ingest UC.
type IngestSubscriber struct {
	js        nats.JetStreamContext
	ingestUC  *ingest.UseCase
	log       zerolog.Logger
}

const SubjectIngestTask = "data.sync.update"

// NewIngestSubscriber creates a new IngestSubscriber.
func NewIngestSubscriber(
	js nats.JetStreamContext,
	ingestUC *ingest.UseCase,
	log zerolog.Logger,
) *IngestSubscriber {
	return &IngestSubscriber{js: js, ingestUC: ingestUC, log: log}
}

// Start subscribes to data.sync.update and processes ingest tasks.
func (s *IngestSubscriber) Start(ctx context.Context) error {
	_, err := s.js.Subscribe(SubjectIngestTask, func(msg *nats.Msg) {
		var task IngestTask
		if err := json.Unmarshal(msg.Data, &task); err != nil {
			s.log.Warn().Err(err).Msg("ingest_subscriber: bad payload")
			msg.Nak()
			return
		}

		result, err := s.ingestUC.Execute(ctx, ingest.IngestRequest{
			RawRecord: task.Record,
			Source:    task.Source,
			SourceURL: task.SourceURL,
		})

		if err != nil {
			s.log.Error().Err(err).Str("source", task.Source).Msg("ingest_subscriber: ingest failed")
			msg.Nak()
			return
		}

		s.log.Info().
			Str("cve_id", result.CVEID).
			Str("action", result.Action).
			Msg("ingest_subscriber: processed")
		msg.Ack()
	}, nats.Durable("data-ingest-worker"))

	if err != nil {
		return fmt.Errorf("subscribe %s: %w", SubjectIngestTask, err)
	}

	<-ctx.Done()
	return ctx.Err()
}
```

## Files to Extend

### Extend: `services/data-service/internal/domain/repository/cve_repository.go`

Thêm `Upsert` method vào interface nếu chưa có:

```go
type MongoDBCVERepository interface {
    // ... existing methods ...
    Upsert(ctx context.Context, rawJSON []byte, id string, source string) error
    FindByID(ctx context.Context, id string) (*CVERecord, error)
}
```

### Extend: `services/data-service/cmd/server/main.go`

```go
// Khởi tạo ingest UC:
ingestUC := ingest.NewUseCase(mongoRepo, cvePublisher, logger)

// Khởi tạo subscriber:
ingestSub := nats_infra.NewIngestSubscriber(js, ingestUC, logger)
go func() {
    if err := ingestSub.Start(ctx); err != nil {
        log.Error().Err(err).Msg("ingest subscriber stopped")
    }
}()
```

## Verification

```bash
cd services/data-service && go build ./...

# Test ingest via NATS:
nats pub data.sync.update '{"source":"test","source_url":"","record":"{\"id\":\"CVE-2024-12345\",\"modified\":\"2024-01-01T00:00:00Z\"}"}'
# Check logs: should see "ingest: record processed" with action="created"
```
