# SB-CLI-01 — CLI NATS Publisher (importer dual backend)

## Metadata
- **Task ID**: SB-CLI-01
- **Sprint**: B (P1 — CLI Enhancement)
- **Ước tính**: 1.5 giờ
- **Dependencies**: SA-SHARED-01
- **Spec nguồn**: `specs/solutions/enhance-cli-app/02_cli-upgrade.md` § "2.1 cmd/importer"

---

## Context

```bash
# Xem importer hiện tại
cat apps/cli/cmd/importer/main.go | head -80
cat apps/cli/internal/importer/importer.go | head -80
cat apps/cli/go.mod | grep -A5 "require"

# Xem existing publisher interface
grep -r "Publisher" apps/cli/internal/importer/ --include="*.go" | head -20
grep -r "clients.Publisher" apps/cli/ --include="*.go" | head -10
```

---

## Goal

Thêm NATS publisher alternative vào `apps/cli` để `cmd/importer` có thể publish events đến data-service qua NATS thay vì GCP Pub/Sub. Code GCP cũ KHÔNG bị xóa hay sửa.

---

## Files to Create

### File 1: `apps/cli/internal/importer/nats_publisher.go`

```go
// Package importer — NATSPublisher is the microservices-backend alternative
// to GCPPublisher. Activated when CLI_BACKEND=microservices.
//
// Instead of sending tasks to GCP Pub/Sub, it publishes vulnerability import
// events to NATS JetStream subject "osv.vuln.imported".
package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// vulnImportedEvent mirrors shared/pkg/events.VulnImportedEvent.
// Duplicated here to avoid circular dependency — CLI should not import services/shared.
type vulnImportedEvent struct {
	ID         string    `json:"id"`
	Source     string    `json:"source"`
	ImportedAt time.Time `json:"imported_at"`
	OSVData    []byte    `json:"osv_data"`
	TraceID    string    `json:"trace_id,omitempty"`
}

const natsSubjectVulnImported = "osv.vuln.imported"

// NATSPublisher publishes vulnerability import events to NATS JetStream.
// Implements a compatible interface with the existing Publisher in importer.go.
type NATSPublisher struct {
	js nats.JetStreamContext
}

// NewNATSPublisher creates a publisher connected to the given NATS server.
// It ensures the OSV_VULN stream exists before returning.
//
// natsURL example: "nats://localhost:4222"
func NewNATSPublisher(natsURL string) (*NATSPublisher, error) {
	nc, err := nats.Connect(natsURL,
		nats.Name("osv-cli-importer"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect %s: %w", natsURL, err)
	}

	js, err := nc.JetStream(nats.PublishAsyncMaxPending(256))
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream context: %w", err)
	}

	// Ensure OSV_VULN stream exists (idempotent)
	if err := ensureVulnStream(js); err != nil {
		nc.Close()
		return nil, fmt.Errorf("ensure nats stream: %w", err)
	}

	return &NATSPublisher{js: js}, nil
}

// PublishVuln publishes a single vulnerability import event.
// This is the primary method — called once per imported vulnerability record.
func (p *NATSPublisher) PublishVuln(ctx context.Context, id, source string, osvJSON []byte) error {
	event := vulnImportedEvent{
		ID:         id,
		Source:     source,
		ImportedAt: time.Now().UTC(),
		OSVData:    osvJSON,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal vuln event: %w", err)
	}
	_, err = p.js.Publish(natsSubjectVulnImported, payload, nats.Context(ctx))
	return err
}

func ensureVulnStream(js nats.JetStreamContext) error {
	_, err := js.AddStream(&nats.StreamConfig{
		Name:     "OSV_VULN",
		Subjects: []string{"osv.vuln.*"},
		Storage:  nats.FileStorage,
		MaxAge:   7 * 24 * time.Hour,
		Replicas: 1,
	})
	// If stream already exists, that's fine
	if err != nil && err.Error() != "nats: stream name already in use" {
		return err
	}
	return nil
}
```

### File 2: `apps/cli/cmd/importer/backend_selector.go`

```go
// backend_selector.go selects between GCP Pub/Sub and NATS backends
// based on the CLI_BACKEND environment variable.
// RULE: existing GCP backend code is NOT modified.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/osv/apps/cli/internal/importer"
)

// selectPublisher returns the appropriate publisher based on CLI_BACKEND env var.
//
// CLI_BACKEND=microservices → NATSPublisher (publishes to NATS)
// CLI_BACKEND=gcp (default) → existing GCPPublisher (unchanged)
//
// This function is called in main() AFTER the existing GCP setup block.
// When CLI_BACKEND=microservices, the returned publisher replaces config.Publisher.
func selectNATSPublisherIfNeeded(ctx context.Context) (*importer.NATSPublisher, bool, error) {
	if os.Getenv("CLI_BACKEND") != "microservices" {
		return nil, false, nil // Use existing GCP publisher
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	pub, err := importer.NewNATSPublisher(natsURL)
	if err != nil {
		return nil, false, fmt.Errorf("nats publisher init: %w", err)
	}

	return pub, true, nil
}
```

---

## Update `apps/cli/cmd/importer/main.go`

**Thêm** (không xóa existing code) vào phần sau khi config được khởi tạo:

```go
// === THÊM VÀO main() sau block "config.Publisher = &clients.GCPPublisher{...}" ===
// Microservices backend override — activated by CLI_BACKEND=microservices
if natsPub, ok, err := selectNATSPublisherIfNeeded(ctx); err != nil {
    logger.FatalContext(ctx, "NATS publisher init failed", slog.Any("error", err))
} else if ok {
    // NATSPublisher does not implement clients.Publisher directly.
    // Use an adapter in the Run() loop — the importer will call natsPub.PublishVuln().
    logger.InfoContext(ctx, "Using NATS backend", slog.String("url", os.Getenv("NATS_URL")))
    config.NATSPublisher = natsPub // config field added (see below)
}
// === END ADD ===
```

---

## Update `apps/cli/go.mod`

```go
// Thêm vào require block:
require (
    // ... existing ...
    github.com/nats-io/nats.go v1.42.0  // NATS client for microservices backend
)
```

---

## Acceptance Criteria

- [ ] `apps/cli/internal/importer/nats_publisher.go` tạo với `NATSPublisher` struct
- [ ] `NewNATSPublisher(natsURL)` connect + setup stream
- [ ] `PublishVuln(ctx, id, source, osvJSON)` publish to `osv.vuln.imported`
- [ ] `apps/cli/cmd/importer/backend_selector.go` tạo với `selectNATSPublisherIfNeeded`
- [ ] Existing `importer.go` và `main.go` KHÔNG bị xóa content
- [ ] `go build ./...` từ `apps/cli` PASS

---

## Verification

```bash
cd apps/cli
go build ./...
# Output: no errors

# Test NATS publisher (requires NATS running):
# CLI_BACKEND=microservices NATS_URL=nats://localhost:4222 go run ./cmd/importer/ --dry-run
```

---

## ✅ Execution Status: COMPLETED ✅

**Completed**: 2026-06-13

### Files Created (additive only)
- `apps/cli/internal/importer/nats_publisher.go` — `NATSPublisher` với `PublishVuln()`, `Close()`
- `apps/cli/cmd/importer/backend_selector.go` — `selectNATSPublisherIfNeeded()` (CLI_BACKEND=microservices)
- `apps/cli/go.mod` — thêm `github.com/nats-io/nats.go v1.42.0`, sửa `osv/pkg` replace path

### Build Verification
```
go build ./internal/importer/...  → OK
go build ./cmd/importer/...       → OK
```
