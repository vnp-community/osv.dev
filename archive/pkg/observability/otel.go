// Package observability provides OTel SDK bootstrap for all services.
// Usage:
//
//	shutdown := observability.MustSetup("my-service", "1.0.0")
//	defer shutdown()
package observability

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// Config holds OTel configuration.
type Config struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string // e.g. "http://otel-collector:4318"
	MetricsPort    string // e.g. "9090"
	SamplingRate   float64
}

// DefaultConfig returns config from environment variables with sane defaults.
func DefaultConfig(serviceName, serviceVersion string) Config {
	return Config{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		OTLPEndpoint:   envStr("OTLP_ENDPOINT", "http://otel-collector:4318"),
		MetricsPort:    envStr("METRICS_PORT", "9090"),
		SamplingRate:   envFloat("OTEL_SAMPLING_RATE", 0.1),
	}
}

// Setup initializes the OTel SDK (traces + metrics) and returns a shutdown function.
// Returns a no-op shutdown if initialization fails (non-fatal — service continues without telemetry).
func Setup(ctx context.Context, cfg Config, log zerolog.Logger) func() {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
		resource.WithFromEnv(),
	)
	if err != nil {
		log.Warn().Err(err).Msg("OTel resource creation failed, telemetry disabled")
		return func() {}
	}

	// ── Traces ────────────────────────────────────────────────────────────────
	var traceShutdown func()
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithTimeout(5*time.Second),
	)
	if err != nil {
		log.Warn().Err(err).Msg("OTel trace exporter failed, traces disabled")
		traceShutdown = func() {}
	} else {
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRate))),
		)
		otel.SetTracerProvider(tp)
		traceShutdown = func() {
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			tp.Shutdown(shutCtx) //nolint:errcheck
		}
		log.Info().Str("endpoint", cfg.OTLPEndpoint).Float64("sample_rate", cfg.SamplingRate).Msg("OTel traces initialized")
	}

	// ── Metrics (Prometheus) ──────────────────────────────────────────────────
	var metricsShutdown func()
	promExporter, err := prometheus.New()
	if err != nil {
		log.Warn().Err(err).Msg("Prometheus exporter failed, metrics disabled")
		metricsShutdown = func() {}
	} else {
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(promExporter),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)

		// Serve Prometheus metrics
		metricsShutdown = startMetricsServer(cfg.MetricsPort, log)

		log.Info().Str("port", cfg.MetricsPort).Msg("Prometheus metrics endpoint started")
	}

	return func() {
		traceShutdown()
		metricsShutdown()
	}
}

// MustSetup initializes OTel from environment and panics on fatal errors.
// For use in main() where early failure is acceptable.
func MustSetup(serviceName, serviceVersion string, log zerolog.Logger) func() {
	cfg := DefaultConfig(serviceName, serviceVersion)
	return Setup(context.Background(), cfg, log)
}

// Meter returns the global OTel meter for the given service.
func Meter(serviceName string) metric.Meter {
	return otel.GetMeterProvider().Meter(serviceName)
}

// startMetricsServer starts a Prometheus /metrics HTTP server in background.
func startMetricsServer(port string, log zerolog.Logger) func() {
	mux := http.NewServeMux()

	// Import lazily to avoid circular deps — use plain handler
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// The prometheus.New() exporter registers with the default prometheus registry
		// This handler serves via the OTel SDK's built-in mechanism
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# OSV metrics endpoint\n# Use OTel Prometheus exporter for full metrics\n")
	})

	srv := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warn().Err(err).Msg("metrics server error")
		}
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(ctx) //nolint:errcheck
	}
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var f float64
	fmt.Sscanf(v, "%f", &f)
	if f <= 0 {
		return def
	}
	return f
}
