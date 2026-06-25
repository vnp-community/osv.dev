package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/osv/identity-service/internal/domain/entity"
	"github.com/osv/identity-service/internal/domain/repository"
	"github.com/osv/identity-service/internal/domain/valueobject"
	ucadmin "github.com/osv/identity-service/internal/usecase/admin_user"
	ucapikey "github.com/osv/identity-service/internal/usecase/manage_api_key"
	"github.com/rs/zerolog"
)

// AdminHandler handles admin-only endpoints
type AdminHandler struct {
	userRepo repository.UserRepository
	adminUC  *ucadmin.UseCase
	apiKeyUC *ucapikey.UseCase
	// [FIX TASK-HC-010] rbacRepo reads role metadata from DB; nil = static fallback
	rbacRepo *RBACRepo
	// [FIX TASK-HC-014] inviteUC handles user invitation with email
	inviteUC *ucadmin.InviteUserUseCase
	appBaseURL string
	log        zerolog.Logger
}

func NewAdminHandler(userRepo repository.UserRepository, apiKeyUC *ucapikey.UseCase, log zerolog.Logger) *AdminHandler {
	return &AdminHandler{
		userRepo: userRepo,
		adminUC:  ucadmin.New(userRepo),
		apiKeyUC: apiKeyUC,
		log:      log,
	}
}

// WithRBACRepo enables DB-backed role metadata for the RBAC matrix.
func (h *AdminHandler) WithRBACRepo(repo *RBACRepo) *AdminHandler {
	h.rbacRepo = repo
	return h
}

// WithInviteUC enables real invitation use case (TASK-HC-014).
func (h *AdminHandler) WithInviteUC(uc *ucadmin.InviteUserUseCase, appBaseURL string) *AdminHandler {
	h.inviteUC = uc
	h.appBaseURL = appBaseURL
	return h
}


// UserDTO represents a user in HTTP responses
type UserDTO struct {
	ID                  string     `json:"id"`
	Email               string     `json:"email"`
	Name                string     `json:"name"`         // FIX: alias for username, required by frontend
	Username            string     `json:"username"`
	Role                string     `json:"role"`
	IsActive            bool       `json:"is_active"`
	IsVerified          bool       `json:"is_verified"`
	MFAEnabled          bool       `json:"mfa_enabled"`  // FIX: added for admin panel
	FailedLoginAttempts int        `json:"failed_login_attempts"`
	LoginAttempts       int        `json:"login_attempts"` // CR-011: alias for FE
	IsLocked            bool       `json:"is_locked"`       // CR-011: true when account is locked
	LastLoginAt         *time.Time `json:"last_login_at"`
	CreatedAt           time.Time  `json:"created_at"`
	Permissions         []string   `json:"permissions,omitempty"`
}

func toUserDTO(u *entity.User) UserDTO {
	// CR-011: is_locked is derived from IsActive (IsLocked() returns !IsActive)
	isLocked := u.IsLocked()
	return UserDTO{
		ID:                  u.ID.String(),
		Email:               u.Email,
		Name:                u.Username,  // FIX: map Username → Name
		Username:            u.Username,
		Role:                string(u.Role),
		IsActive:            u.IsActive,
		IsVerified:          u.IsVerified,
		MFAEnabled:          u.MFAEnabled, // FIX: map MFAEnabled
		FailedLoginAttempts: u.FailedLoginAttempts,
		LoginAttempts:       u.FailedLoginAttempts, // CR-011: alias
		IsLocked:            isLocked,              // CR-011
		LastLoginAt:         u.LastLoginAt,
		CreatedAt:           u.CreatedAt,
		Permissions:         valueobject.PermissionsFor(string(u.Role)),
	}
}


// GET /admin/users/{id} — CR-001
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "Invalid user ID"))
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("NOT_FOUND", "User not found"))
		return
	}

	writeJSON(w, http.StatusOK, toUserDTO(user))
}

// GET /admin/users
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	role := r.URL.Query().Get("role")
	isActive := r.URL.Query().Get("is_active")
	status := r.URL.Query().Get("status")
	if status == "locked" {
		isActive = "false"
	} else if status == "active" {
		isActive = "true"
	}
	q := r.URL.Query().Get("q")
	page, ps := parsePagination(r)

	filter := repository.UserFilter{
		Role:     role,
		IsActive: isActive,
		Query:    q,
		Page:     page,
		PageSize: ps,
	}

	users, total, err := h.userRepo.List(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", err.Error()))
		return
	}

	dtos := make([]UserDTO, len(users))
	for i, u := range users {
		dtos[i] = toUserDTO(u)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users":     dtos,
		"total":     total,
		"page":      page,
		"page_size": ps,
	})
}

// POST /admin/users/invite
// [FIX TASK-HC-014] Now calls real InviteUserUseCase with email sending.
func (h *AdminHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "Invalid request body"))
		return
	}
	if req.Email == "" || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "email and role are required"))
		return
	}

	validRoles := map[string]bool{"admin": true, "user": true, "readonly": true, "agent": true}
	if !validRoles[req.Role] {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "invalid role"))
		return
	}

	existing, _ := h.userRepo.FindByEmail(r.Context(), req.Email)
	if existing != nil {
		writeJSON(w, http.StatusConflict, errResp("CONFLICT", "User with this email already exists"))
		return
	}

	// [FIX TASK-HC-014] Use real InviteUserUseCase when available
	if h.inviteUC != nil {
		inviterIDStr := r.Header.Get("X-User-ID")
		inviterID, _ := uuid.Parse(inviterIDStr)
		result, err := h.inviteUC.Execute(r.Context(), ucadmin.InviteUserInput{
			Email:       req.Email,
			Name:        req.Name,
			Role:        req.Role,
			InvitedByID: inviterID,
			InviterName: r.Header.Get("X-User-Name"),
			BaseURL:     h.appBaseURL,
		})
		if err != nil {
			h.log.Error().Err(err).Msg("failed to create invitation")
			writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Failed to create invitation"))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"status":     "invitation_sent",
			"email":      result.Email,
			"user_id":    result.UserID,
			"expires_at": result.ExpiresAt,
		})
		return
	}

	// Fallback: create user without email (inviteUC not wired)
	user := &entity.User{
		Email:          req.Email,
		Username:       req.Name,
		Role:           req.Role,
		AuthProvider:   entity.AuthProviderLocal,
		IsActive:       true,
		IsVerified:     false,
		HashedPassword: "",
	}
	if err := h.userRepo.Create(r.Context(), user); err != nil {
		h.log.Error().Err(err).Msg("failed to create user")
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Failed to create user"))
		return
	}
	h.log.Warn().Str("email", req.Email).Msg("InviteUser: inviteUC not wired — email not sent")
	writeJSON(w, http.StatusCreated, toUserDTO(user))
}

// PATCH /admin/users/{id}
func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "Invalid user ID"))
		return
	}

	var req struct {
		Role     *string `json:"role"`
		IsActive *bool   `json:"is_active"`
		Name     *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "Invalid request body"))
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("NOT_FOUND", "User not found"))
		return
	}

	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Name != nil {
		user.Username = *req.Name
	}

	if err := h.userRepo.Update(r.Context(), user); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Failed to update user"))
		return
	}

	writeJSON(w, http.StatusOK, toUserDTO(user))
}

// POST /admin/users/{id}/unlock — CR-011
// Resets is_active=true, login_attempts=0, is_locked=false
func (h *AdminHandler) UnlockUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "Invalid user ID"))
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("NOT_FOUND", "User not found"))
		return
	}

	// CR-011: unlock means: re-activate AND reset failure counter
	user.IsActive = true
	user.FailedLoginAttempts = 0
	if err := h.userRepo.Update(r.Context(), user); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", "Failed to unlock user"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":            userID,
		"is_active":     true,
		"is_locked":     false,
		"login_attempts": 0,
		"success":       true,
	})
}

// GET /admin/roles — CR-011
// [FIX TASK-HC-010] Reads roles metadata and permission categories from PostgreSQL.
// Falls back to static definitions when rbacRepo is nil (backward-compat).
func (h *AdminHandler) GetRBACMatrix(w http.ResponseWriter, r *http.Request) {
	roleNames := []string{"admin", "user", "readonly", "agent"}

	// ── Permission categories: DB first, static fallback ────────────────────
	var permissionCategories []map[string]interface{}
	if h.rbacRepo != nil {
		dbCats, err := h.rbacRepo.ListPermissionCategories(r.Context())
		if err == nil && len(dbCats) > 0 {
			for _, c := range dbCats {
				permissionCategories = append(permissionCategories, map[string]interface{}{
					"category": c.Category,
					"items":    c.Items,
				})
			}
		}
	}
	if permissionCategories == nil {
		// Static fallback
		permissionCategories = []map[string]interface{}{
			{"category": "Dashboard",      "items": []string{"scan:read", "finding:read"}},
			{"category": "Scanning",       "items": []string{"scan:create", "scan:read", "scan:delete"}},
			{"category": "Findings",       "items": []string{"finding:write", "finding:read"}},
			{"category": "Reports",        "items": []string{"report:download"}},
			{"category": "AI Center",      "items": []string{"finding:write"}},
			{"category": "Administration", "items": []string{"user:manage", "system:configure"}},
			{"category": "Agent",          "items": []string{"agent:report"}},
		}
	}

	// ── Roles metadata: DB first, static fallback ────────────────────────────
	// Build name → metadata map from DB or static defaults
	roleMeta := map[string]map[string]string{
		"admin":    {"display_name": "Administrator",     "color": "#8B5CF6", "description": "Full system access"},
		"user":     {"display_name": "Security Analyst",  "color": "#3B82F6", "description": "Standard user access"},
		"readonly": {"display_name": "Read-Only Viewer",  "color": "#6B7280", "description": "View-only access"},
		"agent":    {"display_name": "Scan Agent",        "color": "#10B981", "description": "Automated scanner"},
	}
	if h.rbacRepo != nil {
		dbRoles, err := h.rbacRepo.ListRoles(r.Context())
		if err == nil && len(dbRoles) > 0 {
			roleMeta = make(map[string]map[string]string, len(dbRoles))
			for _, rm := range dbRoles {
				roleMeta[rm.Name] = map[string]string{
					"display_name": rm.DisplayName,
					"description":  rm.Description,
					"color":        rm.Color,
				}
				if !sliceContains(roleNames, rm.Name) {
					roleNames = append(roleNames, rm.Name)
				}
			}
		}
	}

	// Build structured roles list with permissions per role
	type RoleDTO struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		DisplayName string   `json:"display_name"`
		Description string   `json:"description"`
		Color       string   `json:"color"`
		UserCount   int      `json:"user_count"`
		Permissions []string `json:"permissions"`
	}

	roles := make([]RoleDTO, 0, len(roleNames))
	for _, name := range roleNames {
		perms := valueobject.PermissionsFor(name)
		if perms == nil {
			perms = []string{}
		}
		meta := roleMeta[name]
		dn := meta["display_name"]
		if dn == "" {
			dn = name
		}

		_, total, _ := h.userRepo.List(r.Context(), repository.UserFilter{
			Role:     name,
			PageSize: 1,
		})

		roles = append(roles, RoleDTO{
			ID:          name,
			Name:        name,
			DisplayName: dn,
			Description: meta["description"],
			Color:       meta["color"],
			UserCount:   total,
			Permissions: perms,
		})
	}

	// Legacy permissions matrix (backward compat)
	allPermsMap := make(map[string]bool)
	for _, name := range roleNames {
		for _, p := range valueobject.PermissionsFor(name) {
			allPermsMap[p] = true
		}
	}
	var permMatrix []map[string]interface{}
	for perm := range allPermsMap {
		roleMap := make(map[string]bool)
		for _, role := range roleNames {
			for _, p := range valueobject.PermissionsFor(role) {
				if p == perm {
					roleMap[role] = true
					break
				}
			}
		}
		permMatrix = append(permMatrix, map[string]interface{}{
			"permission":  perm,
			"description": perm,
			"roles":       roleMap,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"roles":                 roles,
		"permission_categories": permissionCategories,
		"permissions":           permMatrix,
	})
}

// CreateAPIKeyForUser handles POST /admin/users/{id}/api-keys
func (h *AdminHandler) CreateAPIKeyForUser(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_id", "invalid UUID"))
		return
	}

	var req struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid_body", err.Error()))
		return
	}

	result, err := h.apiKeyUC.CreateAPIKey(r.Context(), ucapikey.CreateRequest{
		UserID:      targetID,
		Name:        req.Name,
		Permissions: req.Scopes,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("internal", err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// Helpers
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

// ── SEED-001: Admin direct-create & bulk-create handlers ──────────────────────

// CreateUser handles POST /api/v1/admin/users
// Creates a single user directly with a password (bypasses invite flow).
// Requires X-User-Role: admin (enforced at gateway).
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email      string `json:"email"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		Role       string `json:"role"`
		IsActive   bool   `json:"is_active"`
		IsVerified bool   `json:"is_verified"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "invalid request body"))
		return
	}
	if req.Email == "" || req.Password == "" || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "email, password, and role are required"))
		return
	}

	in := entity.UserCreateInput{
		Email:      req.Email,
		Username:   req.Username,
		Password:   req.Password,
		Role:       req.Role,
		IsActive:   req.IsActive,
		IsVerified: req.IsVerified,
	}

	user, err := h.adminUC.CreateUser(r.Context(), in)
	if err != nil {
		if err.Error() == "email already exists" || contains(err.Error(), "already exists") {
			writeJSON(w, http.StatusConflict, errResp("CONFLICT", "email already registered"))
			return
		}
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", err.Error()))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":          user.ID,
		"email":       user.Email,
		"username":    user.Username,
		"role":        user.Role,
		"is_active":   user.IsActive,
		"is_verified": user.IsVerified,
		"created_at":  user.CreatedAt,
	})
}

// BulkCreateUsers handles POST /api/v1/admin/users/bulk
// Creates multiple users in one transaction, returns 207 Multi-Status.
func (h *AdminHandler) BulkCreateUsers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Users []entity.UserCreateInput `json:"users"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "invalid request body"))
		return
	}

	results, err := h.adminUC.CreateBulkUsers(r.Context(), req.Users)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", err.Error()))
		return
	}

	createdCount, failedCount := 0, 0
	for _, res := range results {
		if res.Status == "created" {
			createdCount++
		} else {
			failedCount++
		}
	}

	writeJSON(w, http.StatusMultiStatus, map[string]interface{}{
		"created_count": createdCount,
		"failed_count":  failedCount,
		"results":       results,
	})
}

// AssignRole handles POST /api/v1/admin/users/{id}/roles
// Grants a global or product-scoped role to a user.
func (h *AdminHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "invalid user ID"))
		return
	}

	actorIDStr := r.Header.Get("X-User-ID")
	actorID, _ := uuid.Parse(actorIDStr)

	var req struct {
		RoleID     int        `json:"role_id"`
		Scope      string     `json:"scope"`
		ResourceID *uuid.UUID `json:"resource_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "invalid request body"))
		return
	}
	if req.Scope == "" {
		req.Scope = "global"
	}

	assignment := entity.RoleAssignment{
		UserID:     targetID,
		RoleID:     req.RoleID,
		Scope:      req.Scope,
		ResourceID: req.ResourceID,
		AssignedBy: actorID,
	}

	if err := h.adminUC.AssignRole(r.Context(), assignment); err != nil {
		if contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, errResp("NOT_FOUND", err.Error()))
			return
		}
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": targetID,
		"role_id": req.RoleID,
		"scope":   req.Scope,
		"status":  "assigned",
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstr(s, substr))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// sliceContains reports whether ss contains target.
func sliceContains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
