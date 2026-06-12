// Package policy implements RBAC permission checking for OpenVulnScan API routes.
package policy

import (
	authDomain "github.com/osv/api-gateway/internal/domain/auth"
)

// MethodPermissions maps URL path prefixes to HTTP method → required permission.
// Use "*" as method key to apply the same permission to all methods.
var MethodPermissions = map[string]map[string]string{
	"/api/v1/scans": {
		"POST":   "scan:create",
		"GET":    "scan:read",
		"DELETE": "scan:delete",
	},
	"/api/v1/assets": {
		"POST":   "asset:write",
		"PUT":    "asset:write",
		"PATCH":  "asset:write",
		"DELETE": "asset:write",
		"GET":    "asset:read",
	},
	"/api/v1/cves":          {"*": "scan:read"},
	"/api/v1/schedules":     {"POST": "scan:create", "PUT": "scan:create", "DELETE": "scan:create", "GET": "scan:read"},
	"/api/v1/reports":       {"*": "report:download"},
	"/api/v1/notifications": {"*": "system:configure"},
	"/api/v1/agents":        {"POST": "asset:write", "DELETE": "asset:write", "GET": "asset:read"},
	// /api/v1/agents/report is handled separately (API key path)
}

// CheckPermission returns true if the principal has the required permission.
// Returns true always if required is empty (unrestricted route).
func CheckPermission(p *authDomain.Principal, required string) bool {
	if required == "" {
		return true
	}
	return p.HasPermission(required)
}

// RequiredPermission looks up the permission needed for a given path prefix and HTTP method.
// Returns "" if the route has no permission requirement (public or unrestricted).
func RequiredPermission(pathPrefix, method string) string {
	methodMap, ok := MethodPermissions[pathPrefix]
	if !ok {
		return ""
	}
	// Try exact method match first
	if perm, found := methodMap[method]; found {
		return perm
	}
	// Fall back to wildcard
	if perm, found := methodMap["*"]; found {
		return perm
	}
	return ""
}

// FindMatchingPrefix returns the longest route prefix that matches the given path.
// Returns "" if no route matches.
func FindMatchingPrefix(path string) string {
	longestMatch := ""
	for prefix := range MethodPermissions {
		if len(prefix) > len(longestMatch) && len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			longestMatch = prefix
		}
	}
	return longestMatch
}
