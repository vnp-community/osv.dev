// adapters.go — Adapter implementations of the orchestrator.Service interface.
// Each adapter wraps an existing service (data-service, search-service, etc.)
// and makes it runnable by the Supervisor.
//
// These adapters start a minimal HTTP health server for each service.
// Full service wiring (gRPC, NATS, DB) is handled by each service's own Start().
// In the current phase, adapters delegate to the embedded server constructors
// defined in Sprint A (SA-SVC-01).
package orchestrator

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// HTTPService is a simple Service that serves an HTTP handler on a given port.
// Used to expose health checks and basic APIs for each embedded service.
type HTTPService struct {
	name    string
	port    int
	mux     *http.ServeMux
}

// NewHTTPService creates a new HTTPService.
func NewHTTPService(name string, port int) *HTTPService {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok","service":%q}`, name)
	})
	return &HTTPService{name: name, port: port, mux: mux}
}

// Handle registers additional HTTP handlers.
func (s *HTTPService) Handle(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

// Name implements orchestrator.Service.
func (s *HTTPService) Name() string { return s.name }

// Start implements orchestrator.Service.
func (s *HTTPService) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("%s listen :%d: %w", s.name, s.port, err)
	}
	srv := &http.Server{
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	go srv.Serve(ln) //nolint:errcheck
	<-ctx.Done()
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutCtx)
}

// envPort reads a port from an environment variable, falling back to defaultPort.
func envPort(key string, defaultPort int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultPort
	}
	var port int
	if _, err := fmt.Sscanf(v, "%d", &port); err != nil || port <= 0 {
		return defaultPort
	}
	return port
}
