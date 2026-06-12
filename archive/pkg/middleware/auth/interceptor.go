// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package auth provides gRPC interceptors for extracting authentication principals.
package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const principalKey contextKey = "osv-principal"

// PrincipalType identifies the type of authenticated principal.
type PrincipalType string

const (
	PrincipalAPIKey        PrincipalType = "API_KEY"
	PrincipalOAuth2        PrincipalType = "OAUTH2"
	PrincipalServiceAccount PrincipalType = "SERVICE_ACCOUNT"
)

// Principal represents an authenticated caller.
type Principal struct {
	ID             string
	Type           PrincipalType
	Roles          []string
	RateLimitTier  string // "anonymous" | "free" | "premium" | "internal"
	Metadata       map[string]string
}

// Validator validates auth credentials and returns a Principal.
type Validator interface {
	Validate(ctx context.Context, token string, tokenType string) (*Principal, error)
}

// UnaryServerInterceptor returns a gRPC unary interceptor that extracts the
// Authorization header and places the Principal in the context.
// Calls with no credentials get an anonymous principal (RateLimitTier=anonymous).
func UnaryServerInterceptor(validator Validator) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ctx, err := extractPrincipal(ctx, validator)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func extractPrincipal(ctx context.Context, validator Validator) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return withAnonymous(ctx), nil
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		// Also check x-api-key
		apiKeyHeaders := md.Get("x-api-key")
		if len(apiKeyHeaders) == 0 {
			return withAnonymous(ctx), nil
		}
		if validator == nil {
			return withAnonymous(ctx), nil
		}
		p, err := validator.Validate(ctx, apiKeyHeaders[0], "API_KEY")
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid API key: %v", err)
		}
		return context.WithValue(ctx, principalKey, p), nil
	}

	authHeader := authHeaders[0]
	if validator == nil {
		return withAnonymous(ctx), nil
	}

	var tokenType, token string
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 {
		tokenType = strings.ToUpper(parts[0])
		token = parts[1]
	} else {
		return nil, status.Errorf(codes.Unauthenticated, "malformed Authorization header")
	}

	p, err := validator.Validate(ctx, token, tokenType)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "auth validation failed: %v", err)
	}
	return context.WithValue(ctx, principalKey, p), nil
}

func withAnonymous(ctx context.Context) context.Context {
	return context.WithValue(ctx, principalKey, &Principal{
		ID:            "anonymous",
		Type:          PrincipalAPIKey,
		Roles:         []string{},
		RateLimitTier: "anonymous",
	})
}

// FromContext retrieves the Principal from the context.
// Returns nil if no principal was set.
func FromContext(ctx context.Context) *Principal {
	if p, ok := ctx.Value(principalKey).(*Principal); ok {
		return p
	}
	return nil
}
