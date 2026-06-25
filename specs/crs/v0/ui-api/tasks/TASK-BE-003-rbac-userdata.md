# TASK-BE-003 — identity-service: RBAC Matrix + Admin Handlers + Profile

| Field | Value |
|-------|-------|
| **Task ID** | TASK-BE-003 |
| **Service** | `services/identity-service` |
| **Solution Ref** | [SOL-UI-001 §2.5, §2.7, §2.9](../solutions/SOL-UI-001-auth-service-extension.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-BE-001 (dto.go phải có trước) |
| **Estimated** | 3h |
| **Status** | ✅ DONE |

---

## Context

Frontend cần:
1. `UserDTO.permissions[]` = RBAC matrix mapping role → permissions list
2. `GET /api/v1/admin/users` + CRUD để Admin quản lý user accounts
3. `GET /api/v1/admin/roles` trả về RBAC permission matrix
4. `GET/PATCH /api/v1/profile` cho user tự quản lý profile
5. `POST /api/v1/profile/change-password`

---

## Goal

Implement RBAC domain, admin handlers, profile handlers, đăng ký tất cả vào router.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `services/identity-service/internal/domain/rbac.go` |
| CREATE | `services/identity-service/internal/adapter/http/admin_handler.go` |
| CREATE | `services/identity-service/internal/adapter/http/profile_handler.go` |
| MODIFY | `services/identity-service/internal/adapter/http/router.go` |

---

## Implementation

### File 1: `services/identity-service/internal/domain/rbac.go`

```go
package domain

// Role represents a user's role in the system
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleUser     Role = "user"
	RoleReadOnly Role = "readonly"
	RoleAgent    Role = "agent"
)

// RolePermissions maps roles to their permission strings
// This is the Single Source of Truth for RBAC
var RolePermissions = map[Role][]string{
	RoleAdmin: {
		"scan:create", "scan:read",
		"asset:write", "asset:read",
		"finding:write", "finding:read",
		"report:download",
		"user:manage",
		"system:configure",
	},
	RoleUser: {
		"scan:create", "scan:read",
		"asset:write", "asset:read",
		"finding:write", "finding:read",
		"report:download",
	},
	RoleReadOnly: {
		"scan:read",
		"asset:read",
		"finding:read",
		"report:download",
	},
	RoleAgent: {
		"asset:write",
		"agent:report",
	},
}

// AllPermissions returns all unique permission strings in the system
var AllPermissions = []PermissionDef{
	{Key: "scan:create",      Description: "Create and start scans"},
	{Key: "scan:read",        Description: "View scan results and history"},
	{Key: "asset:write",      Description: "Create and modify assets"},
	{Key: "asset:read",       Description: "View asset inventory"},
	{Key: "finding:write",    Description: "Update finding status and notes"},
	{Key: "finding:read",     Description: "View findings and details"},
	{Key: "report:download",  Description: "Download security reports"},
	{Key: "user:manage",      Description: "Manage users and role assignments"},
	{Key: "system:configure", Description: "Configure platform settings"},
	{Key: "agent:report",     Description: "Submit agent scan results via API"},
}

// PermissionDef describes a single permission
type PermissionDef struct {
	Key         string `json:"permission"`
	Description string `json:"description"`
}

// HasPermission checks if a role has a specific permission
func HasPermission(role Role, permission string) bool {
	perms := RolePermissions[role]
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}
```

### File 2: `services/identity-service/internal/adapter/http/admin_handler.go`

```go
package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// AdminHandler handles admin-only endpoints
type AdminHandler struct {
	userRepo UserRepository
}

// ────────────────────────────────────────────────
// GET /admin/users
// ────────────────────────────────────────────────

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	role      := r.URL.Query().Get("role")
	isActive  := r.URL.Query().Get("is_active")
	q         := r.URL.Query().Get("q")
	page, ps  := parsePagination(r)

	filter := UserFilter{
		Role:     role,
		IsActive: isActive,
		Query:    q,
		Page:     page,
		PageSize: ps,
	}

	users, total, err := h.userRepo.List(r.Context(), filter)
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", err.Error())
		return
	}

	dtos := make([]*UserDTO, len(users))
	for i, u := range users {
		dtos[i] = toUserDTO(u)
	}

	respondJSON(w, 200, map[string]interface{}{
		"users":     dtos,
		"total":     total,
		"page":      page,
		"page_size": ps,
	})
}

// ────────────────────────────────────────────────
// POST /admin/users/invite
// ────────────────────────────────────────────────

func (h *AdminHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if req.Email == "" || req.Role == "" {
		respondError(w, 400, "VALIDATION_ERROR", "email and role are required")
		return
	}

	// Validate role
	validRoles := map[string]bool{"admin": true, "user": true, "readonly": true}
	if !validRoles[req.Role] {
		respondError(w, 400, "VALIDATION_ERROR", "invalid role: must be admin, user, or readonly")
		return
	}

	// Check if user already exists
	existing, _ := h.userRepo.FindByEmail(r.Context(), req.Email)
	if existing != nil {
		respondError(w, 409, "CONFLICT", "User with this email already exists")
		return
	}

	// Create user with temporary password (will force reset on first login)
	user, err := h.userRepo.Create(r.Context(), CreateUserInput{
		Email:         req.Email,
		Name:          req.Name,
		Role:          Role(req.Role),
		MustResetPassword: true,
	})
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to create user")
		return
	}

	// TODO: Send invitation email via notification-service

	respondJSON(w, 201, toUserDTO(user))
}

// ────────────────────────────────────────────────
// PATCH /admin/users/{id}
// ────────────────────────────────────────────────

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.PathValue("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid user ID")
		return
	}

	var req struct {
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
		Name     *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		respondError(w, 404, "NOT_FOUND", "User not found")
		return
	}

	if req.Role != nil {
		user.Role = Role(*req.Role)
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Name != nil {
		user.Name = *req.Name
	}

	if err := h.userRepo.Update(r.Context(), user); err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to update user")
		return
	}

	respondJSON(w, 200, toUserDTO(user))
}

// ────────────────────────────────────────────────
// POST /admin/users/{id}/unlock
// ────────────────────────────────────────────────

func (h *AdminHandler) UnlockUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid user ID")
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		respondError(w, 404, "NOT_FOUND", "User not found")
		return
	}

	user.IsActive = true
	user.FailedLoginAttempts = 0
	if err := h.userRepo.Update(r.Context(), user); err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to unlock user")
		return
	}

	respondJSON(w, 200, map[string]interface{}{"id": userID, "is_active": true})
}

// ────────────────────────────────────────────────
// GET /admin/roles  — RBAC matrix for UI
// ────────────────────────────────────────────────

func (h *AdminHandler) GetRBACMatrix(w http.ResponseWriter, r *http.Request) {
	roles := []string{"admin", "user", "readonly", "agent"}

	permissions := make([]map[string]interface{}, len(AllPermissions))
	for i, perm := range AllPermissions {
		roleMap := make(map[string]bool)
		for _, role := range roles {
			roleMap[role] = HasPermission(Role(role), perm.Key)
		}
		permissions[i] = map[string]interface{}{
			"permission":  perm.Key,
			"description": perm.Description,
			"roles":       roleMap,
		}
	}

	respondJSON(w, 200, map[string]interface{}{
		"roles":       roles,
		"permissions": permissions,
	})
}

// ─── Helpers ──────────────────────────────────

func parsePagination(r *http.Request) (page, pageSize int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	return
}
```

### File 3: `services/identity-service/internal/adapter/http/profile_handler.go`

```go
package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ProfileHandler handles user self-service profile endpoints
type ProfileHandler struct {
	userRepo UserRepository
}

// GET /profile
func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		respondError(w, 401, "UNAUTHORIZED", "Invalid user ID")
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		respondError(w, 404, "NOT_FOUND", "User not found")
		return
	}

	respondJSON(w, 200, toUserDTO(user))
}

// PATCH /profile
func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		respondError(w, 401, "UNAUTHORIZED", "Invalid user ID")
		return
	}

	var req struct {
		Name      *string `json:"name"`
		AvatarURL *string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		respondError(w, 404, "NOT_FOUND", "User not found")
		return
	}

	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}

	if err := h.userRepo.Update(r.Context(), user); err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to update profile")
		return
	}

	respondJSON(w, 200, toUserDTO(user))
}

// POST /profile/change-password
func (h *ProfileHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.Header.Get("X-User-ID"))
	if err != nil {
		respondError(w, 401, "UNAUTHORIZED", "Invalid user ID")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if len(req.NewPassword) < 8 {
		respondError(w, 400, "VALIDATION_ERROR", "New password must be at least 8 characters")
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		respondError(w, 404, "NOT_FOUND", "User not found")
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.CurrentPassword)); err != nil {
		respondError(w, 401, "INVALID_CREDENTIALS", "Current password is incorrect")
		return
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to hash password")
		return
	}

	user.HashedPassword = string(newHash)
	if err := h.userRepo.Update(r.Context(), user); err != nil {
		respondError(w, 500, "INTERNAL_ERROR", "Failed to update password")
		return
	}

	respondJSON(w, 200, map[string]interface{}{"success": true})
}
```

### Router additions:

```go
// services/identity-service/internal/adapter/http/router.go — thêm routes

// Admin routes — gateway applies RequireRole("admin") before forwarding
mux.HandleFunc("GET  /admin/users",                  h.Admin.ListUsers)
mux.HandleFunc("POST /admin/users/invite",            h.Admin.InviteUser)
mux.HandleFunc("PATCH /admin/users/{id}",             h.Admin.UpdateUser)
mux.HandleFunc("POST /admin/users/{id}/unlock",       h.Admin.UnlockUser)
mux.HandleFunc("POST /admin/users/{id}/reset-password", h.Admin.ResetPassword)
mux.HandleFunc("GET  /admin/roles",                   h.Admin.GetRBACMatrix)

// Profile routes — gateway applies auth middleware
mux.HandleFunc("GET  /profile",               h.Profile.GetProfile)
mux.HandleFunc("PATCH /profile",              h.Profile.UpdateProfile)
mux.HandleFunc("POST /profile/change-password", h.Profile.ChangePassword)
```

---

## Verification

```bash
cd services/identity-service

go build ./...

# Test RBAC matrix
go test ./internal/domain/... -v -run TestRBAC

# Smoke test GetRBACMatrix endpoint
curl http://localhost:8081/admin/roles | jq '.permissions | length'
# Expected: 10
```

---

## Checklist

- [x] `rbac.go` định nghĩa 4 roles: admin, user, readonly, agent
- [x] `RolePermissions` map có đúng 10 unique permissions
- [x] `HasPermission(role, perm)` trả về đúng value
- [x] `AllPermissions` slice có 10 entries với key + description
- [x] `ListUsers` support filter: role, is_active, q (search), pagination
- [x] `InviteUser` validate role, check duplicate email, return 409 nếu exists
- [x] `UpdateUser` chỉ update fields được gửi lên (PATCH semantics)
- [x] `GetRBACMatrix` return matrix 10 permissions × 4 roles với bool values
- [x] `ChangePassword` verify current password trước khi update
- [x] `go build ./...` thành công không có errors
