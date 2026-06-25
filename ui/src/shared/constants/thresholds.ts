/**
 * Tập trung tất cả ngưỡng số nghiệp vụ (business thresholds).
 * Import từ đây thay vì hardcode magic numbers trong component.
 *
 * @see architecture.md Section 5.5 — No-Hardcode Data Rule
 */

// ─── SLA Thresholds ────────────────────────────────────────────────────────
/** >= 95% → green (on target) */
export const SLA_COMPLIANCE_GREEN = 95;
/** >= 90% → yellow (at risk) */
export const SLA_COMPLIANCE_YELLOW = 90;
/** <= 1 day remaining → critical (red) */
export const SLA_DAYS_CRITICAL = 1;
/** <= 7 days remaining → warning (yellow) */
export const SLA_DAYS_WARNING = 7;

// ─── Finding Count Thresholds ──────────────────────────────────────────────
/** > 20 findings → high severity indicator (red) */
export const FINDING_COUNT_HIGH = 20;
/** > 5 findings → medium severity indicator (yellow) */
export const FINDING_COUNT_MEDIUM = 5;

// ─── AI Confidence Thresholds ──────────────────────────────────────────────
/** > 90% confidence → high confidence (green) */
export const AI_CONFIDENCE_HIGH = 0.90;
/** > 70% confidence → medium confidence (yellow) */
export const AI_CONFIDENCE_MEDIUM = 0.70;

// ─── Webhook Performance Thresholds ────────────────────────────────────────
/** > 1000ms response time → slow delivery (red) */
export const WEBHOOK_SLOW_MS = 1000;

// ─── Risk Acceptance Thresholds ────────────────────────────────────────────
/** <= 30 days until expiration → expiring soon (yellow warning) */
export const RISK_DAYS_EXPIRING = 30;

// ─── CVSS / Risk Score Thresholds ──────────────────────────────────────────
/** >= 9.0 → Critical severity */
export const CVSS_CRITICAL = 9.0;
/** >= 7.0 → High severity */
export const CVSS_HIGH = 7.0;
/** >= 4.0 → Medium severity */
export const CVSS_MEDIUM = 4.0;

// ─── EPSS Thresholds ───────────────────────────────────────────────────────
/** >= 0.7 → High exploitation probability */
export const EPSS_HIGH = 0.7;
/** >= 0.3 → Medium exploitation probability */
export const EPSS_MEDIUM = 0.3;
