// Package observability provides standard service metrics using OTel.
package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ServiceMetrics holds standard metrics for a microservice.
type ServiceMetrics struct {
	RequestTotal    metric.Int64Counter
	RequestDuration metric.Float64Histogram
	ErrorTotal      metric.Int64Counter
	CacheHits       metric.Int64Counter
	CacheMisses     metric.Int64Counter
	ActiveRequests  metric.Int64UpDownCounter
}

// NewServiceMetrics creates standard metrics for a service.
func NewServiceMetrics(meter metric.Meter, serviceName string) (*ServiceMetrics, error) {
	requestTotal, err := meter.Int64Counter(
		serviceName+".requests.total",
		metric.WithDescription("Total number of requests processed"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		serviceName+".request.duration_ms",
		metric.WithDescription("Request duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	errorTotal, err := meter.Int64Counter(
		serviceName+".errors.total",
		metric.WithDescription("Total number of errors"),
	)
	if err != nil {
		return nil, err
	}

	cacheHits, err := meter.Int64Counter(
		serviceName+".cache.hits",
		metric.WithDescription("Cache hit count"),
	)
	if err != nil {
		return nil, err
	}

	cacheMisses, err := meter.Int64Counter(
		serviceName+".cache.misses",
		metric.WithDescription("Cache miss count"),
	)
	if err != nil {
		return nil, err
	}

	activeRequests, err := meter.Int64UpDownCounter(
		serviceName+".active_requests",
		metric.WithDescription("Number of requests currently being processed"),
	)
	if err != nil {
		return nil, err
	}

	return &ServiceMetrics{
		RequestTotal:    requestTotal,
		RequestDuration: requestDuration,
		ErrorTotal:      errorTotal,
		CacheHits:       cacheHits,
		CacheMisses:     cacheMisses,
		ActiveRequests:  activeRequests,
	}, nil
}

// RecordRequest records a completed request with its duration and outcome.
func (m *ServiceMetrics) RecordRequest(ctx context.Context, method string, start time.Time, err error) {
	duration := float64(time.Since(start).Milliseconds())
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("status", outcomeLabel(err)),
	}
	m.RequestTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.RequestDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
	if err != nil {
		m.ErrorTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("method", method)))
	}
}

// RecordCacheHit increments the cache hit counter.
func (m *ServiceMetrics) RecordCacheHit(ctx context.Context, cacheLayer string) {
	m.CacheHits.Add(ctx, 1, metric.WithAttributes(attribute.String("layer", cacheLayer)))
}

// RecordCacheMiss increments the cache miss counter.
func (m *ServiceMetrics) RecordCacheMiss(ctx context.Context, cacheLayer string) {
	m.CacheMisses.Add(ctx, 1, metric.WithAttributes(attribute.String("layer", cacheLayer)))
}

// TrackActive increments active requests on entry and decrements on return.
// Usage: defer m.TrackActive(ctx, "operation")()
func (m *ServiceMetrics) TrackActive(ctx context.Context, operation string) func() {
	attrs := metric.WithAttributes(attribute.String("operation", operation))
	m.ActiveRequests.Add(ctx, 1, attrs)
	return func() {
		m.ActiveRequests.Add(ctx, -1, attrs)
	}
}

func outcomeLabel(err error) string {
	if err == nil {
		return "success"
	}
	return "error"
}
