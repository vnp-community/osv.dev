// Package health_test provides unit tests for the health check framework.
package health_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/osv/shared/pkg/health"
)

func TestCheckerFunc_OK(t *testing.T) {
	c := health.NewCheckerFunc("test", func(ctx context.Context) error { return nil })
	result := c.Check(context.Background())
	if result.Status != health.StatusOK {
		t.Fatalf("expected OK, got %s", result.Status)
	}
	if result.Name != "test" {
		t.Fatalf("expected name 'test', got %s", result.Name)
	}
}

func TestCheckerFunc_Down(t *testing.T) {
	c := health.NewCheckerFunc("db", func(ctx context.Context) error {
		return errors.New("connection refused")
	})
	result := c.Check(context.Background())
	if result.Status != health.StatusDown {
		t.Fatalf("expected DOWN, got %s", result.Status)
	}
	if result.Message == "" {
		t.Fatal("expected error message")
	}
}

func TestMultiChecker_AllOK(t *testing.T) {
	mc := health.NewMultiChecker(2*time.Second,
		health.NewCheckerFunc("a", func(ctx context.Context) error { return nil }),
		health.NewCheckerFunc("b", func(ctx context.Context) error { return nil }),
	)
	report := mc.Check(context.Background())
	if report.Status != health.StatusOK {
		t.Fatalf("expected OK, got %s", report.Status)
	}
	if len(report.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(report.Checks))
	}
}

func TestMultiChecker_OneDown(t *testing.T) {
	mc := health.NewMultiChecker(2*time.Second,
		health.NewCheckerFunc("a", func(ctx context.Context) error { return nil }),
		health.NewCheckerFunc("b", func(ctx context.Context) error { return errors.New("down") }),
	)
	report := mc.Check(context.Background())
	if report.Status != health.StatusDown {
		t.Fatalf("expected DOWN, got %s", report.Status)
	}
}

func TestMultiChecker_RunsConcurrently(t *testing.T) {
	start := time.Now()
	mc := health.NewMultiChecker(2*time.Second,
		health.NewCheckerFunc("slow1", func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}),
		health.NewCheckerFunc("slow2", func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}),
	)
	mc.Check(context.Background())
	elapsed := time.Since(start)

	// Both run concurrently → total time should be ~100ms, not ~200ms
	if elapsed > 150*time.Millisecond {
		t.Fatalf("expected concurrent execution, took %s", elapsed)
	}
}

func TestLiveHandler_Returns200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	health.LiveHandler()(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReadyHandler_Returns503WhenDown(t *testing.T) {
	mc := health.NewMultiChecker(2*time.Second,
		health.NewCheckerFunc("db", func(ctx context.Context) error {
			return errors.New("timeout")
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	health.ReadyHandler(mc)(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "down") {
		t.Fatalf("expected 'down' in response body, got: %s", w.Body.String())
	}
}

func TestReadyHandler_Returns200WhenHealthy(t *testing.T) {
	mc := health.NewMultiChecker(2*time.Second,
		health.NewCheckerFunc("ok", func(ctx context.Context) error { return nil }),
	)
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	health.ReadyHandler(mc)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
