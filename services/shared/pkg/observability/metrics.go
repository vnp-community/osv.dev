package observability

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type CommonMetrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	CacheHits       *prometheus.CounterVec
	DBQueryDuration *prometheus.HistogramVec
}

func NewCommonMetrics(service string) *CommonMetrics {
	l := prometheus.Labels{"service": service}
	return &CommonMetrics{
		RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total", ConstLabels: l,
		}, []string{"method", "path", "status"}),
		RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: "http_request_duration_seconds", ConstLabels: l,
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		CacheHits: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "cache_operations_total", ConstLabels: l,
		}, []string{"result"}),
		DBQueryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: "db_query_duration_seconds", ConstLabels: l,
			Buckets: []float64{.001, .005, .01, .05, .1, .5, 1, 2},
		}, []string{"operation"}),
	}
}

// StartMetricsServer starts /metrics HTTP server on given port in background goroutine.
func StartMetricsServer(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(fmt.Sprintf(":%d", port), mux) //nolint:errcheck
}
