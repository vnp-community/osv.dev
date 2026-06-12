package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/osv/admin/internal/infra/audit"
)

func TestInMemoryLogger(t *testing.T) {
	ctx := context.Background()
	logger := audit.NewInMemoryLogger()

	t.Run("Log and retrieve", func(t *testing.T) {
		err := logger.Log(ctx, audit.Entry{
			Actor:      "admin@example.com",
			Action:     audit.ActionWithdrawVuln,
			ResourceID: "CVE-2024-1234",
			Success:    true,
		})
		if err != nil {
			t.Fatalf("Log: unexpected error: %v", err)
		}
		entries := logger.All()
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].Actor != "admin@example.com" {
			t.Errorf("Actor: got %q", entries[0].Actor)
		}
		if entries[0].ID == "" {
			t.Error("ID should be auto-generated")
		}
		if entries[0].Timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
	})

	t.Run("Query by action", func(t *testing.T) {
		logger2 := audit.NewInMemoryLogger()
		_ = logger2.Log(ctx, audit.Entry{Actor: "user1", Action: audit.ActionSyncSource, ResourceID: "src-a"})
		_ = logger2.Log(ctx, audit.Entry{Actor: "user2", Action: audit.ActionWithdrawVuln, ResourceID: "CVE-1"})
		_ = logger2.Log(ctx, audit.Entry{Actor: "user1", Action: audit.ActionSyncSource, ResourceID: "src-b"})

		entries, err := logger2.Query(ctx, audit.QueryFilter{Action: audit.ActionSyncSource})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("expected 2 sync entries, got %d", len(entries))
		}
	})

	t.Run("Query by actor", func(t *testing.T) {
		logger3 := audit.NewInMemoryLogger()
		_ = logger3.Log(ctx, audit.Entry{Actor: "alice", Action: audit.ActionSyncSource})
		_ = logger3.Log(ctx, audit.Entry{Actor: "bob", Action: audit.ActionSyncSource})
		_ = logger3.Log(ctx, audit.Entry{Actor: "alice", Action: audit.ActionWithdrawVuln})

		entries, err := logger3.Query(ctx, audit.QueryFilter{Actor: "alice"})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("expected 2 alice entries, got %d", len(entries))
		}
	})

	t.Run("Query with limit", func(t *testing.T) {
		logger4 := audit.NewInMemoryLogger()
		for i := 0; i < 10; i++ {
			_ = logger4.Log(ctx, audit.Entry{Actor: "user", Action: audit.ActionSyncSource})
		}
		entries, err := logger4.Query(ctx, audit.QueryFilter{Limit: 3})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(entries) != 3 {
			t.Errorf("expected 3 entries (limit), got %d", len(entries))
		}
	})

	t.Run("Query since", func(t *testing.T) {
		logger5 := audit.NewInMemoryLogger()
		cutoff := time.Now()
		_ = logger5.Log(ctx, audit.Entry{Action: audit.ActionSyncSource, Timestamp: cutoff.Add(-2 * time.Hour)})
		_ = logger5.Log(ctx, audit.Entry{Action: audit.ActionSyncSource, Timestamp: cutoff.Add(time.Hour)})

		entries, err := logger5.Query(ctx, audit.QueryFilter{Since: cutoff})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 entry after cutoff, got %d", len(entries))
		}
	})
}

func TestContextLogger(t *testing.T) {
	ctx := context.Background()
	logger := audit.NewInMemoryLogger()

	ctx = audit.WithLogger(ctx, logger)
	fromCtx := audit.FromContext(ctx)
	_ = fromCtx.Log(ctx, audit.Entry{Actor: "svc", Action: audit.ActionRevokeAPIKey})
	if len(logger.All()) != 1 {
		t.Error("expected context logger to write to the injected logger")
	}
}

func TestNoopLogger(t *testing.T) {
	ctx := context.Background()
	// FromContext without injection returns noop
	noop := audit.FromContext(ctx)
	err := noop.Log(ctx, audit.Entry{Actor: "test"})
	if err != nil {
		t.Errorf("noop.Log unexpected error: %v", err)
	}
}

func TestHelperConstructors(t *testing.T) {
	e := audit.NewWithdrawEntry("alice", "CVE-2024-9999", "test withdrawal")
	if e.Action != audit.ActionWithdrawVuln {
		t.Errorf("Action: %v", e.Action)
	}
	if e.ResourceID != "CVE-2024-9999" {
		t.Errorf("ResourceID: %v", e.ResourceID)
	}
}
