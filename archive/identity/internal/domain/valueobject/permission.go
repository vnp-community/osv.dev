// Package valueobject contains domain value objects for the auth service.
package valueobject

// Permission is a fine-grained capability string.
type Permission = string

// Permission constants define all capabilities in the system.
const (
	PermScanCreate      Permission = "scan:create"
	PermScanRead        Permission = "scan:read"
	PermScanDelete      Permission = "scan:delete"
	PermAssetRead       Permission = "asset:read"
	PermAssetWrite      Permission = "asset:write"
	PermReportDownload  Permission = "report:download"
	PermAgentReport     Permission = "agent:report"   // agent-only
	PermSystemConfigure Permission = "system:configure"
)

// AllPermissions is the full set of permissions (admin role).
var AllPermissions = []Permission{
	PermScanCreate, PermScanRead, PermScanDelete,
	PermAssetRead, PermAssetWrite,
	PermReportDownload,
	PermAgentReport,
	PermSystemConfigure,
}

// RolePermissions maps each role to its granted permissions.
var RolePermissions = map[string][]Permission{
	"admin":    AllPermissions,
	"user":     {PermScanCreate, PermScanRead, PermScanDelete, PermAssetRead, PermAssetWrite, PermReportDownload},
	"readonly": {PermScanRead, PermAssetRead},
	"agent":    {PermAgentReport},
}

// PermissionsFor returns the permission list for a given role.
// Returns an empty slice for unknown roles.
func PermissionsFor(role string) []Permission {
	if perms, ok := RolePermissions[role]; ok {
		return perms
	}
	return []Permission{}
}
