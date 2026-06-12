// Package provider implements the auth provider chain for cve-search.
// Supports multiple authentication backends evaluated in a configurable order.
// Syntax mirrors Linux PAM: "local:required,ldap:sufficient"
package provider

import (
	"context"
	"fmt"
	"strings"
)

// AuthResult indicates the outcome of an authentication attempt.
type AuthResult int

const (
	// AuthOK means credentials were accepted by this provider.
	AuthOK AuthResult = iota
	// AuthWrongCreds means credentials were rejected (wrong password / user not found).
	AuthWrongCreds
	// AuthUnavailable means the provider is temporarily unable to respond.
	AuthUnavailable
)

// ControlFlag determines how a provider result affects chain evaluation,
// following the PAM model.
type ControlFlag string

const (
	// Required: must succeed. Failure immediately returns false.
	Required ControlFlag = "required"
	// Sufficient: success immediately returns true. Failure continues the chain.
	Sufficient ControlFlag = "sufficient"
)

// Provider is an authentication backend.
type Provider interface {
	// Name returns the provider identifier, e.g. "local", "ldap".
	Name() string
	// Authenticate checks credentials. Returns AuthResult and any technical error.
	Authenticate(ctx context.Context, username, password string) (AuthResult, error)
}

type chainEntry struct {
	provider Provider
	flag     ControlFlag
}

// Chain evaluates multiple auth providers in order.
type Chain struct {
	entries []chainEntry
}

// New parses a chain spec and builds a Chain.
// Spec format: "providerName:flag[,providerName:flag,...]"
// Example: "local:required" or "local:sufficient,ldap:required"
func New(spec string, providers map[string]Provider) (*Chain, error) {
	var chain Chain
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid chain entry %q: expected 'name:flag'", part)
		}
		name := strings.TrimSpace(kv[0])
		flag := ControlFlag(strings.TrimSpace(kv[1]))

		if flag != Required && flag != Sufficient {
			return nil, fmt.Errorf("unknown control flag %q in chain entry %q (use 'required' or 'sufficient')", flag, part)
		}
		p, ok := providers[name]
		if !ok {
			return nil, fmt.Errorf("unknown provider %q in chain spec (available: %v)", name, providerNames(providers))
		}
		chain.entries = append(chain.entries, chainEntry{provider: p, flag: flag})
	}
	if len(chain.entries) == 0 {
		return nil, fmt.Errorf("auth chain spec is empty")
	}
	return &chain, nil
}

// Authenticate evaluates the provider chain for the given credentials.
// Returns true if authentication succeeds according to chain logic.
func (c *Chain) Authenticate(ctx context.Context, username, password string) bool {
	for _, e := range c.entries {
		result, err := e.provider.Authenticate(ctx, username, password)

		// Technical errors / unavailability: skip provider (don't count as failure)
		if err != nil || result == AuthUnavailable {
			continue
		}

		if result == AuthOK {
			// Any provider succeeding means authenticated
			return true
		}

		// AuthWrongCreds
		if e.flag == Required {
			// A required provider failed → entire chain fails immediately
			return false
		}
		// Sufficient provider failed → continue to next provider
	}
	return false
}

func providerNames(m map[string]Provider) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	return names
}
