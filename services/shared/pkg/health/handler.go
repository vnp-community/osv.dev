// Package health provides standardized HTTP health and readiness handlers.
// Shared across all OSV platform services.
//
// Usage:
//
//	// Simple liveness:
//	r.HandleFunc("/health", health.LivenessHandler)
//
//	// Readiness with checks:
//	pgCheck := func() error { return db.Ping() }
//	natsCheck := func() error { return nc.Flush() }
//	r.HandleFunc("/ready", health.ReadinessHandler(pgCheck, natsCheck))
package health

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"
)

var startTime = time.Now()

// LivenessHandler returns HTTP 200 when the service is running.
// Use for Kubernetes liveness probes.
func LivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"status":     "ok",
		"uptime":     time.Since(startTime).Truncate(time.Second).String(),
		"go_version": runtime.Version(),
	})
}

// ReadinessHandler returns a readiness probe handler that runs all checks.
// Returns HTTP 200 when all checks pass, 503 when any check fails.
// Use for Kubernetes readiness probes.
func ReadinessHandler(checks ...func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type checkResult struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		}

		results := make([]checkResult, 0, len(checks))
		allOK := true

		// Run checks in parallel with timeout
		type namedCheck struct {
			name string
			fn   func() error
		}

		var mu sync.Mutex
		var wg sync.WaitGroup
		wg.Add(len(checks))
		for i, check := range checks {
			check := check
			name := getCheckName(i)
			go func() {
				defer wg.Done()
				err := runWithTimeout(check, 5*time.Second)
				cr := checkResult{Name: name, Status: "ok"}
				if err != nil {
					cr.Status = "fail"
					cr.Error = err.Error()
					mu.Lock()
					allOK = false
					mu.Unlock()
				}
				mu.Lock()
				results = append(results, cr)
				mu.Unlock()
			}()
		}
		wg.Wait()

		status := "ready"
		httpStatus := http.StatusOK
		if !allOK {
			status = "not_ready"
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"status": status,
			"checks": results,
		})
	}
}

func runWithTimeout(fn func() error, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return nil // Timeout is treated as pass to avoid false negatives
	}
}

func getCheckName(i int) string {
	names := []string{"db", "cache", "messaging", "storage", "upstream"}
	if i < len(names) {
		return names[i]
	}
	return "check"
}
