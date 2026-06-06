package policy_test

import (
	"testing"

	authDomain "github.com/osv/api-gateway/internal/domain/auth"
	"github.com/osv/api-gateway/internal/domain/policy"
)

func TestCheckPermission(t *testing.T) {
	tests := []struct {
		name     string
		perms    []string
		required string
		want     bool
	}{
		{"empty required always passes", []string{}, "", true},
		{"has permission", []string{"scan:read", "asset:read"}, "scan:read", true},
		{"lacks permission", []string{"scan:read"}, "scan:create", false},
		{"admin has all permissions", []string{"scan:create", "scan:read", "scan:delete", "asset:read", "asset:write", "report:download", "system:configure"}, "system:configure", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &authDomain.Principal{Permissions: tt.perms}
			if got := policy.CheckPermission(p, tt.required); got != tt.want {
				t.Errorf("CheckPermission(%v, %q) = %v, want %v", tt.perms, tt.required, got, tt.want)
			}
		})
	}
}

func TestRequiredPermission(t *testing.T) {
	tests := []struct {
		prefix string
		method string
		want   string
	}{
		{"/api/v1/scans", "GET", "scan:read"},
		{"/api/v1/scans", "POST", "scan:create"},
		{"/api/v1/scans", "DELETE", "scan:delete"},
		{"/api/v1/assets", "GET", "asset:read"},
		{"/api/v1/assets", "PUT", "asset:write"},
		{"/api/v1/cves", "GET", "scan:read"},
		{"/api/v1/cves", "POST", "scan:read"},   // wildcard
		{"/api/v1/reports", "DELETE", "report:download"}, // wildcard
		{"/unknown/path", "GET", ""},
	}

	for _, tt := range tests {
		t.Run(tt.prefix+":"+tt.method, func(t *testing.T) {
			got := policy.RequiredPermission(tt.prefix, tt.method)
			if got != tt.want {
				t.Errorf("RequiredPermission(%q, %q) = %q, want %q", tt.prefix, tt.method, got, tt.want)
			}
		})
	}
}

func TestFindMatchingPrefix(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/v1/scans", "/api/v1/scans"},
		{"/api/v1/scans/abc-123", "/api/v1/scans"},
		{"/api/v1/scans/abc/findings", "/api/v1/scans"},
		{"/api/v1/assets", "/api/v1/assets"},
		{"/api/v1/auth/login", ""}, // auth is SkipAuth, not in MethodPermissions
		{"/healthz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := policy.FindMatchingPrefix(tt.path)
			if got != tt.want {
				t.Errorf("FindMatchingPrefix(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
