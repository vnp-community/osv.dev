package health

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// ServiceHealth is the health result for a single upstream service.
type ServiceHealth struct {
	Name    string `json:"name"`
	Status  string `json:"status"`  // "healthy" | "unhealthy" | "unreachable"
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AggregateUpstreams concurrently checks the /health endpoint of each upstream
// service. Each entry in services maps a display name to its health URL.
// All checks run in parallel with a 3-second timeout per service.
func AggregateUpstreams(ctx context.Context, services map[string]string) []ServiceHealth {
	results := make([]ServiceHealth, 0, len(services))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, url := range services {
		wg.Add(1)
		go func(n, u string) {
			defer wg.Done()
			h := checkUpstream(ctx, n, u)
			mu.Lock()
			results = append(results, h)
			mu.Unlock()
		}(name, url)
	}
	wg.Wait()
	return results
}

// checkUpstream performs a single GET request to the upstream health URL.
func checkUpstream(ctx context.Context, name, url string) ServiceHealth {
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return ServiceHealth{
			Name:   name,
			Status: "unreachable",
			Error:  err.Error(),
		}
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		return ServiceHealth{
			Name:    name,
			Status:  "unreachable",
			Latency: latency,
			Error:   err.Error(),
		}
	}
	defer resp.Body.Close() //nolint:errcheck

	s := "healthy"
	if resp.StatusCode >= 500 {
		s = "unhealthy"
	} else if resp.StatusCode >= 400 {
		s = "unhealthy"
	}

	return ServiceHealth{
		Name:    name,
		Status:  s,
		Latency: latency,
	}
}

// IsAllHealthy returns true when every ServiceHealth has status "healthy".
func IsAllHealthy(results []ServiceHealth) bool {
	for _, r := range results {
		if r.Status != "healthy" {
			return false
		}
	}
	return true
}
