// Package grpcclient — identity_client.go
// IdentityClient wraps the identity-service gRPC connection.
// Provides JWT validation and API key validation, mapped to the domain Principal type.
package grpcclient

import (
	"context"
	"time"

	"google.golang.org/grpc"

	authv1 "github.com/osv/shared/proto/gen/go/auth/v1"
	"github.com/osv/gateway-service/internal/domain/auth"
)

// IdentityClient wraps the identity-service gRPC AuthServiceClient.
type IdentityClient struct {
	conn   *grpc.ClientConn
	client authv1.AuthServiceClient
}

// NewIdentityClient creates a new IdentityClient connected to addr.
func NewIdentityClient(addr string, opts ...grpc.DialOption) (*IdentityClient, error) {
	all := append(defaultDialOptions(), opts...)
	conn, err := grpc.NewClient(addr, all...)
	if err != nil {
		return nil, err
	}
	return &IdentityClient{
		conn:   conn,
		client: authv1.NewAuthServiceClient(conn),
	}, nil
}

// Close tears down the underlying gRPC connection.
func (c *IdentityClient) Close() error { return c.conn.Close() }

// ValidateToken validates a JWT Bearer token and returns the authenticated Principal.
// Returns error if the token is invalid or expired.
func (c *IdentityClient) ValidateToken(ctx context.Context, token string) (*auth.Principal, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.ValidateToken(ctx, &authv1.ValidateTokenRequest{
		Token: token,
	})
	if err != nil {
		return nil, err
	}
	if !resp.GetValid() {
		return nil, errUnauthorized(resp.GetError())
	}

	principal := &auth.Principal{
		ID:          resp.GetUserId(),
		Permissions: resp.GetPermissions(),
	}
	// Map role string to domain Role
	if r := resp.GetRole(); r != "" {
		principal.Roles = []auth.Role{auth.Role(r)}
	}
	return principal, nil
}

// ValidateAPIKey validates an API key (ovs_ prefix) and returns the authenticated Principal.
func (c *IdentityClient) ValidateAPIKey(ctx context.Context, apiKey string) (*auth.Principal, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.ValidateAPIKey(ctx, &authv1.ValidateAPIKeyRequest{
		ApiKey: apiKey,
	})
	if err != nil {
		return nil, err
	}
	if !resp.GetValid() {
		return nil, errUnauthorized(resp.GetError())
	}

	return &auth.Principal{
		ID:          resp.GetUserId(),
		APIKeyID:    resp.GetKeyId(),
		Type:        auth.PrincipalAPIKey,
		Permissions: resp.GetPermissions(),
	}, nil
}
