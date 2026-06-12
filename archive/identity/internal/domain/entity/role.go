package entity

// RoleID represents a DefectDojo RBAC role.
// Numeric IDs match Django's role numeric identifiers for API compatibility.
type RoleID int

const (
	RoleReader      RoleID = 5
	RoleAPIImporter RoleID = 1
	RoleWriter      RoleID = 2
	RoleMaintainer  RoleID = 3
	RoleOwner       RoleID = 4
)

// RoleName returns the human-readable display name for this role.
func (r RoleID) RoleName() string {
	switch r {
	case RoleReader:
		return "Reader"
	case RoleAPIImporter:
		return "API Importer"
	case RoleWriter:
		return "Writer"
	case RoleMaintainer:
		return "Maintainer"
	case RoleOwner:
		return "Owner"
	default:
		return "Unknown"
	}
}

// Permission is a named action that a role is allowed to perform.
type Permission string

const (
	// Product permissions
	PermProductView   Permission = "product:view"
	PermProductAdd    Permission = "product:add"
	PermProductEdit   Permission = "product:edit"
	PermProductDelete Permission = "product:delete"

	// Engagement permissions
	PermEngagementView Permission = "engagement:view"
	PermEngagementAdd  Permission = "engagement:add"
	PermEngagementEdit Permission = "engagement:edit"

	// Finding permissions
	PermFindingView   Permission = "finding:view"
	PermFindingAdd    Permission = "finding:add"
	PermFindingEdit   Permission = "finding:edit"
	PermFindingClose  Permission = "finding:close"
	PermFindingDelete Permission = "finding:delete"

	// Import permissions
	PermImportScanResult Permission = "import:scan_result"

	// Risk acceptance permissions
	PermRiskAcceptanceAdd    Permission = "risk_acceptance:add"
	PermRiskAcceptanceEdit   Permission = "risk_acceptance:edit"
	PermRiskAcceptanceDelete Permission = "risk_acceptance:delete"

	// User management permissions
	PermUserView Permission = "user:view"
	PermUserAdd  Permission = "user:add"
	PermUserEdit Permission = "user:edit"

	// System-level permissions
	PermSystemConfigure Permission = "system:configure"

	// Report permissions
	PermReportDownload Permission = "report:download"
)

// RolePermissions maps each RoleID to the set of permissions it grants.
// Implements the DefectDojo RBAC matrix from specs/services/02-identity-auth-service.md.
var RolePermissions = map[RoleID][]Permission{
	RoleReader: {
		PermProductView, PermEngagementView, PermFindingView,
		PermReportDownload,
	},
	RoleAPIImporter: {
		PermProductView, PermEngagementView,
		PermFindingView, PermFindingAdd,
		PermImportScanResult,
	},
	RoleWriter: {
		PermProductView,
		PermEngagementView, PermEngagementAdd, PermEngagementEdit,
		PermFindingView, PermFindingAdd, PermFindingEdit, PermFindingClose,
		PermImportScanResult,
		PermReportDownload,
	},
	RoleMaintainer: {
		PermProductView, PermProductEdit,
		PermEngagementView, PermEngagementAdd, PermEngagementEdit,
		PermFindingView, PermFindingAdd, PermFindingEdit, PermFindingClose, PermFindingDelete,
		PermImportScanResult,
		PermRiskAcceptanceAdd, PermRiskAcceptanceEdit, PermRiskAcceptanceDelete,
		PermUserView,
		PermReportDownload,
	},
	RoleOwner: {
		PermProductView, PermProductAdd, PermProductEdit, PermProductDelete,
		PermEngagementView, PermEngagementAdd, PermEngagementEdit,
		PermFindingView, PermFindingAdd, PermFindingEdit, PermFindingClose, PermFindingDelete,
		PermImportScanResult,
		PermRiskAcceptanceAdd, PermRiskAcceptanceEdit, PermRiskAcceptanceDelete,
		PermUserView, PermUserAdd, PermUserEdit,
		PermSystemConfigure,
		PermReportDownload,
	},
}

// RoleHasPermission returns true if roleID grants the given permission.
func RoleHasPermission(roleID RoleID, perm Permission) bool {
	for _, p := range RolePermissions[roleID] {
		if p == perm {
			return true
		}
	}
	return false
}
