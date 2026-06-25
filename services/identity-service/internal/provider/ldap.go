// Package provider — LDAP authentication provider.
// Authenticates users via LDAP simple bind.
// Mirrors Python: lib/authenticationMethods/LDAP.py in cve-search
package provider

import (
	"context"
	"crypto/tls"
	"fmt"

	ldap "github.com/go-ldap/ldap/v3"
)

// LDAPConfig holds LDAP server configuration.
type LDAPConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`        // default: 389
	BaseDN     string `yaml:"base_dn"`     // e.g. "dc=example,dc=com"
	UserAttr   string `yaml:"user_attr"`   // "uid" | "sAMAccountName" | "mail"
	UseTLS     bool   `yaml:"use_tls"`     // use LDAPS
	SkipVerify bool   `yaml:"skip_verify"` // skip TLS cert verification
}

// LDAPProvider authenticates users via LDAP bind.
// Implements the Provider interface (same as local.go).
type LDAPProvider struct {
	cfg LDAPConfig
}

// NewLDAPProvider creates an LDAP auth provider.
func NewLDAPProvider(cfg LDAPConfig) *LDAPProvider {
	// Apply defaults
	if cfg.UserAttr == "" {
		cfg.UserAttr = "uid" // OpenLDAP default
	}
	if cfg.Port == 0 {
		cfg.Port = 389
	}
	return &LDAPProvider{cfg: cfg}
}

// Name implements Provider.
func (p *LDAPProvider) Name() string { return "ldap" }

// Authenticate performs LDAP simple bind authentication.
//
// Strategy:
//  1. Build user DN: "{userAttr}={username},{baseDN}"
//  2. Attempt LDAP bind with user credentials
//  3. AuthOK on success, AuthWrongCreds on invalid creds, AuthUnavailable on network error
//
// Security: Empty password is rejected to prevent anonymous bind attacks.
// Mirrors Python: lib/authenticationMethods/LDAP.py::authenticate()
func (p *LDAPProvider) Authenticate(ctx context.Context, username, password string) (AuthResult, error) {
	// Reject empty password — prevents anonymous bind vulnerability
	if password == "" {
		return AuthWrongCreds, nil
	}

	conn, err := p.dial()
	if err != nil {
		// LDAP server unreachable — skip to next provider in chain
		return AuthUnavailable, fmt.Errorf("LDAP dial %s:%d: %w", p.cfg.Host, p.cfg.Port, err)
	}
	defer conn.Close()

	// Build user DN: "uid=john.doe,dc=example,dc=com"
	userDN := fmt.Sprintf("%s=%s,%s",
		p.cfg.UserAttr,
		ldap.EscapeFilter(username),
		p.cfg.BaseDN,
	)

	if err := conn.Bind(userDN, password); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			return AuthWrongCreds, nil
		}
		// Other LDAP error (account disabled, server error, etc.)
		// Treat as unavailable to allow chain fallback
		return AuthUnavailable, fmt.Errorf("LDAP bind error: %w", err)
	}

	return AuthOK, nil
}

// dial establishes LDAP connection (plain or TLS).
func (p *LDAPProvider) dial() (*ldap.Conn, error) {
	addr := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)

	if p.cfg.UseTLS {
		return ldap.DialTLS("tcp", addr, &tls.Config{
			InsecureSkipVerify: p.cfg.SkipVerify, //nolint:gosec
			ServerName:         p.cfg.Host,
		})
	}
	return ldap.DialURL("ldap://" + addr)
}
