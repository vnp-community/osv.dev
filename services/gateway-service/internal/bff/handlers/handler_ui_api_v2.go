// Package http — handler_ui_api_v2.go
// CR-UI: All additional routes needed for full parity with apps/osv/internal/gateway/router.go.
// This file contains handlers for v2 APIs and missing v1 routes that were in the old gateway.
package http

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// ─────────────────────────────────────────────────────────────────
// UserScopeFilter — chi middleware: injects _user_id for non-admin
// ─────────────────────────────────────────────────────────────────

// userScopeFilter injects _user_id query param so finding/product/report services
// can filter data by owner for non-admin users.
func userScopeFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		roles  := r.Header.Get("X-User-Roles")
		isAdmin := strings.Contains(roles, "Admin")
		if userID != "" && !isAdmin {
			q := r.URL.Query()
			q.Set("_user_id", userID)
			r = r.Clone(r.Context())
			r.URL.RawQuery = q.Encode()
		}
		next.ServeHTTP(w, r)
	})
}

// proxyRequestWithTimeout forwards to upstream with a custom timeout.
func (h *UIAPIHandler) proxyRequestWithTimeout(w http.ResponseWriter, r *http.Request, upstreamURL string, timeout time.Duration) {
	client := &http.Client{Timeout: timeout}
	target := upstreamURL
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	bodyBytes, _ := io.ReadAll(r.Body)
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, strings.NewReader(string(bodyBytes)))
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create upstream request", nil)
		return
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	for _, hdr := range []string{"X-User-ID", "X-User-Role", "X-User-Roles", "X-User-Permissions"} {
		if v := r.Header.Get(hdr); v != "" {
			req.Header.Set(hdr, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Upstream service unavailable", nil)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

// ─────────────────────────────────────────────────────────────────
// Identity — API Keys (rewrite /api/v1/api-keys → /api/v1/auth/api-keys)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) APIKeyList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/auth/api-keys")
}

func (h *UIAPIHandler) APIKeyCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/auth/api-keys")
}

func (h *UIAPIHandler) APIKeyDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/auth/api-keys/"+id)
}

// ─────────────────────────────────────────────────────────────────
// Identity — MFA (rewrite /api/v1/auth/mfa/... → /totp/...)
// ─────────────────────────────────────────────────────────────────

// MFASetup handles GET /api/v1/auth/mfa/setup → identity-service GET /api/v1/auth/totp/setup
// (GET returns setup info and QR code)
func (h *UIAPIHandler) MFASetup(w http.ResponseWriter, r *http.Request) {
	// identity-service has GET /api/v1/auth/totp/setup to get setup QR/secret
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/auth/totp/setup")
}

// MFAConfirm handles POST /api/v1/auth/mfa/confirm → identity-service POST /api/v1/auth/totp/verify
func (h *UIAPIHandler) MFAConfirm(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/auth/totp/verify")
}


// ─────────────────────────────────────────────────────────────────
// Identity — Profile + Sessions
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ProfileGet(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/profile")
}

func (h *UIAPIHandler) ProfilePatch(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/profile")
}

func (h *UIAPIHandler) ProfileChangePassword(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/profile/change-password")
}

func (h *UIAPIHandler) ProfileSessions(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/profile/sessions")
}

func (h *UIAPIHandler) ProfileSessionDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sessionId")
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/profile/sessions/"+id)
}

func (h *UIAPIHandler) ProfileNotifSettings(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/profile/notifications/settings")
}

func (h *UIAPIHandler) ProfileNotifSettingsPut(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/profile/notifications/settings")
}

// ─────────────────────────────────────────────────────────────────
// Admin — Users (extended)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) AdminUserBulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/users/bulk")
}

func (h *UIAPIHandler) AdminUserGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/users/"+id)
}

func (h *UIAPIHandler) AdminUserRoles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/users/"+id+"/roles")
}

func (h *UIAPIHandler) AdminUserAPIKeys(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/users/"+id+"/api-keys")
}

// ─────────────────────────────────────────────────────────────────
// Auth — me, logout (identity-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) AuthMe(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/auth/me")
}

func (h *UIAPIHandler) AuthLogout(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/auth/logout")
}

func (h *UIAPIHandler) AuthTOTPSetup(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/auth/totp/setup")
}

func (h *UIAPIHandler) AuthTOTPVerify(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/auth/totp/verify")
}

func (h *UIAPIHandler) AuthTOTPDelete(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/auth/totp")
}

// ─────────────────────────────────────────────────────────────────
// Product v1 — missing handlers (PUT/DELETE)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ProductPut(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/"+id)
}

func (h *UIAPIHandler) ProductDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/"+id)
}

// ─────────────────────────────────────────────────────────────────
// Engagement v1 — missing handlers
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) EngagementV1List(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements")
}

func (h *UIAPIHandler) EngagementV1Create(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements")
}

func (h *UIAPIHandler) EngagementV1Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements/"+id)
}

func (h *UIAPIHandler) EngagementV1Close(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements/"+id+"/close")
}

func (h *UIAPIHandler) EngagementV1Reopen(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements/"+id+"/reopen")
}

// ─────────────────────────────────────────────────────────────────
// SLA — missing routes
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) SLAConfigPut(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-configurations")
}

func (h *UIAPIHandler) SLAConfigV2Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-configurations/"+id)
}

func (h *UIAPIHandler) SLAConfigV2Put(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-configurations/"+id)
}

func (h *UIAPIHandler) SLAConfigV2Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-configurations/"+id)
}

func (h *UIAPIHandler) SLAConfigBulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-configurations/bulk")
}

func (h *UIAPIHandler) SLAConfigAssignBulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-configurations/assign-bulk")
}

func (h *UIAPIHandler) SLAConfigAssign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "product_id")
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-configurations/"+id+"/assign/"+productID)
}

func (h *UIAPIHandler) SLADashboard(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-dashboard")
}

func (h *UIAPIHandler) SLAViolations(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-violations")
}

func (h *UIAPIHandler) SLAViolationsByProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "product_id")
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-violations/"+id)
}

// ─────────────────────────────────────────────────────────────────
// Notification — SSE stream + extended routes
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) NotificationStream(w http.ResponseWriter, r *http.Request) {
	h.proxySSE(w, r, h.notificationServiceURL+"/api/v1/notifications/stream")
}

func (h *UIAPIHandler) WebhookDeliveriesByWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/webhooks/"+id+"/deliveries")
}

func (h *UIAPIHandler) WebhookStatsAll(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/webhooks/stats")
}

func (h *UIAPIHandler) NotificationRuleBulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/notification-rules/bulk")
}

func (h *UIAPIHandler) SystemNotificationRuleList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/system-notification-rules")
}

func (h *UIAPIHandler) SystemNotificationRulePut(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/system-notification-rules")
}

func (h *UIAPIHandler) WebhookBulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/webhooks/bulk")
}

func (h *UIAPIHandler) SubscriptionBulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/subscriptions/bulk")
}

func (h *UIAPIHandler) AlertCount(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/alerts/count")
}

func (h *UIAPIHandler) AlertReadAll(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/alerts/read-all")
}

// ─────────────────────────────────────────────────────────────────
// JIRA — legacy v1 routes + extended v2
// ─────────────────────────────────────────────────────────────────

// JiraConfigV2Delete deletes a jira configuration.
func (h *UIAPIHandler) JiraConfigV2Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.jiraServiceURL+"/api/v2/jira-configurations/"+id)
}

// JiraConfigPut updates the JIRA config (v1 legacy PUT /api/v1/jira/config).
func (h *UIAPIHandler) JiraConfigPut(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.jiraServiceURL+"/api/v1/jira/config")
}

func (h *UIAPIHandler) JiraIssueGetByFinding(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "finding_id")
	h.proxyRequest(w, r, h.jiraServiceURL+"/api/v2/jira-issues/"+id)
}

func (h *UIAPIHandler) JiraIssueDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.jiraServiceURL+"/api/v2/jira-issues/"+id)
}

// ─────────────────────────────────────────────────────────────────
// Audit v2 (audit-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) AuditLogV2List(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.auditServiceURL+"/api/v2/audit-log")
}

func (h *UIAPIHandler) AuditLogV2Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.auditServiceURL+"/api/v2/audit-log/"+id)
}

func (h *UIAPIHandler) AuditLogV2Resource(w http.ResponseWriter, r *http.Request) {
	typ := chi.URLParam(r, "type")
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.auditServiceURL+"/api/v2/audit-log/resource/"+typ+"/"+id)
}

func (h *UIAPIHandler) AuditLogV2Actor(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "user_id")
	h.proxyRequest(w, r, h.auditServiceURL+"/api/v2/audit-log/actor/"+uid)
}

func (h *UIAPIHandler) AuditLogV2Export(w http.ResponseWriter, r *http.Request) {
	h.proxyRequestWithTimeout(w, r, h.auditServiceURL+"/api/v2/audit-log/export", 120*time.Second)
}

// ─────────────────────────────────────────────────────────────────
// Finding v2 — lifecycle routes (finding-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) FindingV2List(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings")
}

func (h *UIAPIHandler) FindingV2Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id)
}

func (h *UIAPIHandler) FindingV2Put(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id)
}

func (h *UIAPIHandler) FindingV2Patch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id)
}

func (h *UIAPIHandler) FindingV2Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id)
}

func (h *UIAPIHandler) FindingV2Close(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id+"/close")
}

func (h *UIAPIHandler) FindingV2Reopen(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id+"/reopen")
}

func (h *UIAPIHandler) FindingV2AcceptRisk(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id+"/accept-risk")
}

func (h *UIAPIHandler) FindingV2FalsePositive(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id+"/false-positive")
}

func (h *UIAPIHandler) FindingV2OutOfScope(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id+"/out-of-scope")
}

func (h *UIAPIHandler) FindingV2Duplicates(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id+"/duplicates")
}

func (h *UIAPIHandler) FindingV2Notes(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/"+id+"/notes")
}

func (h *UIAPIHandler) FindingV2SeverityCount(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/severity_count")
}

func (h *UIAPIHandler) FindingV2Import(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/import")
}

func (h *UIAPIHandler) FindingV2Bulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/bulk")
}

// ─────────────────────────────────────────────────────────────────
// Product v2 (finding-service) — full CRUD + bulk + seed + members
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ProductV2List(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products")
}

func (h *UIAPIHandler) ProductV2Create(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products")
}

func (h *UIAPIHandler) ProductV2Put(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/"+id)
}

func (h *UIAPIHandler) ProductV2Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/"+id)
}

func (h *UIAPIHandler) ProductV2Bulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/bulk")
}

func (h *UIAPIHandler) ProductV2Import(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/import")
}

func (h *UIAPIHandler) ProductV2Seed(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/"+id+"/seed")
}

func (h *UIAPIHandler) ProductV2MemberList(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/"+id+"/members")
}

func (h *UIAPIHandler) ProductV2MemberAdd(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/"+id+"/members")
}

func (h *UIAPIHandler) ProductV2MemberDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	uid := chi.URLParam(r, "uid")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/"+id+"/members/"+uid)
}

// ─────────────────────────────────────────────────────────────────
// Product Types v2 (finding-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ProductTypeList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/product-types")
}

func (h *UIAPIHandler) ProductTypeCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/product-types")
}

func (h *UIAPIHandler) ProductTypeBulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/product-types/bulk")
}

func (h *UIAPIHandler) ProductTypeGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/product-types/"+id)
}

func (h *UIAPIHandler) ProductTypePut(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/product-types/"+id)
}

func (h *UIAPIHandler) ProductTypeDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/product-types/"+id)
}

// ─────────────────────────────────────────────────────────────────
// Product Grades v2 (finding-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ProductGradeList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/product-grades")
}

func (h *UIAPIHandler) ProductGradeGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/product-grades/"+id)
}

// ─────────────────────────────────────────────────────────────────
// Engagement v2 (finding-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) EngagementV2List(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements")
}

func (h *UIAPIHandler) EngagementV2Create(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements")
}

func (h *UIAPIHandler) EngagementV2Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements/"+id)
}

func (h *UIAPIHandler) EngagementV2Put(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements/"+id)
}

func (h *UIAPIHandler) EngagementV2Close(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements/"+id+"/close")
}

func (h *UIAPIHandler) EngagementV2Reopen(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/engagements/"+id+"/reopen")
}

// ─────────────────────────────────────────────────────────────────
// Test v2 (finding-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) TestV2List(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/tests")
}

func (h *UIAPIHandler) TestV2Create(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/tests")
}

func (h *UIAPIHandler) TestV2Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/tests/"+id)
}

func (h *UIAPIHandler) TestV2Put(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/tests/"+id)
}

func (h *UIAPIHandler) TestV2Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/tests/"+id)
}

// ─────────────────────────────────────────────────────────────────
// Risk Acceptance v2 (finding-service) — extended
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) RiskAcceptanceV2Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/risk-acceptances/"+id)
}

func (h *UIAPIHandler) RiskAcceptanceV2Put(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/risk-acceptances/"+id)
}

func (h *UIAPIHandler) RiskAcceptanceV2Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/risk-acceptances/"+id)
}

func (h *UIAPIHandler) RiskAcceptanceV2FindingRemove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fid := chi.URLParam(r, "fid")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/risk-acceptances/"+id+"/findings/"+fid+"/remove")
}

func (h *UIAPIHandler) RiskAcceptanceV2List(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/risk-acceptances")
}

func (h *UIAPIHandler) RiskAcceptanceV2Create(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/risk-acceptances")
}

// ─────────────────────────────────────────────────────────────────
// Tool Configuration v2 (finding-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ToolConfigList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/tool-configurations")
}

func (h *UIAPIHandler) ToolConfigCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/tool-configurations")
}

func (h *UIAPIHandler) ToolConfigGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/tool-configurations/"+id)
}

func (h *UIAPIHandler) ToolConfigPut(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/tool-configurations/"+id)
}

func (h *UIAPIHandler) ToolConfigDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/tool-configurations/"+id)
}

// ─────────────────────────────────────────────────────────────────
// Metrics v2 (finding-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) MetricsProducts(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/metrics/products")
}

func (h *UIAPIHandler) MetricsProductsById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/metrics/products/"+id)
}

func (h *UIAPIHandler) MetricsFindingTrends(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/metrics/findings/trends")
}

func (h *UIAPIHandler) MetricsSLACompliance(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/metrics/sla-compliance")
}

// ─────────────────────────────────────────────────────────────────
// Reports v2 (finding-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ReportV2Templates(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/reports/templates")
}

func (h *UIAPIHandler) ReportV2List(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/reports")
}

func (h *UIAPIHandler) ReportV2Create(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/reports")
}

func (h *UIAPIHandler) ReportV2Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/reports/"+id)
}

func (h *UIAPIHandler) ReportV2Download(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequestWithTimeout(w, r, h.findingServiceURL+"/api/v2/reports/"+id+"/download", 30*time.Second)
}

func (h *UIAPIHandler) ReportV2Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/reports/"+id)
}

// ─────────────────────────────────────────────────────────────────
// Scan extended — stats, history, agents (scan-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ScanStats(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/stats")
}

func (h *UIAPIHandler) ScanStatsWeekly(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/stats/weekly")
}

func (h *UIAPIHandler) ScanHistory(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/history")
}

func (h *UIAPIHandler) AgentCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/agents")
}

func (h *UIAPIHandler) AgentList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/agents")
}

func (h *UIAPIHandler) AgentGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/agents/"+id)
}

func (h *UIAPIHandler) AgentReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/agents/"+id+"/reports")
}

func (h *UIAPIHandler) ImportScan(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v2/import-scan")
}

func (h *UIAPIHandler) ReimportScan(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v2/reimport-scan")
}

func (h *UIAPIHandler) ParserList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v2/parsers")
}

func (h *UIAPIHandler) TestImportList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v2/test-imports")
}

func (h *UIAPIHandler) TestImportGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v2/test-imports/"+id)
}

// ─────────────────────────────────────────────────────────────────
// CVE/Data — public v1 routes (no auth) + write endpoints
// ─────────────────────────────────────────────────────────────────

// KEVByCVE proxies to data-service KEV by CVE ID.
func (h *UIAPIHandler) KEVByCVE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "cveId")
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v1/kev/"+id)
}

// KEVSyncStatus proxies to data-service KEV sync status.
func (h *UIAPIHandler) KEVSyncStatus(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v1/kev/sync/status")
}

// KEVCheck proxies to data-service KEV check.
func (h *UIAPIHandler) KEVCheck(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v1/kev/check")
}

func (h *UIAPIHandler) EPSSTopV1(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v1/epss/top")
}

func (h *UIAPIHandler) EPSSDistributionV1(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v1/epss/distribution")
}

func (h *UIAPIHandler) CVELastN(w http.ResponseWriter, r *http.Request) {
	n := chi.URLParam(r, "n")
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v1/cve/last/"+n)
}

func (h *UIAPIHandler) CVERecent(w http.ResponseWriter, r *http.Request) {
	tf := chi.URLParam(r, "timeframe")
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v1/cve/recent/"+tf)
}

func (h *UIAPIHandler) CVESearchV1(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v1/cve/search")
}

func (h *UIAPIHandler) CVEGetV1(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v1/cve/"+id)
}

func (h *UIAPIHandler) CVECustomCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v2/cve/custom")
}

func (h *UIAPIHandler) CVEBulkTriage(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v2/cve/bulk-triage")
}

func (h *UIAPIHandler) CVEImport(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v2/cve/import")
}

func (h *UIAPIHandler) CVETriage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v2/cve/"+id+"/triage")
}

func (h *UIAPIHandler) CVESemanticSuggestions(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/cves/search/semantic/suggestions")
}

func (h *UIAPIHandler) DBInfoV1(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v1/dbinfo")
}

// ─────────────────────────────────────────────────────────────────
// Ranking (ranking-service)
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) RankingBulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.rankingServiceURL+"/api/v1/ranking/bulk")
}

// ─────────────────────────────────────────────────────────────────
// Misc routes
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ScanTypes(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v2/scan-types")
}

func (h *UIAPIHandler) DashboardSLA(w http.ResponseWriter, r *http.Request) {
	// Forward to sla-service /api/v2/sla-dashboard
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-dashboard")
}

func (h *UIAPIHandler) DashboardRiskTrend(w http.ResponseWriter, r *http.Request) {
	// Rewrite to /internal/risk-trend on finding-service
	r = r.Clone(r.Context())
	r.URL.Path = "/internal/risk-trend"
	r.RequestURI = "/internal/risk-trend"
	if r.URL.RawQuery != "" {
		r.RequestURI = "/internal/risk-trend?" + r.URL.RawQuery
	}
	h.proxyRequest(w, r, h.findingServiceURL+"/internal/risk-trend")
}

func (h *UIAPIHandler) AIEnrichmentBatchV2(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.aiServiceURL+"/api/v1/ai/enrichment/batch")
}

// ProductV2Get is an alias for ProductGet (which already proxies to finding-service v2).
func (h *UIAPIHandler) ProductV2Get(w http.ResponseWriter, r *http.Request) {
	h.ProductGet(w, r)
}

// Ensure all imports are used
var _ = strings.TrimRight // suppress unused warning if needed
