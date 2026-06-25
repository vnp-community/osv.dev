# UI Specs — OSV Platform Frontend

Thư mục này chứa các tài liệu kỹ thuật cho frontend của OSV Platform.

## Tài liệu

| File | Mô tả |
|------|-------|
| [architecture.md](./architecture.md) | Frontend Architecture — Technology stack, folder structure, routing, state management, API integration, security, performance |
| [TDD.md](./TDD.md) | Technical Design Document — Component specs, TypeScript data types, API contracts, state machine, UX behaviors cho từng module |

## Tổng quan nhanh

### Tech Stack
- **Framework:** React 18 + TypeScript + Vite
- **Styling:** Tailwind CSS + shadcn/ui (Radix primitives)
- **State:** Zustand (auth/UI) + React Query (server state)
- **Routing:** React Router v7 (Data Router)
- **HTTP:** Axios với JWT interceptors
- **Real-time:** EventSource (SSE) cho scan progress
- **Charts:** Recharts
- **Testing:** Vitest + React Testing Library + Playwright

### Modules
1. **Auth** — JWT RS256, TOTP MFA, OAuth2 (Google/GitHub)
2. **Dashboard** — Executive KPIs, Risk Trend, Product Grades
3. **CVE Intelligence** — Search (FTS + Semantic), KEV, EPSS, Vendors, CWE/CAPEC
4. **Scanning** — Nmap, ZAP, SSE progress, Scheduling
5. **Findings** — Lifecycle management, SLA, Bulk ops, AI Triage
6. **Assets** — Asset inventory, Risk scoring, Tag management
7. **Product Security** — Product/Engagement/Test hierarchy, Grading
8. **AI Center** — Triage queue, CVE enrichment
9. **Reports** — PDF/HTML/CSV/Excel/JSON generation
10. **Notifications** — In-app, Email, Slack, Teams
11. **Integrations** — API Keys, Webhooks (HMAC), JIRA
12. **Admin** — Users, RBAC, Audit logs, System health

### API Endpoints
- `/api/v1/` — Scan, Finding, Auth, Product, Report, Asset services
- `/api/v2/` — CVE Search, KEV, Browse, Taxonomy, EPSS, Webhooks

### Design Token
- Background: `#0B1020` (base) → `#0F1629` (surface) → `#151B2F` (card)
- Brand: `#4F8CFF` (blue) + `#7C3AED` (purple)
- Severity: Critical `#EF4444`, High `#F97316`, Medium `#EAB308`, Low `#3B82F6`

## Tài liệu tham chiếu

- [PRD v3.0](../docs/PRD.md) — Product Requirements
- [SRS v3.0](../docs/SRS.md) — System Requirements & Architecture
- [URD v3.0](../docs/URD.md) — User Requirements (59 URs)
- [Frontend Source](./src/) — Current implementation
