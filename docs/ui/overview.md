# OSV Platform — UI Overview

Dựa trên PRD, URD và SRS, **OSV Platform** không chỉ là một công cụ tra cứu CVE mà là một **Security Operations Platform** gồm 5 khối sản phẩm lớn:

| Khối | Mô tả |
|------|-------|
| **Vulnerability Intelligence** | CVE, EPSS, KEV, CWE, CAPEC |
| **Active Scanning** | Nmap, ZAP, Agent |
| **Finding Management** | DefectDojo-like |
| **Asset & Product Security Management** | Quản lý tài sản và bảo mật sản phẩm |
| **Executive Reporting & Dashboard** | Báo cáo và tổng quan cho lãnh đạo |

Để Figma Make sinh giao diện tốt, cần chuyển yêu cầu nghiệp vụ thành **UI Specification + Prompt** cho từng màn hình.

---

## 1. Design System Prompt

> Prompt gốc cho Figma Make

Design a modern enterprise cybersecurity SaaS platform called **OSV Platform**.

### Style

- Similar quality level to **Wiz, Snyk, CrowdStrike Falcon, Datadog, DefectDojo Enterprise**
- Clean security-focused interface
- **Dark mode first**
- Responsive web dashboard
- Professional SOC/CISO aesthetic
- Glassmorphism cards with subtle shadows

### Color Palette

| Token | Hex |
|-------|-----|
| Background | `#0B1020` |
| Surface | `#151B2F` |
| Primary | `#4F8CFF` |
| Success | `#10B981` |
| Warning | `#F59E0B` |
| Critical | `#EF4444` |
| Text | `#E5E7EB` |

### Typography

- Font: **Inter**
- Large KPI cards
- Dense data tables
- Enterprise dashboard layout

### UX Components

- Left sidebar navigation
- Global search
- Notification center
- User profile menu
- Breadcrumbs
- Filters
- Data tables
- Interactive charts
- Real-time status indicators

---

## 2. Dashboard Home

**Mục tiêu:** Cho CISO và Security Manager xem tình hình tổng thể.

### Layout

```
┌──────────────────────────────────────────┐
│              Top Bar                      │
├────────┬─────────────────────────────────┤
│        │  KPI Cards                      │
│        │  ┌──────────┐ ┌──────────┐      │
│ Side   │  │ Critical │ │  High    │      │
│  Bar   │  │ Findings │ │ Findings │      │
│        │  └──────────┘ └──────────┘      │
│        │  ┌──────────┐ ┌──────────┐      │
│        │  │  Assets  │ │  Active  │      │
│        │  │          │ │  Scans   │      │
│        │  └──────────┘ └──────────┘      │
│        ├─────────────────────────────────┤
│        │  Risk Trend Chart               │
│        ├─────────────────────────────────┤
│        │  Severity Distribution          │
│        │  Top Vulnerable Products        │
│        ├─────────────────────────────────┤
│        │  Recent Findings Table          │
└────────┴─────────────────────────────────┘
```

### Widgets

- Product Grade (A–F)
- SLA Compliance %
- Critical Findings
- KEV Vulnerabilities
- Scan Success Rate

### Figma Prompt

Create a cybersecurity executive dashboard.

**Show KPIs:**
- Critical Findings
- High Findings
- Total Assets
- Active Scans

**Add:**
- Risk trend chart
- Severity distribution donut chart
- Product security grades A–F
- SLA compliance widget
- Recent findings table

Enterprise SaaS layout similar to **Datadog** and **Wiz**. Dark mode.

---

## 3. CVE Intelligence Center

**Mục tiêu:** Tra cứu CVE như NVD + Wiz + Snyk.

### Layout

```
┌──────────────────────────────────────────┐
│           Search Bar                      │
├──────────────────────────────────────────┤
│  Filters:  Severity | EPSS | KEV         │
│            Vendor | Product | CWE | CAPEC│
├──────────────────────────────────────────┤
│  CVE Table                               │
├───────────────────────┬──────────────────┤
│                       │ CVE-2025-12345   │
│                       │ CVSS 9.8         │
│                       │ EPSS 98%         │
│                       │                  │
│                       │ Description      │
│                       │ Affected Products│
│                       │ CWE / CAPEC      │
│                       │ Exploit Available│
│                       │ KEV Status       │
│                       │ AI Summary       │
│                       │ References       │
└───────────────────────┴──────────────────┘
```

### Figma Prompt

Design a Vulnerability Intelligence Center.

**Features:**
- Global CVE search
- Natural language search
- EPSS, KEV, Vendor/Product filters
- CVE table with side panel showing: CVSS, EPSS, AI risk analysis, CWE, CAPEC, Exploit availability, References

Modern threat intelligence interface.

---

## 4. Active Scanning Center

> Đây là module quan trọng nhất.

### Layout

```
┌──────────────────────────────────────────┐
│           Create Scan                    │
├──────────────────────────────────────────┤
│  Scan Types:  [ Nmap ]  [ ZAP ]  [ Agent]│
├──────────────────────────────────────────┤
│  Running Scans                           │
│  Progress Bar │ Status │ Duration        │
├──────────────────────────────────────────┤
│  Real-time Progress                      │
│  Host Discovery    ████████░░░  80%      │
│  Port Scan         █████░░░░░░  50%      │
│  Vuln Detection    ██░░░░░░░░░  15%      │
└──────────────────────────────────────────┘
```

### Figma Prompt

Create an Active Security Scanning dashboard.

**Include:**
- Scan creation wizard
- Scan type cards: Network Scan, Web Scan, Agent Scan

**Show:**
- Running scans with real-time progress
- Timeline, Scan queue, Scan history

Use cybersecurity monitoring design similar to **CrowdStrike** and **Rapid7**.

---

## 5. Findings Management

> DefectDojo-style.

### Layout

```
┌──────────────────────────────────────────┐
│  Filters: Severity | Status | Product    │
│           Assignee | SLA                 │
├──────────────────────────────────────────┤
│  Findings Table                          │
│  Title | CVE | Severity | Asset          │
│  Product | Status | SLA | AI Rec.        │
├──────────────────────────────────────────┤
│  Finding Detail                          │
│  Description │ Evidence │ Affected Assets│
│  CVSS │ EPSS │ History │ Comments        │
│  Audit Trail │ AI Triage Recommendation  │
├──────────────────────────────────────────┤
│  Actions: Mitigate │ Accept Risk         │
│           False Positive │ Reopen        │
└──────────────────────────────────────────┘
```

### Figma Prompt

Design a vulnerability findings management interface.

**Features:**
- DefectDojo-style findings table
- Severity filters, Status workflow, SLA indicators, AI recommendations

**Finding detail page includes:**
- Evidence, Audit history, Risk acceptance, Comments, Timeline

Professional security operations UI.

---

## 6. Asset Inventory

### Layout

```
┌──────────────────────────────────────────┐
│  KPIs: Total Assets │ Production │ Crit. │
├──────────────────────────────────────────┤
│  Asset Table                             │
│  IP │ Hostname │ OS │ Risk │ Ports │ Findings │
├──────────────────────────────────────────┤
│  Asset Detail                            │
│  IP │ Hostname │ OS                      │
│  Open Ports │ Running Services           │
│  Technologies │ Risk Score               │
│  Historical Scans │ Related Findings     │
└──────────────────────────────────────────┘
```

### Figma Prompt

Create an enterprise asset inventory dashboard.

**Show:**
- Asset risk scores, Open services, Operating systems
- Historical scans, Related vulnerabilities

Use card-based design and data-rich tables.

---

## 7. Product Security Hierarchy

> Theo DefectDojo:

```
Product Type
    ↓
Product
    ↓
Engagement
    ↓
Test
    ↓
Finding
```

### Tree Navigation

```
Applications
 ├─ Banking App
 │   ├─ Engagements
 │   ├─ Tests
 │   └─ Findings
 └─ Mobile App
```

### Figma Prompt

Design a product security management module.

**Include:**
- Product hierarchy tree
- Product grading A–F
- Engagement tracking, Test history, Security scorecards

Enterprise application portfolio view.

---

## 8. Reporting Center

### Layout

```
┌──────────────────────────────────────────┐
│  Generate Report                         │
├──────────────────────────────────────────┤
│  Type: Executive │ Technical │ Compliance│
│  Date Range │ Product │ Severity         │
├──────────────────────────────────────────┤
│  Templates                               │
├──────────────────────────────────────────┤
│  Preview: Charts │ Findings │ Recs.      │
└──────────────────────────────────────────┘
```

### Figma Prompt

Create a cybersecurity reporting center.

**Features:**
- Report generation wizard, PDF preview, Executive summary, Severity charts

**Export options:** PDF, HTML, Excel, CSV, JSON

Clean enterprise reporting experience.

---

## 9. Administration & Settings

### Sections

| Module | Mô tả |
|--------|-------|
| Users | Quản lý người dùng |
| Roles | Phân quyền RBAC |
| MFA | Xác thực đa yếu tố |
| API Keys | Quản lý API keys |
| Webhooks | Cấu hình webhooks |
| Integrations | Tích hợp bên thứ ba |
| OAuth | Xác thực OAuth |
| LDAP | Tích hợp LDAP |

### Figma Prompt

Design an enterprise security administration panel.

**Modules:**
- User management, RBAC permissions, API key management
- MFA setup, OAuth providers, LDAP integration, Webhook subscriptions

Modern SaaS admin interface.

---

## 10. Prompt Tổng — Sinh Toàn Bộ Sản Phẩm bằng Figma Make

Create a complete enterprise cybersecurity platform called **OSV Platform**.

### Platform Modules

- Vulnerability Intelligence (CVE Database, EPSS Analytics, KEV Tracking)
- Active Security Scanning
- Asset Inventory
- Vulnerability Findings Management
- Product Security Management
- Reporting & Administration

### Pages

| # | Page |
|---|------|
| 1 | Executive Dashboard |
| 2 | CVE Intelligence Center |
| 3 | Active Scanning |
| 4 | Findings Management |
| 5 | Asset Inventory |
| 6 | Product Security Hierarchy |
| 7 | Reporting Center |
| 8 | Notifications Center |
| 9 | User Profile |
| 10 | Administration |

### Requirements

- Enterprise SaaS — similar quality to **Wiz, CrowdStrike Falcon, Snyk, Datadog**
- Dark mode first — Security operations center aesthetic
- Sidebar navigation, KPI widgets, Interactive charts
- Advanced filtering, Real-time scan monitoring
- AI-assisted vulnerability triage
- Responsive design
- Design system + Component library included
- High-fidelity, production-ready screens

---

> **Bước tiếp theo:** Xây dựng hoàn chỉnh Information Architecture (Site Map) + User Flow + Wireframe cấp màn hình cho toàn bộ ~25–30 màn hình chính, sau đó chuyển thành Figma Make prompts theo từng frame cụ thể để AI sinh UI gần như 1:1 với sản phẩm thật.