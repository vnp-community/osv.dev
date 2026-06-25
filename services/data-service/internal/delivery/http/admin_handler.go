// admin_handler.go — Admin HTTP endpoints for data-service.
// Provides OSV schema validation at POST /admin/validate.
//
// This is ADDITIVE — does not modify existing delivery code.
// Mount at /admin/* in data-service HTTP router.
package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ossf/osv-schema/bindings/go/osvschema"
)

// AdminHandler provides admin-level HTTP endpoints for data-service.
type AdminHandler struct{}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler() *AdminHandler {
	return &AdminHandler{}
}

// ValidationResponse is the response from POST /admin/validate.
type ValidationResponse struct {
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors"`
	Warnings   []string `json:"warnings,omitempty"`
	CheckedAt  string   `json:"checked_at"`
}

// ServeHTTP dispatches admin requests.
func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin")
	switch {
	case r.Method == http.MethodPost && (path == "/validate" || path == "/validate/"):
		h.validateOSV(w, r)
	default:
		writeAdminError(w, http.StatusNotFound, "unknown admin endpoint: "+path)
	}
}

// validateOSV validates an OSV JSON record.
// POST /admin/validate — Body: OSV JSON record
// Response: {"valid": true/false, "errors": [...], "warnings": [...]}
func (h *AdminHandler) validateOSV(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB max
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	resp := ValidationResponse{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// JSON structure validation
	var vuln osvschema.Vulnerability
	if err := json.Unmarshal(body, &vuln); err != nil {
		resp.Valid = false
		resp.Errors = []string{"invalid JSON: " + err.Error()}
		writeAdminJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}

	// Semantic validation
	resp.Errors, resp.Warnings = validateOSVSemantics(&vuln)
	resp.Valid = len(resp.Errors) == 0

	status := http.StatusOK
	if !resp.Valid {
		status = http.StatusUnprocessableEntity
	}
	writeAdminJSON(w, status, resp)
}

// validateOSVSemantics checks semantic constraints of an OSV vulnerability.
// Returns (errors, warnings).
func validateOSVSemantics(vuln *osvschema.Vulnerability) (errs, warnings []string) {
	// Required fields
	if vuln.GetId() == "" {
		errs = append(errs, "id: required field is empty")
	}
	if vuln.GetPublished() == nil {
		errs = append(errs, "published: required timestamp is missing")
	}
	if vuln.GetModified() == nil {
		errs = append(errs, "modified: required timestamp is missing")
	}
	if len(vuln.GetAffected()) == 0 {
		errs = append(errs, "affected: at least one entry is required")
	}

	// Recommended fields
	if vuln.Summary == "" {
		warnings = append(warnings, "summary: recommended field is missing")
	}
	if vuln.Details == "" {
		warnings = append(warnings, "details: recommended field is missing")
	}

	// ID format check
	id := vuln.GetId()
	if id != "" {
		knownPrefixes := []string{"CVE-", "GHSA-", "GO-", "OSV-", "PYSEC-", "RUSTSEC-", "CURL-", "GSD-"}
		known := false
		for _, p := range knownPrefixes {
			if strings.HasPrefix(id, p) {
				known = true
				break
			}
		}
		if !known {
			warnings = append(warnings, "id: unrecognized prefix (expected CVE-, GHSA-, GO-, OSV-, etc.)")
		}
	}

	// Timestamps consistency
	if vuln.GetPublished() != nil && vuln.GetModified() != nil {
		pub := vuln.GetPublished().AsTime()
		mod := vuln.GetModified().AsTime()
		if mod.Before(pub) {
			errs = append(errs, "modified: cannot be before published")
		}
	}

	// Validate affected packages
	for i, affected := range vuln.GetAffected() {
		pkg := affected.GetPackage()
		if pkg == nil {
			errs = append(errs, fmt.Sprintf("affected[%d].package: required", i))
			continue
		}
		if pkg.GetName() == "" {
			errs = append(errs, fmt.Sprintf("affected[%d].package.name: required", i))
		}
		if pkg.GetEcosystem() == "" {
			warnings = append(warnings, fmt.Sprintf("affected[%d].package.ecosystem: recommended", i))
		}
	}

	return
}

func writeAdminJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeAdminError(w http.ResponseWriter, status int, msg string) {
	writeAdminJSON(w, status, map[string]string{"error": msg})
}
