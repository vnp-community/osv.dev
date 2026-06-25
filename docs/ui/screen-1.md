# OSV Platform — UI Prompts (Phase 1)

Để Figma Make cho kết quả tốt nhất, mỗi Frame nên có một prompt riêng, mô tả rõ:
- Mục tiêu màn hình
- Layout
- Thành phần UI
- Data giả lập
- Hành vi tương tác
- Design system

Dưới đây là format mà Figma Make hiểu tốt nhất.

---

## FRAME 01 — LOGIN

Design a modern enterprise cybersecurity platform login page for **OSV Platform**.

**Style:**
- Dark mode
- Inspired by Wiz, CrowdStrike Falcon and Datadog
- Professional security operations center aesthetic

**Layout:** Split screen

**Left side:**
- Large OSV Platform logo
- Tagline: "The Complete Vulnerability Intelligence and Scanning Platform"
- Security illustration with cyber defense dashboard

**Right side (Login Card):**
- Fields: Email, Password, Remember Me
- Buttons: Sign In, Continue with Google, Continue with GitHub
- Links: Forgot Password, Create Account
- Additional: MFA verification flow, Security badges, Footer version information

Responsive desktop layout.

---

## FRAME 02 — EXECUTIVE DASHBOARD

Design the Executive Security Dashboard for **OSV Platform**.

**Layout:**
- Top Navigation
- Left Sidebar
- Main Dashboard Content

**Top KPI Row:**
- Critical Findings, High Findings, Total Assets, Active Scans, Product Security Grade, SLA Compliance

**Charts:**
- Risk Trend Over Time, Severity Distribution Donut, Top Vulnerable Products, EPSS Distribution

**Tables (Recent Critical Findings):**
- Columns: CVE, Severity, Product, Asset, Status

**Widgets:**
- KEV Alerts, Upcoming SLA Breaches, Recent Scans

Dark mode enterprise SaaS. Inspired by Wiz and Datadog.

---

## FRAME 03 — CVE SEARCH

Design a Vulnerability Intelligence Center.

**Purpose:** Search and analyze CVEs.

**Layout:** Left Filter Panel, Main Results Area, Right Detail Drawer

**Filters:**
- Severity, CVSS, EPSS, KEV, Vendor, Product, CWE, CAPEC

**Top Search:**
- Natural language search (e.g., "Find vulnerabilities similar to Log4Shell")

**Main Table:**
- Columns: CVE ID, Severity, CVSS, EPSS, KEV, Vendor, Updated Date

**Right Drawer (Selected CVE):**
- Show: Description, AI Summary, CVSS, EPSS, Exploit Available, CWE, CAPEC, References

Threat intelligence style interface.

---

## FRAME 04 — KEV CATALOG

Design a CISA KEV Tracking dashboard.

**Show:**
- Total KEV Vulnerabilities, New KEV This Week, Known Ransomware Count

**Filters:** Vendor, Severity, Ransomware, Date Added

**Main Table:**
- Columns: CVE, Vendor, Product, Date Added, Ransomware Usage, Required Action

**Side Drawer (KEV Detail):**
- Include: Description, Exploitation Status, CISA Guidance, References

Professional threat intelligence UI.

---

## FRAME 05 — SCAN DASHBOARD

Design a Security Scanning Operations Center.

**Purpose:** Manage active scans.

**Top Actions:** New Scan, Import Scan, Scheduled Scans

**KPI Cards:** Running Scans, Completed Today, Failed Scans, Assets Scanned

**Main Areas:** Running Scans, Recent Scans, Scheduled Scans

**Charts:** Scan Activity, Scan Success Rate

Use cybersecurity monitoring dashboard style.

---

## FRAME 06 — CREATE SCAN WIZARD

Design a multi-step scan creation wizard.

**Step 1: Choose Scan Type**
- Cards: Network Scan (Nmap), Web Application Scan (OWASP ZAP), Agent Scan

**Step 2: Targets**
- Input: IP Range, CIDR, Hostname, URL

**Step 3: Configuration**
- Options: Service Detection, OS Detection, Vulnerability Scripts, Concurrency, Timeout

**Step 4: Review**
- Summary card, Start Scan button

Modern wizard experience.

---

## FRAME 07 — RUNNING SCAN DETAIL

Design a real-time scan monitoring screen.

**Header:** Scan Name, Status, Duration

**Progress Components:**
- Host Discovery, Port Scan, Service Detection, Vulnerability Detection

**Live Logs Panel, Timeline, Discovered Assets, Detected CVEs**

**Actions:** Pause, Cancel, Export

Real-time SOC monitoring experience.

---

## FRAME 08 — FINDINGS LIST

Design a vulnerability findings management page. Inspired by DefectDojo Enterprise.

**Filters:** Severity, Status, Product, Asset, SLA, Assignee

**Bulk Actions:** Close, Reopen, Accept Risk, Tag

**Table Columns:**
- Title, CVE, Severity, Product, Asset, Status, SLA, AI Recommendation

Enterprise data table optimized for analysts.

---

## FRAME 09 — FINDING DETAIL

Design a detailed vulnerability finding page.

**Header:** Finding Title, Severity Badge

**Tabs:** Overview, Evidence, Timeline, Audit Trail, Comments

**Overview:** CVSS, EPSS, KEV Status, Affected Assets, Affected Products

**Evidence Section:** Nmap Results, ZAP Evidence, Screenshots

**AI Section:** Recommendation, Confidence Score, Suggested Actions

**Workflow Actions:** Mitigate, Accept Risk, Mark False Positive, Reopen

Professional triage workflow.

---

## FRAME 10 — ASSET INVENTORY

Design an enterprise asset inventory page.

**KPI Cards:** Total Assets, Critical Assets, Production Assets, High Risk Assets

**Filters:** Tags, Risk Score, Operating System, Environment

**Asset Table Columns:** IP, Hostname, OS, Risk Score, Open Ports, Last Scan, Tags

Asset-centric cybersecurity interface.

---

## FRAME 11 — ASSET DETAIL

Design an Asset Security Profile page.

**Header:** Asset Name, IP Address

**Overview Cards:** Risk Score, Active Findings, Open Ports, Last Scan

**Sections:** Operating System, Services, Technologies, Historical Scans, Related Findings, Asset Timeline, Risk Trend

Modern cyber asset management interface.

---

## FRAME 12 — PRODUCT SECURITY

Design a Product Security Management module.

**Left Panel:** Product Hierarchy Tree (Product Type > Product > Engagement > Test)

**Main Area:** Selected Product Overview

**Cards:** Security Grade, Critical Findings, High Findings, SLA Status

**Tabs:** Engagements, Tests, Findings, Risk Acceptance

Portfolio security dashboard.

---

## FRAME 13 — PRODUCT DETAIL

Design a Product Security Scorecard page.

**Header:** Product Name

**Cards:** Grade, Risk Score, Critical Findings, SLA Compliance

**Charts:** Findings Trend, Severity Distribution, Risk Trend

**Sections:** Engagement History, Test History, Open Findings

Executive product security view.

---

## FRAME 14 — AI TRIAGE CENTER

Design an AI Security Assistant dashboard.

**Purpose:** Review AI-generated triage recommendations.

**Queue Table Columns:** Finding, AI Verdict, Confidence, Severity, Created

**Detail Panel:** AI Analysis, Reasoning, Suggested Fixes, Related CVEs

**Actions:** Accept Recommendation, Reject Recommendation, Manual Review

Modern AI copilot experience.

---

## FRAME 15 — REPORT CENTER

Design a Security Reporting Center.

**Top Action:** Generate Report

**Templates:** Executive, Technical, Compliance

**Filters:** Product, Date Range, Severity

**Generated Reports Table Columns:** Name, Type, Created, Status, Download

**Preview Panel:** PDF Preview

Professional reporting experience.

---

## FRAME 16 — NOTIFICATION CENTER

Design a cybersecurity notification center.

**Categories:** Critical Vulnerabilities, SLA Breaches, KEV Updates, Scan Completion

**Timeline View, Severity Badges**

**Filters:** Type, Product, Severity

Modern notification inbox.

---

## FRAME 17 — API KEY MANAGEMENT

Design an API Key Management page.

**Table Columns:** Name, Prefix, Scope, Created, Last Used, Expiration

**Actions:** Create Key, Revoke Key, Rotate Key

**Create Key Modal:** Name, Scope Selection, Expiration

Enterprise developer portal experience.

---

## FRAME 18 — WEBHOOK MANAGEMENT

Design a Webhook Management dashboard.

**Webhook Table Columns:** Endpoint, Status, Events, Last Delivery

**Detail Panel:** HMAC Secret, Retry Statistics, Delivery History

**Actions:** Create Webhook, Test Webhook, Disable

Modern integration dashboard.

---

## FRAME 19 — USER MANAGEMENT

Design an enterprise user management page.

**Table Columns:** User, Email, Role, MFA Status, Last Login

**Roles:** Admin, User, Readonly, Agent

**Actions:** Invite User, Disable User, Reset MFA

Enterprise SaaS administration interface.

---

## FRAME 20 — AUDIT LOGS

Design a security audit log viewer.

**Filters:** User, Action, Resource, Date

**Timeline Table Columns:** Timestamp, User, Action, Before, After, Severity

Immutable audit trail interface similar to enterprise SIEM products.

---

## FRAME 21 — DESIGN SYSTEM

Create a complete design system for **OSV Platform**.

**Components:**
- Navigation: Sidebar, Topbar, Breadcrumb
- Cards: KPI Card, Risk Card, Alert Card, Grade Card
- Tables: Findings Table, CVE Table, Asset Table
- Charts: Severity Donut, Risk Trend, EPSS Trend, SLA Trend
- Forms: Scan Wizard, Report Wizard
- Modals: Create Scan, Accept Risk, Generate Report

**Theme:** Dark mode first
**Colors:** Critical Red, Warning Orange, Success Green, Primary Blue
**Typography:** Inter

Enterprise cybersecurity SaaS quality.