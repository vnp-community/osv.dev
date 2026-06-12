// Package health provides an enterprise health check framework.
// Supports multiple probers aggregated into a single liveness/readiness response.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents a health check result.
type Status string

const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
)

// CheckResult is the result of a single health check.
type CheckResult struct {
	Name    string        `json:"name"`
	Status  Status        `json:"status"`
	Message string        `json:"message,omitempty"`
	Latency time.Duration `json:"latency_ms"`
}

// Report is the aggregated health report.
type Report struct {
	Status  Status                 `json:"status"`
	Checks  []CheckResult          `json:"checks"`
	Time    time.Time              `json:"time"`
}

// Checker is a single health probe.
type Checker interface {
	Name() string
	Check(ctx context.Context) CheckResult
}

// CheckerFunc adapts a function to the Checker interface.
type CheckerFunc struct {
	name string
	fn   func(ctx context.Context) error
}

// NewCheckerFunc creates a CheckerFunc.
func NewCheckerFunc(name string, fn func(ctx context.Context) error) *CheckerFunc {
	return &CheckerFunc{name: name, fn: fn}
}

func (c *CheckerFunc) Name() string { return c.name }
func (c *CheckerFunc) Check(ctx context.Context) CheckResult {
	start := time.Now()
	err := c.fn(ctx)
	latency := time.Since(start)
	if err != nil {
		return CheckResult{
			Name:    c.name,
			Status:  StatusDown,
			Message: err.Error(),
			Latency: latency,
		}
	}
	return CheckResult{Name: c.name, Status: StatusOK, Latency: latency}
}

// MultiChecker aggregates multiple health checkers.
type MultiChecker struct {
	checkers []Checker
	timeout  time.Duration
}

// NewMultiChecker creates a MultiChecker with the given checkers.
func NewMultiChecker(timeout time.Duration, checkers ...Checker) *MultiChecker {
	return &MultiChecker{checkers: checkers, timeout: timeout}
}

// Check runs all health checks concurrently and returns an aggregate report.
func (m *MultiChecker) Check(ctx context.Context) Report {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	results := make([]CheckResult, len(m.checkers))
	var wg sync.WaitGroup

	for i, c := range m.checkers {
		i, c := i, c
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = c.Check(ctx)
		}()
	}
	wg.Wait()

	overall := StatusOK
	for _, r := range results {
		if r.Status == StatusDown {
			overall = StatusDown
			break
		}
		if r.Status == StatusDegraded {
			overall = StatusDegraded
		}
	}

	return Report{Status: overall, Checks: results, Time: time.Now().UTC()}
}

// LiveHandler returns an HTTP handler for liveness probes (always 200 if process is running).
func LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
	}
}

// ReadyHandler returns an HTTP handler for readiness probes.
// Returns 200 when all checkers pass, 503 when any checker fails.
func ReadyHandler(checker *MultiChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := checker.Check(r.Context())
		w.Header().Set("Content-Type", "application/json")
		if report.Status == StatusDown {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(report) //nolint:errcheck
	}
}
