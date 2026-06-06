// Package auth provides DefectDojo JWT claims parsing.
// Extends the OSV JWT validator with DD-specific RBAC fields.
package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// DDClaims extends standard JWT RegisteredClaims with DefectDojo RBAC fields.
// These fields are populated by the Identity Service when issuing tokens.
type DDClaims struct {
	UserID     string   `json:"sub"`
	Email      string   `json:"email"`
	Role       string   `json:"role"`        // primary role string (OSV compat)
	Roles      []string `json:"roles"`       // all role names the user holds
	IsSuper    bool     `json:"is_super"`    // superuser — bypasses all RBAC checks
	GlobalRole string   `json:"global_role"` // system-level role name
	jwt.RegisteredClaims
}
