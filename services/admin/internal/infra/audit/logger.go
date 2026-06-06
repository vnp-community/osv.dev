// Package audit provides audit trail logging for all write operations.
// TASK-06-05: Record all admin actions with actor, timestamp, and diff.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ActionType identifies the kind of admin action.
type ActionType string

const (
	ActionWithdrawVuln     ActionType = "withdraw_vuln"
	ActionReprocessVuln    ActionType = "reprocess_vuln"
	ActionSyncSource       ActionType = "sync_source"
	ActionPauseSource      ActionType = "pause_source"
	ActionResumeSource     ActionType = "resume_source"
	ActionUpdateSourceConf ActionType = "update_source_config"
	ActionCreateAPIKey     ActionType = "create_api_key"
	ActionRevokeAPIKey     ActionType = "revoke_api_key"
	ActionResolveImportErr ActionType = "resolve_import_error"
	ActionRotateCredential ActionType = "rotate_credential"
)

// Entry is a single audit log record.
type Entry struct {
	ID         string     `json:"id"`
	Timestamp  time.Time  `json:"timestamp"`
	Actor      string     `json:"actor"`       // user/service that performed the action
	Action     ActionType `json:"action"`
	ResourceID string     `json:"resource_id"` // CVE ID, source name, API key ID, etc.
	Details    any        `json:"details,omitempty"` // action-specific metadata
	Success    bool       `json:"success"`
	Error      string     `json:"error,omitempty"`
	IPAddress  string     `json:"ip_address,omitempty"`
	UserAgent  string     `json:"user_agent,omitempty"`
}

// Logger is the interface for recording audit events.
type Logger interface {
	// Log records a completed audit event.
	Log(ctx context.Context, entry Entry) error

	// Query returns entries matching the filter criteria.
	Query(ctx context.Context, filter QueryFilter) ([]Entry, error)
}

// QueryFilter defines filter criteria for audit log queries.
type QueryFilter struct {
	Actor      string
	Action     ActionType
	ResourceID string
	Since      time.Time
	Until      time.Time
	Limit      int
}

// contextKey is the key for injecting audit logger into context.
type contextKey struct{}

// WithLogger injects an audit Logger into the context.
func WithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext extracts the audit Logger from context, or returns a no-op if not set.
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(contextKey{}).(Logger); ok {
		return l
	}
	return &noopLogger{}
}

// noopLogger discards all audit events (for testing/dev).
type noopLogger struct{}

func (*noopLogger) Log(_ context.Context, _ Entry) error                  { return nil }
func (*noopLogger) Query(_ context.Context, _ QueryFilter) ([]Entry, error) { return nil, nil }

// ---- In-memory audit logger ----

// InMemoryLogger stores audit events in memory (dev/testing only).
type InMemoryLogger struct {
	mu      sync.RWMutex
	entries []Entry
	seq     int
}

// NewInMemoryLogger creates a new in-memory audit logger.
func NewInMemoryLogger() *InMemoryLogger {
	return &InMemoryLogger{}
}

// Log records an audit entry.
func (l *InMemoryLogger) Log(_ context.Context, entry Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seq++
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit-%06d", l.seq)
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	l.entries = append(l.entries, entry)
	return nil
}

// Query returns entries matching the filter.
func (l *InMemoryLogger) Query(_ context.Context, f QueryFilter) ([]Entry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var results []Entry
	for _, e := range l.entries {
		if f.Actor != "" && e.Actor != f.Actor {
			continue
		}
		if f.Action != "" && e.Action != f.Action {
			continue
		}
		if f.ResourceID != "" && e.ResourceID != f.ResourceID {
			continue
		}
		if !f.Since.IsZero() && e.Timestamp.Before(f.Since) {
			continue
		}
		if !f.Until.IsZero() && e.Timestamp.After(f.Until) {
			continue
		}
		results = append(results, e)
		if f.Limit > 0 && len(results) >= f.Limit {
			break
		}
	}
	return results, nil
}

// All returns all stored audit entries (for testing).
func (l *InMemoryLogger) All() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// ---- Structured log (stdout) audit logger ----

// StructuredLogger writes audit events as JSON lines to stdout via zerolog.
// In production, plug this into your log aggregator (Cloud Logging, Datadog, etc.)
type StructuredLogger struct {
	writer func(entry Entry) // injectable for testing
}

// NewStructuredLogger creates a structured (JSON) audit logger.
func NewStructuredLogger() *StructuredLogger {
	return &StructuredLogger{
		writer: func(entry Entry) {
			b, _ := json.Marshal(entry)
			fmt.Printf("[AUDIT] %s\n", string(b))
		},
	}
}

// Log records an audit entry as a JSON line.
func (l *StructuredLogger) Log(_ context.Context, entry Entry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	l.writer(entry)
	return nil
}

// Query is not supported for the structured logger (use log aggregator queries).
func (l *StructuredLogger) Query(_ context.Context, _ QueryFilter) ([]Entry, error) {
	return nil, fmt.Errorf("StructuredLogger.Query: use your log aggregator (Cloud Logging, Datadog) to query audit logs")
}

// ---- Helper constructors ----

// NewWithdrawEntry creates an audit entry for a vulnerability withdrawal.
func NewWithdrawEntry(actor, vulnID, reason string) Entry {
	return Entry{
		Actor:      actor,
		Action:     ActionWithdrawVuln,
		ResourceID: vulnID,
		Success:    true,
		Details:    map[string]string{"reason": reason},
	}
}

// NewSyncSourceEntry creates an audit entry for a manual source sync trigger.
func NewSyncSourceEntry(actor, sourceName string) Entry {
	return Entry{
		Actor:      actor,
		Action:     ActionSyncSource,
		ResourceID: sourceName,
		Success:    true,
	}
}

// NewAPIKeyEntry creates an audit entry for API key creation/revocation.
func NewAPIKeyEntry(action ActionType, actor, keyID string) Entry {
	return Entry{
		Actor:      actor,
		Action:     action,
		ResourceID: keyID,
		Success:    true,
	}
}
