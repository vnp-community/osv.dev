// infra/proxy/grpc_proxy.go — Transparent gRPC reverse proxy
package proxy

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/osv/unified-gateway/internal/domain/policy"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// connPool caches upstream gRPC connections.
type connPool struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn
}

// GRPCProxy is a transparent gRPC reverse proxy that forwards calls to upstream services.
type GRPCProxy struct {
	routes []policy.Route
	pool   *connPool
	log    zerolog.Logger
}

// NewGRPCProxy creates a new gRPC reverse proxy.
func NewGRPCProxy(routes []policy.Route, log zerolog.Logger) *GRPCProxy {
	return &GRPCProxy{
		routes: routes,
		pool:   &connPool{conns: make(map[string]*grpc.ClientConn)},
		log:    log,
	}
}

// Forward proxies a gRPC unary call to the correct upstream service.
// Matches by method prefix: "/osv.vuln.v1.OSVService/..." → vulnerability-query service.
func (p *GRPCProxy) Forward(ctx context.Context, method string, req, resp interface{}) error {
	upstream, err := p.resolveUpstream(method)
	if err != nil {
		return fmt.Errorf("no upstream for %s: %w", method, err)
	}

	conn, err := p.getConn(upstream)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", upstream.Address, err)
	}

	// Propagate incoming metadata
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	return conn.Invoke(ctx, method, req, resp)
}

func (p *GRPCProxy) resolveUpstream(method string) (policy.ServiceRef, error) {
	// Map gRPC full method name to upstream by prefix convention
	methodMap := map[string]string{
		"/osv.vuln.v1":         "vulnerability-query",
		"/osv.search.v1":       "search",
		"/osv.version.v1":      "version-index",
		"/osv.ingestion.v1":    "ingestion",
		"/osv.ai.v1":           "ai-enrichment",
		"/osv.alias.v1":        "alias-relations",
		"/osv.sourcesync.v1":   "source-sync",
		"/osv.notification.v1": "notification",
	}

	for prefix, svcName := range methodMap {
		if strings.HasPrefix(method, prefix) {
			for _, route := range p.routes {
				if route.Upstream.Name == svcName {
					return route.Upstream, nil
				}
			}
		}
	}
	return policy.ServiceRef{}, fmt.Errorf("unknown service for method %s", method)
}

func (p *GRPCProxy) getConn(svc policy.ServiceRef) (*grpc.ClientConn, error) {
	p.pool.mu.RLock()
	if conn, ok := p.pool.conns[svc.Address]; ok {
		p.pool.mu.RUnlock()
		return conn, nil
	}
	p.pool.mu.RUnlock()

	p.pool.mu.Lock()
	defer p.pool.mu.Unlock()

	// Double-check after acquiring write lock
	if conn, ok := p.pool.conns[svc.Address]; ok {
		return conn, nil
	}

	conn, err := grpc.NewClient(svc.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", svc.Address, err)
	}
	p.pool.conns[svc.Address] = conn
	return conn, nil
}
