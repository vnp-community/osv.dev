# Data Models — OSV Platform Services

> **Cập nhật**: 2026-06-19  
> **Nguồn**: Được đồng bộ trực tiếp từ source code trong `services/*/internal/domain/`  
> Tài liệu này mô tả các data models của từng microservice **không chứa code**. Mỗi file được cập nhật theo Go struct definitions thực tế.

---

## Danh sách Services

| Service | Storage | Mô tả | File |
|---------|---------|-------|------|
| [identity-service](#identity-service) | PostgreSQL, Redis | Xác thực, phân quyền, API keys | [identity-service.md](./identity-service.md) |
| [finding-service](#finding-service) | PostgreSQL | Vòng đời lỗ hổng bảo mật | [finding-service.md](./finding-service.md) |
| [data-service](#data-service) | PostgreSQL, MongoDB, Redis | Dữ liệu CVE trung tâm | [data-service.md](./data-service.md) |
| [scan-service](#scan-service) | PostgreSQL | Vulnerability scanning, agents | [scan-service.md](./scan-service.md) |
| [asset-service](#asset-service) | PostgreSQL | Asset management, tagging, risk | [asset-service.md](./asset-service.md) |
| [search-service](#search-service) | MongoDB, PostgreSQL, Redis | CVE search & browse | [search-service.md](./search-service.md) |
| [ai-service](#ai-service) | MongoDB, Redis, PostgreSQL | AI enrichment cho CVE | [ai-service.md](./ai-service.md) |
| [notification-service](#notification-service) | PostgreSQL, Redis | Thông báo đa kênh | [notification-service.md](./notification-service.md) |
| [audit-service](#audit-service) | PostgreSQL | Audit log bất biến | [audit-service.md](./audit-service.md) |
| [jira-service](#jira-service) | PostgreSQL | Tích hợp JIRA | [jira-service.md](./jira-service.md) |
| [sla-service](#sla-service) | PostgreSQL | SLA configuration | [sla-service.md](./sla-service.md) |
| [ranking-service](#ranking-service) | MongoDB | CPE-based priority ranking | [ranking-service.md](./ranking-service.md) |
| [report-service](#report-service) | PostgreSQL, S3/MinIO | Tạo báo cáo vulnerability | [report-service.md](./report-service.md) |
| [gateway-service](#gateway-service) | PostgreSQL, Redis | API Gateway, routing, auth | [gateway-service.md](./gateway-service.md) |
| [product-service](#product-service) | PostgreSQL | Product/Engagement/Test hierarchy | [product-service.md](./product-service.md) |

---

## Tổng quan kiến trúc

### Cross-service Entity Relationships

```
identity-service
  User ──────────────────────────── finding-service.ProductMember
  User ──────────────────────────── finding-service.RiskAcceptance (acceptedBy)
  APIKey ────────────────────────── gateway-service.APIKey (sync)
  Session ───────────────────────── gateway-service (JWT validation)

data-service
  CVE ────────────────────────────── ai-service.EnrichmentResult (1:1)
  CVE ────────────────────────────── search-service.CVE (read model)
  CVE ────────────────────────────── finding-service.Finding (via cve field)

product-service / finding-service (shared hierarchy)
  ProductType → Product → Engagement → Test → Finding

scan-service
  Scan ──────────────────────────── finding-service.Test (scan_id FK)
  Asset ─────────────────────────── finding-service.Finding (asset_ip/hostname)
  Agent ─────────────────────────── scan-service.AgentReport

notification-service
  NotificationRule ──────────────── finding-service events (NATS)
  Alert ─────────────────────────── identity-service.User

audit-service
  AuditEvent ────────────────────── (all services via NATS)

jira-service
  JIRAConfig ────────────────────── finding-service.Product (1:1)
  JIRAConfig ────────────────────── finding-service.Finding (sync)

sla-service
  SLAConfiguration ──────────────── finding-service.Product (1:1)
  SLAConfiguration ──────────────── finding-service.Finding (sla_expiration_date)

gateway-service
  UpstreamRoute ─────────────────── All services (routing)
  APIKey ────────────────────────── identity-service (validation)

report-service
  ReportRun ─────────────────────── scan-service.Scan
  ReportRun ─────────────────────── finding-service.Product
  ReportFinding ─────────────────── finding-service.Finding (snapshot)
```

---

## Messaging (NATS Subjects)

| Subject Pattern | Publisher | Subscribers |
|----------------|-----------|-------------|
| `defectdojo.finding.*` | finding-service | audit-service, notification-service, jira-service |
| `defectdojo.product.*` | product-service | audit-service, notification-service |
| `defectdojo.engagement.*` | finding-service | audit-service, notification-service |
| `defectdojo.risk_acceptance.*` | finding-service | audit-service, notification-service |
| `defectdojo.sla.*` | sla-service | audit-service, notification-service |
| `defectdojo.jira.*` | jira-service | audit-service, notification-service |
| `defectdojo.report.*` | report-service | audit-service |
| `identity.user.*` | identity-service | audit-service |
| `scan.import.*` | scan-service | audit-service |

---

## Storage Summary

| Storage | Services |
|---------|---------|
| **PostgreSQL** | identity, finding, data, scan, asset, notification, audit, jira, sla, report, gateway, product |
| **MongoDB** | data (cve-search), ai (enrichment), ranking |
| **Redis** | identity (sessions), gateway (cache, rate limit), search (CPE catalog), scan (agent), ai (EPSS cache) |
| **S3/MinIO** | report (artifacts), finding (risk_acceptance proof files) |

---

## identity-service

**Entities**: User, Session, APIKey, OAuthAccount, RoleAssignment  
**Key concepts**: JWT + Refresh Token rotation, MFA TOTP, OAuth2 (Google/GitHub), RBAC (5 roles)

## finding-service

**Entities**: ProductType, Product, Engagement, Test, Finding, FindingNote, FindingGroup, RiskAcceptance, SLAConfiguration, ProductMember, ProductTypeMember, ToolConfiguration  
**Key concepts**: SHA-256 deduplication, SLA breach detection, state machine, RBAC per product

## data-service

**Entities**: CVE, AffectedPackage, CVERange, CVESeverity, TriageEntry, PURL2CPE, DBState  
**Key concepts**: Dual-backend (PostgreSQL + MongoDB), VEX/triage support, multi-source (NVD/CIRCL/JVN/ExploitDB)

## scan-service

**Entities**: Scan, ScanOptions, Finding, WebAlert, DiscoveryHost, Asset (scan domain), Vulnerability, VulnSummary, Agent, AgentReport, Package, PackageCVE, Schedule  
**Key concepts**: Scan state machine, nmap + ZAP scanners, agent deployment, cron scheduling (Schedule domain: full_scan/incremental_scan/targeted_scan)

## asset-service

**Entities**: Asset, ServicePort, Vulnerability, AssetFilter, AssetCreateInput, BulkAssetResult, ScanSchedule  
**Key concepts**: Risk scoring formula, tag management (set/add/remove), BFF pattern, có domain entity riêng (không dùng từ scan-service)

## search-service

**Entities**: CVE (read model), CVESummary, SearchFilter, VendorCatalog, ProductCatalog, CWEEntry, CAPECEntry  
**Key concepts**: Multi-field filter, EPSS sort, KEV/exploit filter, Redis CPE cache, taxonomy

## ai-service

**Entities**: EnrichmentResult, EPSSSnapshot, EPSSScore, MITRETag, SeverityLevel, SeverityPrediction, FindingInput, TriageResult, TriageDecision  
**Key concepts**: AI pipeline (OpenAI/Gemini), MITRE ATT&CK mapping, CVSS-first severity classification với LLM fallback, vector embeddings (Redis cache)

## notification-service

**Entities**: Alert, AlertSubscription, NotificationRule, Webhook, WebhookDelivery, DeliveryRecord  
**Key concepts**: Multi-channel delivery (email/Slack/Teams/webhook/in-app), HMAC webhook signing, retry logic

## audit-service

**Entities**: AuditEvent  
**Key concepts**: Append-only (no Update/Delete), HMAC-SHA256 integrity, NATS-driven, compliance export

## jira-service

**Entities**: JIRAConfig  
**Key concepts**: AES-256-GCM credential encryption, priority mapping, deduplication

## sla-service

**Entities**: SLAConfiguration, SLAProductAssignment  
**Key concepts**: Per-product or global default, severity-based deadlines

## ranking-service

**Entities**: RankingEntry, GroupRank, LookupResult  
**Key concepts**: Fuzzy CPE match, group-based priority

## report-service

**Entities**: ReportRun, ReportArtifact, ReportInput, ReportFinding, ScanStats, ProductSection  
**Key concepts**: Async job, multi-format (PDF/HTML/CSV/Excel), MinIO storage, presigned URLs

## gateway-service

**Entities**: APIKey, UpstreamRoute, Principal  
**Key concepts**: Longest-prefix routing, JWT + API key auth, rate limiting, RBAC enforcement

## product-service

**Entities**: ProductType, Product, Engagement, Test  
**Key concepts**: Hierarchy (ProductType → Product → Engagement → Test), shared với finding-service
