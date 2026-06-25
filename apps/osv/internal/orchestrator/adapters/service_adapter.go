package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// Shared Listener for all in-memory gRPC services (if we want to use a single one)
// But typically, each service gets its own bufconn.Listener.
type EmbeddedServiceAdapter struct {
	name     string
	httpPort int // Optional: for exposing external REST if needed
	Listener *bufconn.Listener
	Server   *grpc.Server
	Mux      *http.ServeMux
}

func NewEmbeddedService(name string, httpPort int) *EmbeddedServiceAdapter {
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()

	// Register health service by default for all embedded services
	healthSvc := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSvc)
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	return &EmbeddedServiceAdapter{
		name:     name,
		httpPort: httpPort,
		Listener: lis,
		Server:   srv,
		Mux:      http.NewServeMux(),
	}
}

func (s *EmbeddedServiceAdapter) Name() string {
	return s.name
}

// Start runs the gRPC server (in-memory) and optionally an HTTP server.
func (s *EmbeddedServiceAdapter) Start(ctx context.Context) error {
	errCh := make(chan error, 2)

	// Start gRPC server
	go func() {
		slog.InfoContext(ctx, "starting embedded gRPC server", slog.String("service", s.name))
		if err := s.Server.Serve(s.Listener); err != nil && err != grpc.ErrServerStopped {
			errCh <- fmt.Errorf("grpc serve: %w", err)
		}
	}()

	// Start optional HTTP server if port > 0
	var httpSrv *http.Server
	if s.httpPort > 0 {
		s.Mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"status":"ok","service":%q,"mode":"embedded"}`, s.name)
		})
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.httpPort))
		if err == nil {
			httpSrv = &http.Server{Handler: s.Mux, ReadTimeout: 5 * time.Second}
			go func() {
				slog.InfoContext(ctx, "starting embedded HTTP server", slog.String("service", s.name), slog.Int("port", s.httpPort))
				if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
					errCh <- fmt.Errorf("http serve: %w", err)
				}
			}()
		} else {
			slog.WarnContext(ctx, "http port in use, skipping http for embedded service", slog.String("service", s.name), slog.Int("port", s.httpPort))
		}
	}

	select {
	case <-ctx.Done():
		slog.InfoContext(ctx, "shutting down embedded service", slog.String("service", s.name))
		s.Server.GracefulStop()
		if httpSrv != nil {
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpSrv.Shutdown(shutCtx)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

// Dial creates an in-memory client connection to this embedded service.
func (s *EmbeddedServiceAdapter) Dial(ctx context.Context) (*grpc.ClientConn, error) {
	return grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return s.Listener.Dial()
	}), grpc.WithInsecure())
}
