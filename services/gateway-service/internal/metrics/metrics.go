// Package metrics wires Prometheus metrics for this service.
// Uses the shared metrics package from services/shared/pkg/metrics.
//
// Usage in main.go:
//
//	m := metrics.Service()
//	r.Handle("/metrics", m.Handler())
//	handler = m.WrapHTTP("handler_name", handler)
package metrics

import sharedmetrics "github.com/osv/shared/pkg/metrics"

// Service returns the singleton Metrics instance for this service.
// Call once in main.go and pass to handlers.
func Service() *sharedmetrics.Metrics {
	return sharedmetrics.New("gateway-service")
}
