// Package health — aggregate health check usecase for gateway-service.
// Performs parallel health checks against all upstream services within a 3s deadline.
package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// UpstreamConfig holds URL and name of an upstream service to health-check.
type UpstreamConfig struct {
	Name string // e.g. "data-service"
	URL  string // Base URL, e.g. "http://data-service:8082"
}

// ServiceHealth is the health status of one upstream.
type ServiceHealth struct {
	Name      string `json:"name"`
	Status    string `json:"status"`     // "healthy" | "unhealthy"
	LatencyMs int64  `json:"latency_ms"` // round-trip latency in milliseconds
	Error     string `json:"error,omitempty"`
}

// SystemHealth is the aggregated system health response.
type SystemHealth struct {
	Status    string                       `json:"status"`      // "healthy" | "degraded"
	CheckedAt time.Time                    `json:"checked_at"`
	Services  map[string]*ServiceHealth    `json:"services"`
	Version   string                       `json:"version"`
}

// AggregateUseCase performs parallel health checks against all upstreams.
type AggregateUseCase struct {
	upstreams []UpstreamConfig
	client    *http.Client // short timeout: 3s per check
	version   string
}

// NewAggregateUseCase creates a health check use case.
// version: service version string shown in /health response.
func NewAggregateUseCase(upstreams []UpstreamConfig, version string) *AggregateUseCase {
	return &AggregateUseCase{
		upstreams: upstreams,
		client:    &http.Client{Timeout: 3 * time.Second},
		version:   version,
	}
}

// Check performs all health checks in parallel and returns the aggregated result.
// If any service is unhealthy, the overall status is "degraded" (not "unhealthy").
// Total runtime is bounded by 3s (the upstream client timeout).
func (uc *AggregateUseCase) Check(ctx context.Context) *SystemHealth {
	results := make([]*ServiceHealth, len(uc.upstreams))
	var wg sync.WaitGroup

	for i, upstream := range uc.upstreams {
		wg.Add(1)
		go func(idx int, u UpstreamConfig) {
			defer wg.Done()
			start := time.Now()
			svc := &ServiceHealth{Name: u.Name}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.URL+"/health", nil)
			if err != nil {
				svc.Status = "unhealthy"
				svc.Error = fmt.Sprintf("build request: %v", err)
				svc.LatencyMs = time.Since(start).Milliseconds()
				results[idx] = svc
				return
			}

			resp, err := uc.client.Do(req)
			svc.LatencyMs = time.Since(start).Milliseconds()

			if err != nil {
				svc.Status = "unhealthy"
				svc.Error = err.Error()
			} else {
				resp.Body.Close() //nolint:errcheck
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					svc.Status = "healthy"
				} else {
					svc.Status = "unhealthy"
					svc.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
				}
			}
			results[idx] = svc
		}(i, upstream)
	}

	wg.Wait()

	system := &SystemHealth{
		Status:    "healthy",
		CheckedAt: time.Now().UTC(),
		Services:  make(map[string]*ServiceHealth, len(results)),
		Version:   uc.version,
	}

	for _, svc := range results {
		if svc == nil {
			continue
		}
		system.Services[svc.Name] = svc
		if svc.Status == "unhealthy" {
			system.Status = "degraded" // partial availability — not a fatal error
		}
	}

	return system
}
