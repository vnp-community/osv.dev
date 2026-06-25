// Package nats — cve_publisher.go
// CVEEventPublisher publishes CVE lifecycle events to NATS JetStream.
//
// Events published:
//   - osv.vuln.imported  (existing subject, now typed)
//   - osv.vuln.updated
//   - osv.vuln.withdrawn
//   - osv.kev.updated    (new — triggers notification-service alerts)
//
// These subjects are already declared in bootstrap.go PublishedSubjects.
// This file adds typed event structs and a publisher that wraps the generic Publisher.
//
// Usage (additive — existing Publisher is unchanged):
//
//	cvePublisher := nats.NewCVEEventPublisher(publisher, log)
//	cvePublisher.PublishImported(ctx, nats.CVEImportedEvent{ID: "CVE-2024-12345", Source: "nvd"})
package nats

import (
	"context"
	"time"
)

// ── Subject constants ────────────────────────────────────────────────────────
// These must match the PublishedSubjects list in bootstrap.go.

const (
	SubjectCVEImported  = "osv.vuln.imported"
	SubjectCVEUpdated   = "osv.vuln.updated"
	SubjectCVEWithdrawn = "osv.vuln.withdrawn"
	SubjectKEVUpdated   = "osv.kev.updated"
)

// ── Event structs ────────────────────────────────────────────────────────────

// CVEImportedEvent is published when a new CVE/vuln record is ingested.
type CVEImportedEvent struct {
	EventType string    `json:"event_type"` // "cve.imported"
	ID        string    `json:"id"`         // CVE/GHSA/OSV ID
	Source    string    `json:"source"`     // "nvd" | "ghsa" | "pypi" | etc.
	SourceURL string    `json:"source_url,omitempty"`
	Severity  string    `json:"severity,omitempty"`  // CRITICAL|HIGH|MEDIUM|LOW
	SyncedAt  time.Time `json:"synced_at"`
}

// CVEUpdatedEvent is published when an existing record changes.
type CVEUpdatedEvent struct {
	EventType     string    `json:"event_type"` // "cve.updated"
	ID            string    `json:"id"`
	ChangedFields []string  `json:"changed_fields,omitempty"` // e.g. ["description","cvss_score"]
	Source        string    `json:"source,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CVEWithdrawnEvent is published when a CVE is withdrawn or rejected.
type CVEWithdrawnEvent struct {
	EventType   string    `json:"event_type"` // "cve.withdrawn"
	ID          string    `json:"id"`
	Reason      string    `json:"reason,omitempty"`
	WithdrawnAt time.Time `json:"withdrawn_at"`
}

// KEVUpdatedEvent is published after a CISA KEV catalog sync completes.
type KEVUpdatedEvent struct {
	EventType    string    `json:"event_type"` // "kev.updated"
	AddedCount   int       `json:"added_count"`
	UpdatedCount int       `json:"updated_count"`
	SyncedAt     time.Time `json:"synced_at"`
}

// ── Publisher ────────────────────────────────────────────────────────────────

// CVEEventPublisher wraps the generic Publisher to provide typed CVE event methods.
// It is additive: the existing Publisher remains unchanged.
type CVEEventPublisher struct {
	p *Publisher // existing generic publisher
}

// NewCVEEventPublisher creates a CVEEventPublisher wrapping the existing Publisher.
func NewCVEEventPublisher(p *Publisher) *CVEEventPublisher {
	return &CVEEventPublisher{p: p}
}

// PublishImported publishes a CVEImportedEvent asynchronously (non-blocking).
// Any publish error is logged by the underlying Publisher; it does not block the caller.
func (c *CVEEventPublisher) PublishImported(ctx context.Context, event CVEImportedEvent) {
	event.EventType = "cve.imported"
	go c.p.Publish(ctx, SubjectCVEImported, event) //nolint:errcheck
}

// PublishUpdated publishes a CVEUpdatedEvent asynchronously.
func (c *CVEEventPublisher) PublishUpdated(ctx context.Context, event CVEUpdatedEvent) {
	event.EventType = "cve.updated"
	go c.p.Publish(ctx, SubjectCVEUpdated, event) //nolint:errcheck
}

// PublishWithdrawn publishes a CVEWithdrawnEvent asynchronously.
func (c *CVEEventPublisher) PublishWithdrawn(ctx context.Context, event CVEWithdrawnEvent) {
	event.EventType = "cve.withdrawn"
	go c.p.Publish(ctx, SubjectCVEWithdrawn, event) //nolint:errcheck
}

// PublishKEVUpdated publishes a KEVUpdatedEvent asynchronously.
func (c *CVEEventPublisher) PublishKEVUpdated(ctx context.Context, event KEVUpdatedEvent) {
	event.EventType = "kev.updated"
	go c.p.Publish(ctx, SubjectKEVUpdated, event) //nolint:errcheck
}
