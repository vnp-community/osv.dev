// Package http — CR-UI-003: CVE Intelligence API extended routes.
// Adds new endpoints and enriches existing proxy routes with is_kev/epss fields.
// CR-UI-004: Active Scanning API routes.
// CR-UI-005: Finding Management API routes.
// CR-UI-006: Asset Management API routes.
// CR-UI-007: Product Security API routes.
// CR-UI-008: AI Center API routes.
// CR-UI-009: Reports & Notifications API routes.
// CR-UI-010: Administration & Integrations API routes.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)


// ─────────────────────────────────────────────────────────────────
// UIAPIHandler — single handler struct for all CR-UI-003 to CR-UI-010
// ─────────────────────────────────────────────────────────────────

// UIAPIHandler proxies UI API calls to the appropriate microservices.
type UIAPIHandler struct {
	dataServiceURL         string
	searchServiceURL       string // search-service: /api/v2/cves/*, /api/v2/cves/search, etc.
	findingServiceURL      string
	scanServiceURL         string
	assetServiceURL        string
	productServiceURL      string
	aiServiceURL           string
	reportServiceURL       string
	notificationServiceURL string
	identityServiceURL     string
	jiraServiceURL         string
	slaServiceURL          string
	auditServiceURL        string // audit-service: /api/v2/audit-log
	rankingServiceURL      string // ranking-service: /api/v1/ranking/bulk
	httpClient             *http.Client
}

// NewUIAPIHandler creates the unified UI API handler.
func NewUIAPIHandler(urls map[string]string) *UIAPIHandler {
	// search-service URL: prefer explicit 'search' key, fall back to 'data' for backward compat.
	searchURL := strings.TrimRight(urls["search"], "/")
	if searchURL == "" {
		searchURL = strings.TrimRight(urls["data"], "/")
	}
	return &UIAPIHandler{
		dataServiceURL:         strings.TrimRight(urls["data"], "/"),
		searchServiceURL:       searchURL,
		findingServiceURL:      strings.TrimRight(urls["finding"], "/"),
		scanServiceURL:         strings.TrimRight(urls["scan"], "/"),
		assetServiceURL:        strings.TrimRight(urls["asset"], "/"),
		productServiceURL:      strings.TrimRight(urls["product"], "/"),
		aiServiceURL:           strings.TrimRight(urls["ai"], "/"),
		reportServiceURL:       strings.TrimRight(urls["report"], "/"),
		notificationServiceURL: strings.TrimRight(urls["notification"], "/"),
		identityServiceURL:     strings.TrimRight(urls["identity"], "/"),
		jiraServiceURL:         strings.TrimRight(urls["jira"], "/"),
		slaServiceURL:          strings.TrimRight(urls["sla"], "/"),
		auditServiceURL:        strings.TrimRight(urls["audit"], "/"),
		rankingServiceURL:      strings.TrimRight(urls["ranking"], "/"),
		httpClient:             &http.Client{Timeout: 30 * time.Second},
	}
}

// RegisterUIAPIRoutes mounts all CR-UI-003..010 routes on the router.
// Parameters:
//   - r       : protected chi.Router (auth required for all routes by default)
//   - admin   : admin-only chi.Router (auth + Admin role required)
//   - sseAuth : SSE chi.Router (auth accepts ?token=<jwt> for browser EventSource)
//   - h       : the UIAPIHandler instance
func RegisterUIAPIRoutes(r chi.Router, admin chi.Router, sseAuth chi.Router, h *UIAPIHandler) {
	// ── CR-UI-003: CVE Intelligence (v2) ─────────────────────────────────────
	r.Post("/api/v2/cves/search", h.CVESearch)
	r.Post("/api/v2/cves/search/semantic", h.CVESemanticSearch)
	r.Get("/api/v2/cves/aggregations", h.CVEAggregations)
	r.Get("/api/v2/cves/export", h.CVEExport)
	r.Get("/api/v2/cves/{id}", h.CVEDetail)
	r.Get("/api/v2/kev", h.KEVList)
	r.Get("/api/v2/kev/stats", h.KEVStats)
	r.Get("/api/v2/kev/ransomware", h.KEVRansomware)
	r.Get("/api/v2/epss/{cve_id}", h.EPSSGet)
	r.Get("/api/v2/epss/top", h.EPSSTop)
	r.Get("/api/v2/epss/distribution", h.EPSSDistribution)
	r.Get("/api/v2/browse", h.VendorBrowse)
	r.Get("/api/v2/browse/{vendor}", h.VendorProducts)
	r.Get("/api/v2/browse/{vendor}/{product}", h.VendorProductCVEs)
	r.Get("/api/v2/cwe", h.CWEList)
	r.Get("/api/v2/cwe/{id}", h.CWEDetail)
	r.Get("/api/v2/capec/{id}", h.CAPECDetail)
	r.Get("/api/v2/vendors", h.VendorsAutocomplete)
	r.Get("/api/v2/dbinfo", h.DBInfo)

	// ── CR-UI-004: Active Scanning ────────────────────────────────────────────
	r.Get("/api/v1/scans", h.ScanList)
	r.Post("/api/v1/scans", h.ScanCreate)
	r.Get("/api/v1/scans/scheduled", h.ScanScheduled)
	r.Post("/api/v1/scans/scheduled", h.ScanScheduledProxy)
	r.Get("/api/v1/scans/scheduled/{id}", h.ScanScheduledProxy)
	r.Put("/api/v1/scans/scheduled/{id}", h.ScanScheduledProxy)
	r.Delete("/api/v1/scans/scheduled/{id}", h.ScanScheduledProxy)
	r.Post("/api/v1/scans/import", h.ScanImport)
	r.Get("/api/v1/scans/{id}", h.ScanGet)
	r.Get("/api/v1/scans/{id}/stream", h.ScanStream)
	r.Post("/api/v1/scans/{id}/cancel", h.ScanCancel)
	r.Get("/api/v1/scans/{id}/results/nmap", h.ScanResultsNmap)
	r.Get("/api/v1/scans/{id}/results/zap", h.ScanResultsZAP)

	// ── CR-UI-005: Finding Management ─────────────────────────────────────────
	r.Get("/api/v1/findings", h.FindingList)
	r.Post("/api/v1/findings", h.FindingCreate)
	r.Post("/api/v1/findings/bulk", h.FindingBulkCreate)
	r.Get("/api/v1/findings/stats", h.FindingStats)
	r.Get("/api/v1/findings/{id}", h.FindingGet)
	r.Patch("/api/v1/findings/{id}", h.FindingUpdate)
	r.Get("/api/v1/findings/{id}/notes", h.FindingListNotes)
	r.Post("/api/v1/findings/{id}/notes", h.FindingAddNote)
	r.Get("/api/v1/findings/{id}/audit", h.FindingAudit)
	r.Post("/api/v1/findings/bulk/close", h.FindingBulkClose)
	r.Post("/api/v1/findings/bulk/reopen", h.FindingBulkReopen)
	r.Post("/api/v1/findings/bulk/assign", h.FindingBulkAssign)
	// v2 aliases used by seed script
	r.Post("/api/v2/findings", h.FindingCreate)
	r.Post("/api/v2/findings/bulk-create", h.FindingBulkCreate)
	r.Get("/api/v1/risk-acceptances", h.RiskAcceptanceList)
	r.Post("/api/v1/risk-acceptances", h.RiskAcceptanceCreate)
	r.Delete("/api/v1/risk-acceptances/{id}", h.RiskAcceptanceDelete)
	r.Get("/api/v1/sla/config", h.SLAConfigGet)
	r.Put("/api/v1/sla/config", h.SLAConfigUpdate)
	r.Get("/api/v1/sla/overview", h.SLAOverview)
	// v2 SLA — seed uses POST /api/v2/sla-configurations
	r.Get("/api/v2/sla-configurations", h.SLAConfigGet)
	r.Post("/api/v2/sla-configurations", h.SLACreate)
	r.Get("/api/v1/audit-log", h.AuditLog)

	// ── CR-UI-006: Asset Management ───────────────────────────────────────────
	r.Get("/api/v1/assets", h.AssetList)
	r.Post("/api/v1/assets", h.AssetCreate)
	r.Get("/api/v1/assets/tags", h.AssetTags)
	r.Get("/api/v1/assets/{id}", h.AssetGet)
	r.Patch("/api/v1/assets/{id}", h.AssetUpdate)
	r.Get("/api/v1/assets/{id}/findings", h.AssetFindings)

	// ── CR-UI-007: Product Security ───────────────────────────────────────────
	r.Get("/api/v1/products", h.ProductList)
	r.Post("/api/v1/products", h.ProductCreate)
	r.Get("/api/v1/products/types", h.ProductTypes)
	r.Get("/api/v1/products/grades", h.ProductGrades)
	r.Get("/api/v1/products/{id}", h.ProductGet)
	r.Patch("/api/v1/products/{id}", h.ProductUpdate)
	r.Get("/api/v1/products/{id}/engagements", h.EngagementList)
	r.Post("/api/v1/products/{id}/engagements", h.EngagementCreate)
	r.Get("/api/v1/engagements/{id}/tests", h.TestList)
	r.Post("/api/v1/engagements/{id}/tests", h.TestCreate)

	// ── CR-UI-008: AI Center ──────────────────────────────────────────────────
	r.Get("/api/v1/ai/triage/queue", h.AITriageQueue)
	r.Post("/api/v1/ai/triage/bulk-seed", h.AITriageBulkSeed)
	r.Post("/api/v1/ai/triage/{findingId}", h.AITriage)
	r.Post("/api/v1/ai/triage/{findingId}/review", h.AITriageReview)
	r.Get("/api/v1/ai/enrichment", h.AIEnrichmentList)
	r.Post("/api/v1/ai/enrichment/trigger", h.AIEnrichmentTrigger)
	r.Post("/api/v1/ai/enrichment/batch", h.AIEnrichmentBatch)
	r.Get("/api/v1/ai/enrichment/{cveId}", h.AIEnrichmentGet)
	r.Get("/api/v1/ai/insights", h.AIInsights) // BUG-011: missing route

	// ── CR-UI-009: Reports & Notifications ───────────────────────────────────
	r.Get("/api/v1/reports", h.ReportList)
	r.Post("/api/v1/reports", h.ReportCreate)
	r.Get("/api/v1/reports/{id}", h.ReportGet)
	r.Get("/api/v1/reports/{id}/download", h.ReportDownload)
	r.Delete("/api/v1/reports/{id}", h.ReportDelete)
	r.Get("/api/v1/notifications", h.NotificationList)
	r.Patch("/api/v1/notifications/{id}/read", h.NotificationMarkRead)
	r.Post("/api/v1/notifications/mark-all-read", h.NotificationMarkAllRead)
	r.Get("/api/v1/notifications/unread-count", h.NotificationUnreadCount)
	r.Get("/api/v1/webhooks", h.WebhookList)
	r.Post("/api/v1/webhooks", h.WebhookCreate)
	// CR-009: literal paths MUST be before wildcard /{id} to prevent chi 405
	r.Get("/api/v1/webhooks/deliveries", h.WebhookDeliveryList)          // flat delivery list
	r.Post("/api/v1/webhooks/deliveries/{id}/retry", h.WebhookDeliveryRetry) // retry failed
	r.Get("/api/v1/webhooks/stats/hourly", h.WebhookHourlyStats)          // 24h chart
	r.Delete("/api/v1/webhooks/{id}", h.WebhookDelete)
	r.Post("/api/v1/webhooks/{id}/test", h.WebhookTest)

	// ── CR-UI-010: Administration & Integrations ──────────────────────────────
	r.Get("/api/v1/admin/users", h.AdminUserList)
	r.Post("/api/v1/admin/users", h.AdminUserCreate)       // seed: create users
	r.Post("/api/v1/admin/users/invite", h.AdminUserInvite)
	r.Patch("/api/v1/admin/users/{id}", h.AdminUserUpdate)
	r.Post("/api/v1/admin/users/{id}/unlock", h.AdminUserUnlock)
	r.Post("/api/v1/admin/users/{id}/reset-password", h.AdminUserResetPassword)
	r.Get("/api/v1/admin/roles", h.AdminRoles)
	r.Get("/api/v1/admin/health", h.AdminHealth)
	r.Get("/api/v1/admin/settings", h.AdminSettingsGet)
	r.Patch("/api/v1/admin/settings", h.AdminSettingsUpdate)
	r.Put("/api/v1/admin/settings", h.AdminSettingsUpdate) // CR-012: PUT alias for PATCH

	r.Get("/api/v2/jira-configurations", h.JiraConfigList)
	r.Post("/api/v2/jira-configurations", h.JiraConfigCreate)
	r.Post("/api/v2/jira-configurations/bulk", h.JiraConfigBulk)
	r.Get("/api/v2/jira-configurations/{id}", h.JiraConfigGet)
	r.Put("/api/v2/jira-configurations/{id}", h.JiraConfigUpdate)
	r.Post("/api/v2/jira-configurations/{id}/test", h.JiraConfigTest)
	r.Get("/api/v2/jira-issues", h.JiraIssueList)
	r.Post("/api/v2/jira-issues", h.JiraIssueCreate)
	// CR-013: Audit log with search/severity/date filters (proxied to finding-service)
	r.Get("/api/v1/audit-log", h.AuditLog)

	// ── CR-UI-009 extended: Notification rules & subscriptions ─────────────────
	r.Get("/api/v2/notification-rules", h.NotificationRuleList)
	r.Post("/api/v2/notification-rules", h.NotificationRuleCreate)
	r.Put("/api/v2/notification-rules/{id}", h.NotificationRuleUpdate)
	r.Delete("/api/v2/notification-rules/{id}", h.NotificationRuleDelete)
	r.Get("/api/v2/subscriptions", h.SubscriptionList)
	r.Post("/api/v2/subscriptions", h.SubscriptionCreate)
	r.Delete("/api/v2/subscriptions/{id}", h.SubscriptionDelete)
	r.Get("/api/v2/alerts", h.AlertList)
	r.Post("/api/v2/alerts/{id}/read", h.AlertMarkRead)

	// ── PARITY: Auth / Identity routes missing from gateway-service ───────────
	// Auth: me, logout, TOTP, MFA aliases, API keys
	r.Get("/api/v1/auth/me", h.AuthMe)
	r.Post("/api/v1/auth/logout", h.AuthLogout)
	r.Post("/api/v1/auth/totp/setup", h.AuthTOTPSetup)
	r.Post("/api/v1/auth/totp/verify", h.AuthTOTPVerify)
	r.Delete("/api/v1/auth/totp", h.AuthTOTPDelete)
	// MFA aliases: /mfa/setup → totp/setup, /mfa/confirm → totp/verify
	r.Get("/api/v1/auth/mfa/setup", h.MFASetup)
	r.Post("/api/v1/auth/mfa/confirm", h.MFAConfirm)
	// API keys (rewrite path → /api/v1/auth/api-keys)
	r.Get("/api/v1/api-keys", h.APIKeyList)
	r.Post("/api/v1/api-keys", h.APIKeyCreate)
	r.Delete("/api/v1/api-keys/{id}", h.APIKeyDelete)

	// Profile routes
	r.Get("/api/v1/profile", h.ProfileGet)
	r.Patch("/api/v1/profile", h.ProfilePatch)
	r.Post("/api/v1/profile/change-password", h.ProfileChangePassword)
	r.Get("/api/v1/profile/sessions", h.ProfileSessions)
	r.Delete("/api/v1/profile/sessions/{sessionId}", h.ProfileSessionDelete)
	r.Get("/api/v1/profile/notifications/settings", h.ProfileNotifSettings)
	r.Put("/api/v1/profile/notifications/settings", h.ProfileNotifSettingsPut)

	// Admin — extended (missing: GET /{id}, bulk, roles, api-keys per user)
	admin.Post("/api/v1/admin/users/bulk", h.AdminUserBulk)
	admin.Get("/api/v1/admin/users/{id}", h.AdminUserGet)         // was 405
	admin.Post("/api/v1/admin/users/{id}/roles", h.AdminUserRoles)
	admin.Post("/api/v1/admin/users/{id}/api-keys", h.AdminUserAPIKeys)

	// ── PARITY: Product v1 — missing PUT/DELETE ───────────────────────────────
	r.Put("/api/v1/products/{id}", h.ProductPut)
	r.Delete("/api/v1/products/{id}", h.ProductDelete)

	// ── PARITY: Engagement v1 — missing handlers ─────────────────────────────
	r.Get("/api/v1/engagements", h.EngagementV1List)
	r.Post("/api/v1/engagements", h.EngagementV1Create)
	r.Get("/api/v1/engagements/{id}", h.EngagementV1Get)
	r.Post("/api/v1/engagements/{id}/close", h.EngagementV1Close)
	r.Post("/api/v1/engagements/{id}/reopen", h.EngagementV1Reopen)

	// ── PARITY: SLA v2 — violations, dashboard, bulk, assign ─────────────────
	// PUT /api/v1/sla/config already registered above but aliased to slaServiceURL
	admin.Put("/api/v1/sla/config", h.SLAConfigPut)                  // was 405 — needs admin
	r.Get("/api/v2/sla-configurations/{id}", h.SLAConfigV2Get)
	r.Put("/api/v2/sla-configurations/{id}", h.SLAConfigV2Put)
	r.Delete("/api/v2/sla-configurations/{id}", h.SLAConfigV2Delete)
	admin.Post("/api/v2/sla-configurations/bulk", h.SLAConfigBulk)
	admin.Post("/api/v2/sla-configurations/assign-bulk", h.SLAConfigAssignBulk)
	r.Post("/api/v2/sla-configurations/{id}/assign/{product_id}", h.SLAConfigAssign)
	r.Get("/api/v2/sla-dashboard", h.SLADashboard)
	r.Get("/api/v2/sla-violations", h.SLAViolations)
	r.Get("/api/v2/sla-violations/{product_id}", h.SLAViolationsByProduct)

	// ── PARITY: Notification — SSE stream + per-webhook + extended ────────────
	sseAuth.Get("/api/v1/notifications/stream", h.NotificationStream)
	r.Get("/api/v1/webhooks/stats", h.WebhookStatsAll)                // 405 fix alias
	r.Get("/api/v1/webhooks/{id}/deliveries", h.WebhookDeliveriesByWebhook)
	r.Post("/api/v2/notification-rules/bulk", h.NotificationRuleBulk)
	r.Get("/api/v2/system-notification-rules", h.SystemNotificationRuleList)
	r.Put("/api/v2/system-notification-rules", h.SystemNotificationRulePut)
	r.Post("/api/v2/webhooks/bulk", h.WebhookBulk)
	r.Post("/api/v2/subscriptions/bulk", h.SubscriptionBulk)
	r.Get("/api/v2/alerts/count", h.AlertCount)
	r.Post("/api/v2/alerts/read-all", h.AlertReadAll)

	// ── PARITY: JIRA legacy v1 routes ────────────────────────────────────────
	admin.Get("/api/v1/jira/config", h.JiraConfigGet)
	admin.Post("/api/v1/jira/config", h.JiraConfigCreate)
	admin.Put("/api/v1/jira/config", h.JiraConfigPut)
	admin.Post("/api/v1/jira/config/test", h.JiraConfigTest)
	// /integrations/jira alias → /jira/config
	admin.Get("/api/v1/integrations/jira", h.JiraConfigGet)
	admin.Put("/api/v1/integrations/jira", h.JiraConfigPut)
	// v2 jira extended
	r.Delete("/api/v2/jira-configurations/{id}", h.JiraConfigV2Delete)
	r.Get("/api/v2/jira-issues/{finding_id}", h.JiraIssueGetByFinding)
	r.Delete("/api/v2/jira-issues/{id}", h.JiraIssueDelete)

	// ── PARITY: Audit v2 (audit-service) ──────────────────────────────────────
	r.Get("/api/v2/audit-log", h.AuditLogV2List)
	r.Get("/api/v2/audit-log/{id}", h.AuditLogV2Get)
	r.Get("/api/v2/audit-log/resource/{type}/{id}", h.AuditLogV2Resource)
	r.Get("/api/v2/audit-log/actor/{user_id}", h.AuditLogV2Actor)
	r.Get("/api/v2/audit-log/export", h.AuditLogV2Export)

	// ── PARITY: Finding v2 — lifecycle routes (finding-service) ───────────────
	r.With(userScopeFilter).Get("/api/v2/findings", h.FindingV2List)
	r.Get("/api/v2/findings/severity_count", h.FindingV2SeverityCount)
	r.Post("/api/v2/findings/import", h.FindingV2Import)
	r.Post("/api/v2/findings/bulk", h.FindingV2Bulk)
	r.Get("/api/v2/findings/{id}", h.FindingV2Get)
	r.Put("/api/v2/findings/{id}", h.FindingV2Put)
	r.Patch("/api/v2/findings/{id}", h.FindingV2Patch)
	r.Delete("/api/v2/findings/{id}", h.FindingV2Delete)
	r.Post("/api/v2/findings/{id}/close", h.FindingV2Close)
	r.Post("/api/v2/findings/{id}/reopen", h.FindingV2Reopen)
	r.Post("/api/v2/findings/{id}/accept-risk", h.FindingV2AcceptRisk)
	r.Post("/api/v2/findings/{id}/false-positive", h.FindingV2FalsePositive)
	r.Post("/api/v2/findings/{id}/out-of-scope", h.FindingV2OutOfScope)
	r.Get("/api/v2/findings/{id}/duplicates", h.FindingV2Duplicates)
	r.Get("/api/v2/findings/{id}/notes", h.FindingV2Notes)
	r.Post("/api/v2/findings/{id}/notes", h.FindingV2Notes)

	// ── PARITY: Product v2 — full CRUD + bulk + seed + members ────────────────
	r.With(userScopeFilter).Get("/api/v2/products", h.ProductV2List)
	r.Post("/api/v2/products", h.ProductV2Create)
	r.Post("/api/v2/products/bulk", h.ProductV2Bulk)
	admin.Post("/api/v2/products/import", h.ProductV2Import)
	r.Get("/api/v2/products/{id}", h.ProductV2Get) // already proxies to finding-service
	r.Put("/api/v2/products/{id}", h.ProductV2Put)
	r.Delete("/api/v2/products/{id}", h.ProductV2Delete)
	r.Post("/api/v2/products/{id}/seed", h.ProductV2Seed)
	r.Get("/api/v2/products/{id}/members", h.ProductV2MemberList)
	r.Post("/api/v2/products/{id}/members", h.ProductV2MemberAdd)
	r.Delete("/api/v2/products/{id}/members/{uid}", h.ProductV2MemberDelete)

	// Product Types v2
	r.Get("/api/v2/product-types", h.ProductTypeList)
	r.Post("/api/v2/product-types", h.ProductTypeCreate)
	r.Post("/api/v2/product-types/bulk", h.ProductTypeBulk)
	r.Get("/api/v2/product-types/{id}", h.ProductTypeGet)
	r.Put("/api/v2/product-types/{id}", h.ProductTypePut)
	r.Delete("/api/v2/product-types/{id}", h.ProductTypeDelete)

	// Product Grades v2
	r.Get("/api/v2/product-grades", h.ProductGradeList)
	r.Get("/api/v2/product-grades/{id}", h.ProductGradeGet)

	// ── PARITY: Engagement v2 (finding-service) ───────────────────────────────
	r.Get("/api/v2/engagements", h.EngagementV2List)
	r.Post("/api/v2/engagements", h.EngagementV2Create)
	r.Get("/api/v2/engagements/{id}", h.EngagementV2Get)
	r.Put("/api/v2/engagements/{id}", h.EngagementV2Put)
	r.Post("/api/v2/engagements/{id}/close", h.EngagementV2Close)
	r.Post("/api/v2/engagements/{id}/reopen", h.EngagementV2Reopen)

	// ── PARITY: Test v2 (finding-service) ────────────────────────────────────
	r.Get("/api/v2/tests", h.TestV2List)
	r.Post("/api/v2/tests", h.TestV2Create)
	r.Get("/api/v2/tests/{id}", h.TestV2Get)
	r.Put("/api/v2/tests/{id}", h.TestV2Put)
	r.Delete("/api/v2/tests/{id}", h.TestV2Delete)

	// ── PARITY: Risk Acceptance v2 extended ───────────────────────────────────
	r.Get("/api/v2/risk-acceptances", h.RiskAcceptanceV2List)
	r.Post("/api/v2/risk-acceptances", h.RiskAcceptanceV2Create)
	r.Get("/api/v2/risk-acceptances/{id}", h.RiskAcceptanceV2Get)
	r.Put("/api/v2/risk-acceptances/{id}", h.RiskAcceptanceV2Put)
	r.Delete("/api/v2/risk-acceptances/{id}", h.RiskAcceptanceV2Delete)
	r.Post("/api/v2/risk-acceptances/{id}/findings/{fid}/remove", h.RiskAcceptanceV2FindingRemove)

	// ── PARITY: Tool Configuration v2 (finding-service) ──────────────────────
	r.Get("/api/v2/tool-configurations", h.ToolConfigList)
	r.Post("/api/v2/tool-configurations", h.ToolConfigCreate)
	r.Get("/api/v2/tool-configurations/{id}", h.ToolConfigGet)
	r.Put("/api/v2/tool-configurations/{id}", h.ToolConfigPut)
	r.Delete("/api/v2/tool-configurations/{id}", h.ToolConfigDelete)

	// ── PARITY: Metrics v2 (finding-service) ──────────────────────────────────
	r.Get("/api/v2/metrics/products", h.MetricsProducts)
	r.Get("/api/v2/metrics/products/{id}", h.MetricsProductsById)
	r.Get("/api/v2/metrics/findings/trends", h.MetricsFindingTrends)
	r.Get("/api/v2/metrics/sla-compliance", h.MetricsSLACompliance)

	// ── PARITY: Reports v2 (finding-service) ──────────────────────────────────
	r.Get("/api/v2/reports/templates", h.ReportV2Templates)
	r.With(userScopeFilter).Get("/api/v2/reports", h.ReportV2List)
	r.Post("/api/v2/reports", h.ReportV2Create)
	r.Get("/api/v2/reports/{id}", h.ReportV2Get)
	r.Get("/api/v2/reports/{id}/download", h.ReportV2Download)
	r.Delete("/api/v2/reports/{id}", h.ReportV2Delete)

	// ── PARITY: Scan extended — stats, history, agents ────────────────────────
	r.Get("/api/v1/scans/stats/weekly", h.ScanStatsWeekly)
	r.Get("/api/v1/scans/stats", h.ScanStats)
	r.Get("/api/v1/scans/history", h.ScanHistory)
	admin.Post("/api/v1/agents", h.AgentCreate)
	r.Get("/api/v1/agents", h.AgentList)
	r.Get("/api/v1/agents/{id}", h.AgentGet)
	r.Post("/api/v1/agents/{id}/reports", h.AgentReport)
	r.Post("/api/v2/import-scan", h.ImportScan)
	r.Post("/api/v2/reimport-scan", h.ReimportScan)
	r.Get("/api/v2/parsers", h.ParserList)
	r.Get("/api/v2/test-imports", h.TestImportList)
	r.Get("/api/v2/test-imports/{id}", h.TestImportGet)
	r.Get("/api/v2/scan-types", h.ScanTypes)

	// ── PARITY: CVE write endpoints (data-service) ────────────────────────────
	admin.Post("/api/v2/cve/custom", h.CVECustomCreate)
	r.Post("/api/v2/cve/bulk-triage", h.CVEBulkTriage)
	admin.Post("/api/v2/cve/import", h.CVEImport)
	r.Put("/api/v2/cve/{id}/triage", h.CVETriage)
	r.Get("/api/v2/cves/search/semantic/suggestions", h.CVESemanticSuggestions)

	// ── PARITY: Public KEV/EPSS/CVE v1 routes (no auth needed) ───────────────
	// Note: these are handled by chi with auth, but authMiddleware skips them via skipPaths
	r.Get("/api/v1/kev", h.KEVList)
	r.Get("/api/v1/kev/sync/status", h.KEVSyncStatus)
	r.Get("/api/v1/kev/check", h.KEVCheck)
	r.Get("/api/v1/kev/stats", h.KEVStats)   // NOTE: re-use existing KEVStats
	r.Get("/api/v1/kev/ransomware", h.KEVRansomware)
	r.Get("/api/v1/kev/{cveId}", h.KEVByCVE)
	r.Get("/api/v1/epss/top", h.EPSSTopV1)
	r.Get("/api/v1/epss/distribution", h.EPSSDistributionV1)
	r.Get("/api/v1/cve/last/{n}", h.CVELastN)
	r.Get("/api/v1/cve/recent/{timeframe}", h.CVERecent)
	r.Get("/api/v1/cve/search", h.CVESearchV1)
	r.Get("/api/v1/cve/{id}", h.CVEGetV1)
	r.Get("/api/v1/dbinfo", h.DBInfoV1)

	// ── PARITY: Ranking (ranking-service) ────────────────────────────────────
	admin.Post("/api/v1/ranking/bulk", h.RankingBulk)

	// ── PARITY: Dashboard sub-BFF routes ─────────────────────────────────────
	r.Get("/api/v1/dashboard/sla", h.DashboardSLA)
	r.Get("/api/v1/dashboard/risk-trend", h.DashboardRiskTrend)


	// ── CR-UI-005 extended: Finding groups ────────────────────────────────────
	r.Get("/api/v1/finding-groups", h.FindingGroupList)
	r.Post("/api/v1/finding-groups", h.FindingGroupCreate)
	r.Delete("/api/v1/finding-groups/{id}", h.FindingGroupDelete)
}

// ─────────────────────────────────────────────────────────────────
// Generic proxy helpers
// ─────────────────────────────────────────────────────────────────

// proxyRequest forwards an HTTP request to the upstream service, preserving:
//   - method, path, query, body, content-type, Authorization header, and principal headers.
func (h *UIAPIHandler) proxyRequest(w http.ResponseWriter, r *http.Request, upstreamURL string) {
	target := upstreamURL
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(bodyBytes))
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create upstream request", nil)
		return
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	// Forward principal headers set by gateway auth middleware
	for _, h := range []string{"X-User-ID", "X-User-Role", "X-User-Roles", "X-User-Permissions"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Upstream service unavailable", nil)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

// proxySSE proxies an SSE stream from the upstream service.
func (h *UIAPIHandler) proxySSE(w http.ResponseWriter, r *http.Request, upstreamURL string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "SSE not supported", nil)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), "GET", upstreamURL, nil)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create upstream request", nil)
		return
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("Accept", "text/event-stream")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		// Service unavailable — send ping-only fallback stream
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		fmt.Fprintf(w, "event: ping\ndata: {\"ts\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339))
		flusher.Flush()
		<-r.Context().Done()
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n]) //nolint:errcheck
			flusher.Flush()
		}
		if err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		default:
		}
	}
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-003: CVE Intelligence endpoints
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) CVESearch(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/cves/search")
}

func (h *UIAPIHandler) CVESemanticSearch(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/cves/search/semantic")
}

func (h *UIAPIHandler) CVEAggregations(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/cves/aggregations")
}

func (h *UIAPIHandler) CVEExport(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/cves/export")
}

func (h *UIAPIHandler) CVEDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/cves/"+id)
}

func (h *UIAPIHandler) KEVList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v2/kev")
}

func (h *UIAPIHandler) KEVStats(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v2/kev/stats")
}

func (h *UIAPIHandler) KEVRansomware(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.dataServiceURL+"/api/v2/kev/ransomware")
}

func (h *UIAPIHandler) EPSSGet(w http.ResponseWriter, r *http.Request) {
	cveID := chi.URLParam(r, "cve_id")
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/epss/"+cveID)
}

func (h *UIAPIHandler) EPSSTop(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/epss/top")
}

func (h *UIAPIHandler) EPSSDistribution(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/epss/distribution")
}

func (h *UIAPIHandler) VendorBrowse(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/browse")
}

func (h *UIAPIHandler) VendorProducts(w http.ResponseWriter, r *http.Request) {
	vendor := chi.URLParam(r, "vendor")
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/browse/"+vendor)
}

func (h *UIAPIHandler) VendorProductCVEs(w http.ResponseWriter, r *http.Request) {
	vendor := chi.URLParam(r, "vendor")
	product := chi.URLParam(r, "product")
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/browse/"+vendor+"/"+product)
}

func (h *UIAPIHandler) CWEList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/cwe")
}

func (h *UIAPIHandler) CWEDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/cwe/"+id)
}

func (h *UIAPIHandler) CAPECDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/capec/"+id)
}

func (h *UIAPIHandler) VendorsAutocomplete(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/vendors")
}

func (h *UIAPIHandler) DBInfo(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v2/dbinfo")
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-004: Active Scanning endpoints
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ScanList(w http.ResponseWriter, r *http.Request) {
	// Preserve query string (status=running, page, etc.)
	upstreamURL := h.scanServiceURL + "/api/v1/scans"
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", upstreamURL, nil)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"scans": []interface{}{}, "total": 0, "page": 1, "limit": 20,
		})
		return
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	resp, err := h.httpClient.Do(req)
	if err != nil || resp == nil {
		// scan-service unavailable — return empty but valid structure
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"scans": []interface{}{}, "total": 0, "page": 1, "limit": 20,
		})
		return
	}
	defer resp.Body.Close()
	// If scan-service stub returns 404 (route not yet implemented), return empty list
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNotImplemented {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"scans": []interface{}{}, "total": 0, "page": 1, "limit": 20,
		})
		return
	}
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}


func (h *UIAPIHandler) ScanCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans")
}

func (h *UIAPIHandler) ScanGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/"+id)
}

func (h *UIAPIHandler) ScanStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxySSE(w, r, h.scanServiceURL+"/api/v1/scans/"+id+"/stream")
}

func (h *UIAPIHandler) ScanCancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/"+id+"/cancel")
}

func (h *UIAPIHandler) ScanResultsNmap(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/"+id+"/results/nmap")
}

func (h *UIAPIHandler) ScanResultsZAP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/"+id+"/results/zap")
}

func (h *UIAPIHandler) ScanScheduledProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost || r.Method == http.MethodGet {
		h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/scheduled")
		return
	}
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/scheduled/"+id)
}

func (h *UIAPIHandler) ScanScheduled(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/scheduled")
}

func (h *UIAPIHandler) ScanImport(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.scanServiceURL+"/api/v1/scans/import")
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-005: Finding Management endpoints
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) FindingList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/findings")
}

func (h *UIAPIHandler) FindingCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings")
}

func (h *UIAPIHandler) FindingBulkCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/findings/bulk-create")
}

func (h *UIAPIHandler) FindingStats(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/findings/stats")
}

func (h *UIAPIHandler) FindingGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/findings/"+id)
}

func (h *UIAPIHandler) FindingUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/findings/"+id)
}

func (h *UIAPIHandler) FindingListNotes(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/findings/"+id+"/notes")
}

func (h *UIAPIHandler) FindingAddNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/findings/"+id+"/notes")
}

func (h *UIAPIHandler) FindingAudit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/findings/"+id+"/audit")
}

func (h *UIAPIHandler) FindingBulkClose(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/findings/bulk/close")
}

func (h *UIAPIHandler) FindingBulkReopen(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/findings/bulk/reopen")
}

func (h *UIAPIHandler) FindingBulkAssign(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/findings/bulk/assign")
}

func (h *UIAPIHandler) RiskAcceptanceList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/risk-acceptances")
}

func (h *UIAPIHandler) RiskAcceptanceCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/risk-acceptances")
}

func (h *UIAPIHandler) RiskAcceptanceDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/risk-acceptances/"+id)
}

func (h *UIAPIHandler) SLAConfigGet(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-configurations")
}

func (h *UIAPIHandler) SLACreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-configurations")
}

func (h *UIAPIHandler) SLAConfigUpdate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.slaServiceURL+"/api/v2/sla-configurations")
}

// SLAOverview handles GET /api/v1/sla/overview — returns SLA summary stats.
// Tries sla-service; returns a graceful empty response if unavailable.
func (h *UIAPIHandler) SLAOverview(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), "GET",
		h.slaServiceURL+"/api/v1/sla/overview", nil)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"total": 0, "breached": 0, "at_risk": 0, "on_track": 0, "items": []interface{}{},
		})
		return
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	resp, err := h.httpClient.Do(req)
	if err != nil || resp == nil || resp.StatusCode >= 500 || resp.StatusCode == http.StatusNotFound {
		// sla-service unavailable or route not yet implemented — return empty but valid structure
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"total": 0, "breached": 0, "at_risk": 0, "on_track": 0, "items": []interface{}{},
		})
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

// AuditLog handles GET /api/v1/audit-log — CR-013 §3.
// Supports query params: search, severity, date_from, date_to, page, page_size.
// Tries identity-service /api/v1/admin/audit-log first, falls back to empty response.
func (h *UIAPIHandler) AuditLog(w http.ResponseWriter, r *http.Request) {
	// Forward to identity-service which has audit_logs table
	targetURL := h.identityServiceURL + "/api/v1/admin/audit-log"
	req, err := http.NewRequestWithContext(r.Context(), "GET", targetURL, nil)
	if err == nil {
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
		req.Header.Set("X-User-ID", r.Header.Get("X-User-ID"))
		req.Header.Set("X-User-Role", r.Header.Get("X-User-Role"))
		// Forward all query params (search, severity, date_from, date_to, page, page_size)
		req.URL.RawQuery = r.URL.RawQuery
		resp, err2 := h.httpClient.Do(req)
		if err2 == nil && resp.StatusCode < 500 {
			defer resp.Body.Close()
			w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body) //nolint:errcheck
			return
		}
		if err2 == nil {
			resp.Body.Close()
		}
	}
	// Fallback: return empty paginated response
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries":   []interface{}{},
		"total":     0,
		"page":      1,
		"page_size": 20,
	})
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-006: Asset Management endpoints
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) AssetList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.assetServiceURL+"/api/v1/assets")
}

func (h *UIAPIHandler) AssetCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.assetServiceURL+"/api/v1/assets")
}

func (h *UIAPIHandler) AssetTags(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.assetServiceURL+"/api/v1/assets/tags")
}

func (h *UIAPIHandler) AssetGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.assetServiceURL+"/api/v1/assets/"+id)
}

func (h *UIAPIHandler) AssetUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.assetServiceURL+"/api/v1/assets/"+id)
}

func (h *UIAPIHandler) AssetFindings(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// Proxy to finding-service filtered by asset IP (asset-service returns IP)
	h.proxyRequest(w, r, h.assetServiceURL+"/api/v1/assets/"+id+"/findings")
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-007: Product Security endpoints
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ProductList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.productServiceURL+"/api/v1/products")
}

func (h *UIAPIHandler) ProductCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.productServiceURL+"/api/v1/products")
}

// ProductTypes handles GET /api/v1/products/types
// [FIX TASK-HC-008] Proxies to product-service — no longer hardcoded in gateway.
// product-service owns the product type catalog stored in PostgreSQL.
func (h *UIAPIHandler) ProductTypes(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), "GET",
		h.productServiceURL+"/api/v1/products/types", nil)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable,
			map[string]string{"error": "failed to build request to product-service"})
		return
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("X-User-ID", r.Header.Get("X-User-ID"))

	resp, err := h.httpClient.Do(req)
	if err != nil || resp == nil {
		// product-service unavailable — return minimal fallback so UI is not broken
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"types":      []interface{}{},
			"_source":    "fallback",
			"_available": false,
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode >= 500 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"types":      []interface{}{},
			"_source":    "fallback",
			"_available": false,
		})
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

// ProductGrades handles GET /api/v1/products/grades — all product grades for Scorecards.
func (h *UIAPIHandler) ProductGrades(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.productServiceURL+"/api/v1/products/grades")
}

func (h *UIAPIHandler) ProductGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// finding-service owns product detail with grade/score/critical_count enrichment
	// product-service only has CRUD without scoring
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/"+id)
}

func (h *UIAPIHandler) ProductUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v2/products/"+id)
}

func (h *UIAPIHandler) EngagementList(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// finding-service owns engagements with correct {"engagements":[]|"total":N} schema
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/products/"+id+"/engagements")
}

func (h *UIAPIHandler) EngagementCreate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/products/"+id+"/engagements")
}

func (h *UIAPIHandler) TestList(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.productServiceURL+"/api/v1/engagements/"+id+"/tests")
}

func (h *UIAPIHandler) TestCreate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.productServiceURL+"/api/v1/engagements/"+id+"/tests")
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-008: AI Center endpoints
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) AITriage(w http.ResponseWriter, r *http.Request) {
	findingID := chi.URLParam(r, "findingId")
	h.proxyRequest(w, r, h.aiServiceURL+"/api/v1/ai/triage/"+findingID)
}

func (h *UIAPIHandler) AITriageReview(w http.ResponseWriter, r *http.Request) {
	findingID := chi.URLParam(r, "findingId")
	h.proxyRequest(w, r, h.aiServiceURL+"/api/v1/ai/triage/"+findingID+"/review")
}

func (h *UIAPIHandler) AITriageQueue(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.aiServiceURL+"/api/v1/ai/triage/queue")
}

func (h *UIAPIHandler) AIEnrichmentList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.aiServiceURL+"/api/v1/ai/enrichment")
}

func (h *UIAPIHandler) AIEnrichmentTrigger(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.aiServiceURL+"/api/v1/ai/enrichment/trigger")
}

func (h *UIAPIHandler) AIEnrichmentGet(w http.ResponseWriter, r *http.Request) {
	cveID := chi.URLParam(r, "cveId")
	h.proxyRequest(w, r, h.aiServiceURL+"/api/v1/ai/enrichment/"+cveID)
}

// AIInsights handles GET /api/v1/ai/insights — BUG-011
// Returns AI-generated security insights summary for the dashboard.
// Gracefully returns empty list if ai-service is unavailable.
func (h *UIAPIHandler) AIInsights(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), "GET",
		h.aiServiceURL+"/api/v1/ai/insights", nil)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"items":   []interface{}{},
			"status":  "unavailable",
			"message": "AI service unreachable",
		})
		return
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	for _, h2 := range []string{"X-User-ID", "X-User-Role"} {
		if v := r.Header.Get(h2); v != "" {
			req.Header.Set(h2, v)
		}
	}
	if r.URL.RawQuery != "" {
		req.URL.RawQuery = r.URL.RawQuery
	}
	resp, err := h.httpClient.Do(req)
	if err != nil || resp == nil {
		// ai-service unavailable — return graceful empty response
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"items":   []interface{}{},
			"status":  "ai_unavailable",
			"message": "AI provider not configured.",
		})
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-009: Reports & Notifications endpoints
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) ReportList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.reportServiceURL+"/api/v1/reports")
}

func (h *UIAPIHandler) ReportCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.reportServiceURL+"/api/v1/reports")
}

func (h *UIAPIHandler) ReportGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.reportServiceURL+"/api/v1/reports/"+id)
}

// ReportDownload handles GET /api/v1/reports/{id}/download.
// Redirects to presigned download URL from report-service.
func (h *UIAPIHandler) ReportDownload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "pdf"
	}

	req, err := http.NewRequestWithContext(r.Context(), "GET",
		h.reportServiceURL+"/api/v1/reports/"+id+"/download?format="+format, nil)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create request", nil)
		return
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))

	resp, err := h.httpClient.Do(req)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Report service unavailable", nil)
		return
	}
	defer resp.Body.Close()

	// If upstream returns a redirect, follow it
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect {
		http.Redirect(w, r, resp.Header.Get("Location"), http.StatusFound)
		return
	}

	// Otherwise stream the body directly
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		w.Header().Set("Content-Disposition", cd)
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

func (h *UIAPIHandler) ReportDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.reportServiceURL+"/api/v1/reports/"+id)
}

func (h *UIAPIHandler) NotificationList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/notifications")
}

func (h *UIAPIHandler) NotificationMarkRead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/notifications/"+id+"/read")
}

func (h *UIAPIHandler) NotificationMarkAllRead(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/notifications/mark-all-read")
}

func (h *UIAPIHandler) NotificationUnreadCount(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/notifications/unread-count")
}

func (h *UIAPIHandler) WebhookList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/webhooks")
}

func (h *UIAPIHandler) WebhookCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/webhooks")
}

func (h *UIAPIHandler) WebhookDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/webhooks/"+id)
}

func (h *UIAPIHandler) WebhookTest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/webhooks/"+id+"/test")
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-010: Administration & Integrations endpoints
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) AdminUserList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/users")
}

func (h *UIAPIHandler) AdminUserInvite(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/users/invite")
}

func (h *UIAPIHandler) AdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/users/"+id)
}

func (h *UIAPIHandler) AdminUserUnlock(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/users/"+id+"/unlock")
}

func (h *UIAPIHandler) AdminUserResetPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/users/"+id+"/reset-password")
}

// AdminRoles handles GET /api/v1/admin/roles — full RBAC matrix with permission_categories.
// Proxies to identity-service which returns structured roles + permission_categories + legacy permissions.
func (h *UIAPIHandler) AdminRoles(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/roles")
}

// AdminHealth handles GET /api/v1/admin/health — fan-out to all service /health endpoints.
func (h *UIAPIHandler) AdminHealth(w http.ResponseWriter, r *http.Request) {
	type serviceHealth struct {
		Service string `json:"service"`
		URL     string `json:"url"`
		Status  string `json:"status"`
		Latency string `json:"latency_ms"`
	}

	services := map[string]string{
		"finding-service":      h.findingServiceURL,
		"scan-service":         h.scanServiceURL,
		"data-service":         h.dataServiceURL,
		"asset-service":        h.assetServiceURL,
		"product-service":      h.productServiceURL,
		"ai-service":           h.aiServiceURL,
		"report-service":       h.reportServiceURL,
		"notification-service": h.notificationServiceURL,
		"identity-service":     h.identityServiceURL,
	}

	results := make([]serviceHealth, 0, len(services))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, baseURL := range services {
		name, baseURL := name, baseURL
		wg.Add(1)
		go func() {
			defer wg.Done()
			sh := serviceHealth{Service: name, URL: baseURL}
			start := time.Now()
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/health", nil)
			resp, err := h.httpClient.Do(req)
			latency := time.Since(start).Milliseconds()
			sh.Latency = fmt.Sprintf("%d", latency)
			if err != nil || (resp != nil && resp.StatusCode >= 500) {
				sh.Status = "unhealthy"
			} else {
				sh.Status = "healthy"
			}
			if resp != nil {
				resp.Body.Close()
			}
			mu.Lock()
			results = append(results, sh)
			mu.Unlock()
		}()
	}
	wg.Wait()

	overall := "healthy"
	for _, r := range results {
		if r.Status != "healthy" {
			overall = "degraded"
			break
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   overall,
		"services": results,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// AdminSettings handles GET /api/v1/admin/settings.
// Returns the typed platform settings schema (4 sections).
// Tries identity-service first; falls back to scan-service (which hosts SettingsBFF);
// finally returns a default empty-but-valid settings structure.
func (h *UIAPIHandler) AdminSettingsGet(w http.ResponseWriter, r *http.Request) {
	// Try identity-service first
	req, err := http.NewRequestWithContext(r.Context(), "GET", h.identityServiceURL+"/api/v1/admin/settings", nil)
	if err == nil {
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
		req.Header.Set("X-User-ID", r.Header.Get("X-User-ID"))
		if resp, err := h.httpClient.Do(req); err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			io.Copy(w, resp.Body) //nolint:errcheck
			return
		}
	}
	// Default typed settings response — 4 sections: general/smtp/security/ai (per test spec)
	// [FIX BUG-002] ai.endpoint reads from AI_BASE_URL env var instead of localhost hardcode
	aiEndpoint := os.Getenv("AI_BASE_URL")
	if aiEndpoint == "" {
		aiEndpoint = os.Getenv("OLLAMA_BASE_URL") // legacy name
	}
	if aiEndpoint == "" {
		aiEndpoint = "http://ollama:11434" // container default — not localhost
	}
	aiModel := os.Getenv("AI_MODEL")
	if aiModel == "" {
		aiModel = "qwen2.5:1.5b"
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"general": map[string]interface{}{
			"platform_name":   "OpenVulnScan",
			"organization":    "",
			"support_email":   "",
			"timezone":        "UTC",
			"date_format":     "YYYY-MM-DD",
			"session_timeout": 30,
		},
		"smtp": map[string]interface{}{
			"enabled":    false,
			"host":       "",
			"port":       587,
			"username":   "",
			"from_email": "",
			"tls":        true,
		},
		"security": map[string]interface{}{
			"mfa_required":         false,
			"password_policy":      "medium",
			"max_login_attempts":   5,
			"lockout_duration_min": 30,
			"api_key_expiry_days":  90,
			"jwt_expiry_min":       15,
		},
		"ai": map[string]interface{}{
			"enabled":         false,
			"provider":        "ollama",
			"model":           aiModel,          // [FIX BUG-002] was: "llama3" hardcoded
			"endpoint":        aiEndpoint,        // [FIX BUG-002] was: "http://localhost:11434"
			"auto_triage":     false,
			"auto_enrichment": false,
		},
		// _meta: this is a generated default, not live config from a database
		"_meta": map[string]interface{}{"is_default": true},
	})
}

func (h *UIAPIHandler) AdminSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	// BFF validation: validate input before proxying to identity-service.
	// The identity-service does not enforce email/port/timeout validation.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Failed to read request body", nil)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var patch map[string]interface{}
	if jsonErr := json.Unmarshal(body, &patch); jsonErr == nil {
		if gen, ok := patch["general"].(map[string]interface{}); ok {
			if email, ok := gen["support_email"].(string); ok && email != "" {
				if !settingsIsValidEmail(email) {
					writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR",
						"general.support_email: invalid email format", nil)
					return
				}
			}
		}
		if sec, ok := patch["security"].(map[string]interface{}); ok {
			if minLen, ok := sec["password_min_length"].(float64); ok && minLen > 0 && minLen < 8 {
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR",
					"security.password_min_length: must be >= 8", nil)
				return
			}
			if timeout, ok := sec["session_timeout_minutes"].(float64); ok &&
				timeout > 0 && (timeout < 5 || timeout > 480) {
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR",
					"security.session_timeout_minutes: must be 5-480", nil)
				return
			}
		}
		if smtp, ok := patch["smtp"].(map[string]interface{}); ok {
			if port, ok := smtp["port"].(float64); ok && port != 0 && (port < 1 || port > 65535) {
				writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR",
					"smtp.port: must be 1-65535", nil)
				return
			}
		}
	}

	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/settings")
}

// settingsIsValidEmail validates an email address format.
func settingsIsValidEmail(email string) bool {
	at := strings.Index(email, "@")
	if at < 1 {
		return false
	}
	domain := email[at+1:]
	return strings.Index(domain, ".") >= 1
}

func (h *UIAPIHandler) JiraConfigList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.jiraServiceURL+"/api/v2/jira-configurations")
}

func (h *UIAPIHandler) JiraConfigCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.jiraServiceURL+"/api/v2/jira-configurations")
}

func (h *UIAPIHandler) JiraConfigBulk(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.jiraServiceURL+"/api/v2/jira-configurations/bulk")
}

func (h *UIAPIHandler) JiraConfigGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.jiraServiceURL+"/api/v2/jira-configurations/"+id)
}

func (h *UIAPIHandler) JiraConfigUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.jiraServiceURL+"/api/v2/jira-configurations/"+id)
}

func (h *UIAPIHandler) JiraConfigTest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.jiraServiceURL+"/api/v2/jira-configurations/"+id+"/test")
}

func (h *UIAPIHandler) JiraIssueList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/jira-issues")
}

func (h *UIAPIHandler) JiraIssueCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/jira-issues")
}

func (h *UIAPIHandler) SearchRecent(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v1/search/recent")
}

func (h *UIAPIHandler) SearchHistoryRecord(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.searchServiceURL+"/api/v1/search/history")
}

func (h *UIAPIHandler) AITriageBulkSeed(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.aiServiceURL+"/api/v1/ai/triage/bulk-seed")
}

func (h *UIAPIHandler) AIEnrichmentBatch(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.aiServiceURL+"/api/v1/ai/enrichment/batch")
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-010 extended: Admin user management
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) AdminUserCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.identityServiceURL+"/api/v1/admin/users")
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-009 extended: Notification rules & subscriptions
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) NotificationRuleList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/notification-rules")
}

func (h *UIAPIHandler) NotificationRuleCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/notification-rules")
}

func (h *UIAPIHandler) NotificationRuleUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/notification-rules/"+id)
}

func (h *UIAPIHandler) NotificationRuleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/notification-rules/"+id)
}

func (h *UIAPIHandler) SubscriptionList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/subscriptions")
}

func (h *UIAPIHandler) SubscriptionCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/subscriptions")
}

func (h *UIAPIHandler) SubscriptionDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/subscriptions/"+id)
}

func (h *UIAPIHandler) AlertList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/alerts")
}

func (h *UIAPIHandler) AlertMarkRead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v2/alerts/"+id+"/read")
}

// ─────────────────────────────────────────────────────────────────
// CR-UI-005 extended: Finding groups
// ─────────────────────────────────────────────────────────────────

func (h *UIAPIHandler) FindingGroupList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/finding-groups")
}

func (h *UIAPIHandler) FindingGroupCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/finding-groups")
}

func (h *UIAPIHandler) FindingGroupDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.findingServiceURL+"/api/v1/finding-groups/"+id)
}

// ─────────────────────────────────────────────────────────────────
// CR-009: Webhook Delivery endpoints
// GET  /api/v1/webhooks/deliveries            → flat delivery list
// POST /api/v1/webhooks/deliveries/{id}/retry → retry failed delivery
// GET  /api/v1/webhooks/stats/hourly          → 24h hourly stats chart
// ─────────────────────────────────────────────────────────────────

// WebhookDeliveryList handles GET /api/v1/webhooks/deliveries.
// Proxy to notification-service flat delivery list (filterable by webhook_id, status).
func (h *UIAPIHandler) WebhookDeliveryList(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/webhooks/deliveries")
}

// WebhookDeliveryRetry handles POST /api/v1/webhooks/deliveries/{id}/retry.
func (h *UIAPIHandler) WebhookDeliveryRetry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/webhooks/deliveries/"+id+"/retry")
}

// WebhookHourlyStats handles GET /api/v1/webhooks/stats/hourly.
func (h *UIAPIHandler) WebhookHourlyStats(w http.ResponseWriter, r *http.Request) {
	h.proxyRequest(w, r, h.notificationServiceURL+"/api/v1/webhooks/stats/hourly")
}
