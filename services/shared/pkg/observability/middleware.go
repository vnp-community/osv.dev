package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

// LoggingMiddleware logs each request with structured fields.
func LoggingMiddleware(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info().
				Str("method", r.Method).Str("path", r.URL.Path).
				Int("status", rec.code).
				Int64("latency_ms", time.Since(start).Milliseconds()).
				Msg("request")
		})
	}
}

// MetricsMiddleware records Prometheus metrics per request.
func MetricsMiddleware(m *CommonMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rec, r)
			status := strconv.Itoa(rec.code)
			m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
			m.RequestDuration.WithLabelValues(r.Method, r.URL.Path).
				Observe(time.Since(start).Seconds())
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}
