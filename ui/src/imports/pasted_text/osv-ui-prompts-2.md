# OSV Platform — UI Prompts (Phase 2 & 3)

Để đạt mức production-ready SaaS theo SRS hiện tại, 21 frame là chưa đủ. OSV Platform thực tế cần khoảng 45–60 màn hình.

Dưới đây là Phase 2 & Phase 3 Figma Make Prompts để hoàn thiện toàn bộ hệ thống.

---

## FRAME 22 — GLOBAL SEARCH CENTER

Design a universal security search experience for **OSV Platform**.

**Purpose:** Search across all platform entities (CVEs, Findings, Assets, Products, Engagements, Users, Reports).

**Layout:**
- Global Search Bar
- Recent Searches, Suggested Searches
- Results grouped by category.

**Each result card displays:** Type icon, Name, Description, Last Updated.
**Support:** Keyboard navigation.

Inspired by Linear command palette and Datadog search.

---

## FRAME 23 — SEMANTIC CVE SEARCH

Design an AI-powered semantic vulnerability search page.

**Search Example:** "Find vulnerabilities similar to Log4Shell"

**Layout:**
- Large AI Search Bar
- Suggested Queries, Results Grid

**Each Result:** CVE, Similarity Score, CVSS, EPSS, Vendor, AI Summary

**Right Panel:** Explain why the CVE matches the query.

Modern AI search experience similar to Perplexity and Wiz.

---

## FRAME 24 — VENDOR CATALOG

Design a vulnerability vendor catalog page.

**Purpose:** Browse vendors and security posture.

**Cards:** Microsoft, Oracle, Cisco, VMware, Apache

**Each Card:** Total CVEs, Critical CVEs, KEV Count, Average EPSS

**Support:** Search, Sorting, Pagination

Threat intelligence visual style.

---

## FRAME 25 — PRODUCT CATALOG

Design a product vulnerability catalog.

**Layout:** Vendor Selection, Product Grid

**Each Product Card:** Product Name, Total CVEs, Critical CVEs, Last Updated

**Selecting a product opens:** Vulnerability list, Timeline, EPSS trend

---

## FRAME 26 — CWE LIBRARY

Design a CWE knowledge base.

**Table Columns:** CWE ID, Name, Severity Impact, Linked CVEs, CAPEC Count

**Detail Drawer:** Description, Mitigation, Related CAPEC, Related CVEs

Modern security taxonomy UI.

---

## FRAME 27 — CAPEC LIBRARY

Design a CAPEC attack pattern explorer.

**Cards:** CAPEC ID, Attack Name, Likelihood, Severity

**Detail View:** Description, Execution Flow, Mitigations, Related CVEs

Threat intelligence design.

---

## FRAME 28 — EPSS ANALYTICS

Design an EPSS analytics dashboard.

**Widgets:** Average EPSS, High Risk CVEs, EPSS Trend

**Charts:** Distribution, Top EPSS CVEs, Heatmap

Interactive analytics experience.

---

## FRAME 29 — KEV ANALYTICS

Design a KEV intelligence dashboard.

**Metrics:** KEV Total, Known Ransomware, New This Month, Top Vendors

**Charts:** KEV Growth, Vendor Breakdown, Exploitation Timeline

Executive intelligence dashboard.

---

## FRAME 30 — SCAN HISTORY

Design a scan history management page.

**Filters:** Date, Type, Status, User

**Table Columns:** Scan ID, Target, Type, Duration, Findings, Status

**Actions:** View, Export, Duplicate

---

## FRAME 31 — SCHEDULED SCANS

Design a scheduled scanning management page.

**Cards:** Upcoming Scans, Failed Schedules, Weekly Runs

**Table Columns:** Name, Targets, Cron, Next Run, Status

**Actions:** Edit, Disable, Run Now

---

## FRAME 32 — NMAP RESULT VIEWER

Design a Nmap scan result explorer.

**Layout:** Hosts List, Selected Host Detail

**Show:** IP, Hostname, OS, Open Ports, Services, Detected CVEs, Risk Score

Technical security analyst experience.

---

## FRAME 33 — OWASP ZAP RESULTS

Design a web application vulnerability result viewer.

**Sections:** Alerts, Evidence, Requests, Responses, Risk Breakdown

**Alert Detail:** XSS, SQL Injection, CSRF, Authentication Issues

Professional pentesting dashboard.

---

## FRAME 34 — AGENT MANAGEMENT

Design a remote security agent management dashboard.

**Metrics:** Connected Agents, Offline Agents, Reported Findings

**Table Columns:** Agent, Hostname, Version, Last Check-in, Status

**Actions:** Update, Disable, View Logs

---

## FRAME 35 — AGENT DETAIL

Design an agent profile page.

**Show:** Agent Information, Host Information, Installed Packages, Reported Findings, Last Reports, Timeline

Modern endpoint security UI.

---

## FRAME 36 — SLA DASHBOARD

Design a service level agreement dashboard.

**Widgets:** Compliant Findings, Breached Findings, Upcoming Breaches

**Charts:** SLA Trend, Product Compliance, Risk by SLA

Executive management style.

---

## FRAME 37 — SLA DETAIL

Design a finding SLA detail page.

**Show:** Current Status, Expiration Date, Time Remaining, Assigned Owner, History, Escalations

Timeline visualization.

---

## FRAME 38 — RISK ACCEPTANCE CENTER

Design a risk acceptance management page.

**Table Columns:** Finding, Product, Reason, Expiration, Owner, Status

**Actions:** Approve, Reject, Extend

---

## FRAME 39 — RISK ACCEPTANCE DETAIL

Design a detailed risk acceptance workflow.

**Sections:** Business Justification, Risk Analysis, Expiration, Approvals, Related Findings

Approval workflow design.

---

## FRAME 40 — AI ENRICHMENT CENTER

Design an AI vulnerability enrichment dashboard.

**Metrics:** Enriched CVEs, Pending Analysis, Failed Jobs

**Panels:** AI Summary, Severity Prediction, MITRE Mapping, Exploit Analysis

Modern AI operations dashboard.

---

## FRAME 41 — AI FINDING REVIEW

Design a human-in-the-loop AI review workflow.

**Split Layout:** Finding, AI Analysis, Human Decision

**Buttons:** Approve, Reject, Modify

Enterprise AI governance UX.

---

## FRAME 42 — EXECUTIVE REPORT BUILDER

Design an executive report builder.

**Steps:** Template, Date Range, Products, Severity, Preview, Export

PDF-style experience.

---

## FRAME 43 — REPORT PREVIEW

Design a professional cybersecurity report preview.

**Sections:** Executive Summary, Risk Metrics, Severity Charts, Critical Findings, Recommendations

Print-friendly design.

---

## FRAME 44 — REPORT LIBRARY

Design a report management library.

**Cards:** Generated Reports, Scheduled Reports, Shared Reports

**Actions:** Download, Duplicate, Delete, Share

---

## FRAME 45 — NOTIFICATION INBOX

Design a security operations notification center.

**Categories:** Critical Findings, SLA, KEV, Scan, System

**Timeline layout.** Modern enterprise inbox.

---

## FRAME 46 — WEBHOOK EVENT VIEWER

Design a webhook monitoring dashboard.

**Metrics:** Success Rate, Failed Deliveries, Retries

**Table Columns:** Event, Endpoint, Status, Response Time, Delivery Logs

---

## FRAME 47 — JIRA INTEGRATION

Design a Jira integration management page.

**Show:** Connected Projects, Mapped Fields, Sync Status, Ticket Statistics

Enterprise integration interface.

---

## FRAME 48 — USER PROFILE

Design a user profile and preferences page.

**Tabs:** Profile, Security, Notifications, API Keys, Sessions

Enterprise SaaS account management.

---

## FRAME 49 — MFA SETUP

Design a multi-factor authentication setup flow.

**Steps:** Generate QR, Verify Code, Backup Codes, Success

Security-first UX.

---

## FRAME 50 — RBAC MANAGEMENT

Design a role and permission management page.

**Roles:** Admin, User, Readonly, Agent

**Matrix View:** Permissions vs Roles

Enterprise IAM interface.

---

## FRAME 51 — AUDIT TIMELINE

Design a security audit timeline explorer.

**Timeline:** User Actions, Finding Changes, System Events

**Filters:** User, Entity, Date

Similar to enterprise SIEM products.

---

## FRAME 52 — SYSTEM HEALTH

Design a platform observability dashboard.

**Metrics:** API Latency, Service Health, Queue Status, Database Health

**Charts:** Traffic, Errors, Availability

Inspired by Grafana and Datadog.

---

## FRAME 53 — MICROSERVICE MONITORING

Design a microservices monitoring dashboard.

**Services:** Auth, Scan, Finding, Report, AI, Notification

**Show:** Latency, Errors, Requests, Health

Technical operations center.

---

## FRAME 54 — EVENT STREAM MONITOR

Design a NATS JetStream monitoring page.

**Show:** Subjects, Consumers, Message Rate, Failures

Real-time event flow visualization.

---

## FRAME 55 — SECURITY SETTINGS

Design a platform security settings page.

**Sections:** Password Policy, Session Policy, MFA Policy, API Key Policy, Rate Limits

Enterprise security administration.

---

## FRAME 56 — SYSTEM SETTINGS

Design a platform configuration center.

**Sections:** General, Storage, Email, Integrations, AI Providers, Notification Settings

Enterprise admin console.

---

## FRAME 57 — AI PROVIDER CONFIGURATION

Design an AI provider configuration page.

**Providers:** OpenAI, Azure OpenAI, Ollama

**Show:** Status, Latency, Usage, Costs

Modern AI operations interface.

---

## FRAME 58 — API DEVELOPER PORTAL

Design an API developer portal.

**Sections:** API Documentation, SDKs, OpenAPI, Examples, API Keys

Developer-focused experience similar to Stripe.

---

## FRAME 59 — API DOCUMENTATION

Design a modern API documentation page.

**Features:** Endpoints, Schemas, Code Samples, Authentication, Try API

Inspired by Stripe and Postman.

---

## FRAME 60 — ONBOARDING EXPERIENCE

Design a first-time onboarding experience.

**Steps:** Create Product, Create Asset, Run First Scan, Review Findings, Generate Report

Progressive setup wizard. Modern SaaS onboarding UX.

---

> **Kết luận:** Sau 60 frame này, **OSV Platform** đã bao phủ gần như toàn bộ chức năng trong PRD/URD/SRS:
>
> - Vulnerability Intelligence (10 màn hình)
> - Scanning (10 màn hình)
> - Findings Management (8 màn hình)
> - Asset Management (6 màn hình)
> - Product Security (5 màn hình)
> - AI Center (5 màn hình)
> - Reporting (5 màn hình)
> - Integrations (5 màn hình)
> - Administration (10+ màn hình)
>
> => Đây là phạm vi tương đương một sản phẩm thương mại cấp enterprise như **Tenable.io, Wiz** hoặc **DefectDojo Enterprise** ở mức thiết kế UX/UI hoàn chỉnh.