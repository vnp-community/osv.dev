// Package metrics provides a shared Prometheus metrics setup for OSV platform services.
// Each service embeds this package and exposes /metrics endpoint.
//
// Usage:
//
//	m := metrics.New("gateway-service")
//	router.Handle("/metrics", m.Handler())
//	// Wrap your handlers:
//	handler = m.WrapHTTP("get_vuln_by_id", handler)
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for a service.
type Metrics struct {
	serviceName string
	registry    *prometheus.Registry

	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	activeRequests  *prometheus.GaugeVec
	errorsTotal     *prometheus.CounterVec
}

// New creates a new Metrics instance for the given service name.
// Each call creates a fresh prometheus registry (isolated metrics).
func New(serviceName string) *Metrics {
	reg := prometheus.NewRegistry()

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "osv",
			Subsystem: serviceName,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests processed",
		},
		[]string{"handler", "method", "status"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "osv",
			Subsystem: serviceName,
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"handler", "method"},
	)

	activeRequests := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "osv",
			Subsystem: serviceName,
			Name:      "active_requests",
			Help:      "Number of currently active HTTP requests",
		},
		[]string{"handler"},
	)

	errorsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "osv",
			Subsystem: serviceName,
			Name:      "errors_total",
			Help:      "Total number of errors",
		},
		[]string{"handler", "error_type"},
	)

	reg.MustRegister(requestsTotal, requestDuration, activeRequests, errorsTotal)
	// Standard Go runtime metrics
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	return &Metrics{
		serviceName:     serviceName,
		registry:        reg,
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
		activeRequests:  activeRequests,
		errorsTotal:     errorsTotal,
	}
}

// Handler returns the Prometheus HTTP handler for /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		Registry: m.registry,
	})
}

// WrapHTTP wraps an http.HandlerFunc with request count and duration instrumentation.
func (m *Metrics) WrapHTTP(handlerName string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.activeRequests.WithLabelValues(handlerName).Inc()
		defer m.activeRequests.WithLabelValues(handlerName).Dec()

		// Wrap ResponseWriter to capture status code
		wrapped := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(wrapped.statusCode)

		m.requestsTotal.WithLabelValues(handlerName, r.Method, status).Inc()
		m.requestDuration.WithLabelValues(handlerName, r.Method).Observe(duration)
	}
}

// RecordError increments the error counter for the given handler and error type.
func (m *Metrics) RecordError(handlerName, errorType string) {
	m.errorsTotal.WithLabelValues(handlerName, errorType).Inc()
}

// statusRecorder captures the HTTP status code from the response.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}
