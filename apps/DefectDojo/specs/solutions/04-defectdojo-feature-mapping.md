# DefectDojo Feature → Go Service Mapping

## Mapping Đầy Đủ

### 1. Product Management

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| Tạo Product | `dojo/product/views.py` | product-service | `CreateProduct` | gRPC → REST |
| Sửa Product | `dojo/product/views.py` | product-service | `UpdateProduct` | gRPC → REST |
| Xóa Product | `dojo/product/views.py` | product-service | `DeleteProduct` | gRPC → REST |
| List Products | `dojo/product/views.py` | product-service | `ListProducts` | gRPC → REST |
| Product Metrics | `dojo/metrics/` | report-service | `GetProductMetrics` | gRPC → REST |
| Product Tags | `dojo/tag_utils.py` | product-service | `UpdateTags` | gRPC → REST |
| Business Criticality | `dojo/models.py` | product-service | field in Product | - |
| Product Members | `dojo/authorization/` | auth-service + product-service | `AddMember` | gRPC → REST |
| Product Groups | `dojo/group/` | auth-service | `CreateGroup` | gRPC → REST |
| GitHub Integration | `dojo/github/` | integration-service | `LinkGitHub` | NATS |
| JIRA Integration | `dojo/jira/` | integration-service | `LinkJIRA` | NATS |
| SLA Configuration | `dojo/sla_config/` | finding-service | `CreateSLAConfig` | gRPC → REST |
| Tool Configuration | `dojo/tool_config/` | integration-service | `CreateToolConfig` | gRPC → REST |

### 2. Engagement Management

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| Tạo Engagement | `dojo/engagement/` | product-service | `CreateEngagement` | gRPC → REST |
| Close Engagement | `dojo/engagement/` | product-service | `CloseEngagement` | gRPC → REST |
| Engagement Notes | `dojo/notes/` | product-service | `AddNote` | gRPC → REST |
| Engagement Risk | `dojo/risk_acceptance/` | finding-service | `CreateRiskAcceptance` | gRPC → REST |
| CI/CD Engagement | `dojo/engagement/` | product-service + scan-service | `CreateCICDEngagement` | gRPC |
| Engagement Findings | `dojo/finding/` | finding-service | `ListFindings(engagement_id)` | gRPC → REST |

### 3. Test & Scan Management

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| Tạo Test | `dojo/test/` | scan-service | `CreateTest` | gRPC → REST |
| Import Scan Results | `dojo/importers/` | scan-service → ingestion-service | `ImportScan` | gRPC + NATS |
| Re-import Scan | `dojo/importers/` | scan-service → ingestion-service | `ReimportScan` | gRPC + NATS |
| Scan Parser (SAST) | `dojo/tools/` | ingestion-service | Built-in parsers | internal |
| Scan Parser (DAST) | `dojo/tools/` | ingestion-service | Built-in parsers | internal |
| Scan Parser (SCA) | `dojo/tools/` | ingestion-service | Built-in parsers | internal |
| Scan Parser (Container) | `dojo/tools/` | ingestion-service | Built-in parsers | internal |
| SBOM Import | `dojo/scans/` | scan-service (sbom pkg) | `ImportSBOM` | gRPC |
| Scheduled Scan | `dojo/tasks.py` | scan-service (scheduler) | `ScheduleScan` | NATS |
| Scan Agent | `dojo/scans/` | scan-service | `RegisterAgent` | gRPC |

### 4. Finding Management

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| Tạo Finding | `dojo/finding/views.py` | finding-service | `CreateFinding` | gRPC → REST |
| Sửa Finding | `dojo/finding/views.py` | finding-service | `UpdateFinding` | gRPC → REST |
| Xóa Finding | `dojo/finding/views.py` | finding-service | `DeleteFinding` | gRPC → REST |
| List Findings | `dojo/finding/views.py` | finding-service | `ListFindings` | gRPC → REST |
| Finding Detail | `dojo/finding/views.py` | finding-service | `GetFinding` | gRPC → REST |
| Bulk Update | `dojo/finding/views.py` | finding-service | `BulkUpdateFindings` | gRPC → REST |
| Close Finding | `dojo/finding/` | finding-service | `CloseFinding` | gRPC → REST |
| Mark Mitigated | `dojo/finding/` | finding-service | `MitigateFinding` | gRPC → REST |
| Mark False Positive | `dojo/finding/` | finding-service | `SetFalsePositive` | gRPC → REST |
| Risk Acceptance | `dojo/risk_acceptance/` | finding-service | `AcceptRisk` | gRPC → REST |
| Deduplication | `dojo/utils.py` | finding-service | `FindByHashCode` | gRPC (internal) |
| Finding Notes | `dojo/notes/` | finding-service | `AddNote` | gRPC → REST |
| Finding Tags | `dojo/tag_utils.py` | finding-service | `ApplyTags` | gRPC |
| Finding Endpoints | `dojo/endpoint/` | finding-service | `AddEndpoint` | gRPC → REST |
| CVSS Score | `dojo/models.py` | finding-service | field in Finding | - |
| CWE/CVE Link | `dojo/models.py` | finding-service + vuln-service | field + lookup | gRPC |
| SLA Status | `dojo/sla_config/` | finding-service (SLA checker) | `GetSLAStatus` | gRPC → REST |
| Component Name/Version | `dojo/models.py` | finding-service | field in Finding | - |
| Finding Groups | `dojo/finding_group/` | finding-service | `CreateFindingGroup` | gRPC → REST |

### 5. SLA Management

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| SLA Policy | `dojo/sla_config/` | finding-service | `CreateSLAConfig` | gRPC → REST |
| SLA Calculation | `dojo/tasks.py` | finding-service (SLA checker) | Ticker-based | internal |
| SLA Expiry Alert | `dojo/notifications/` | finding-service → notification-service | `SLABreach` | NATS |
| SLA Dashboard | `dojo/metrics/` | report-service | `GetSLAMetrics` | gRPC → REST |
| SLA Override | `dojo/finding/` | finding-service | `ExtendSLA` | gRPC → REST |

### 6. Notification & Alerting

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| System Notifications | `dojo/notifications/` | notification-service | `CreateAlert` | NATS |
| Email Notifications | `dojo/notifications/` | notification-service | `SendEmail` | NATS |
| Slack Integration | `dojo/notifications/` | notification-service | `SendSlack` | NATS |
| MS Teams | `dojo/notifications/` | notification-service | `SendTeams` | NATS |
| Webhook | `dojo/notifications/` | notification-service | `TriggerWebhook` | NATS |
| Alert Rules | `dojo/notifications/` | notification-service | `CreateRule` | gRPC → REST |
| Alert Subscriptions | `dojo/notifications/` | notification-service | `Subscribe` | gRPC → REST |
| User Alerts | `dojo/notifications/` | notification-service | `ListAlerts` | gRPC → REST |

### 7. Reporting

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| Executive Report | `dojo/reports/` | report-service | `GenerateExecutiveReport` | gRPC → REST |
| Findings Report | `dojo/reports/` | report-service | `GenerateFindingsReport` | gRPC → REST |
| Engagement Report | `dojo/reports/` | report-service | `GenerateEngagementReport` | gRPC → REST |
| Product Report | `dojo/reports/` | report-service | `GenerateProductReport` | gRPC → REST |
| PDF Export | `dojo/reports/` | report-service (PDF formatter) | `ExportPDF` | gRPC → REST |
| CSV Export | `dojo/reports/` | report-service (CSV formatter) | `ExportCSV` | gRPC → REST |
| JSON Export | `dojo/reports/` | report-service (JSON formatter) | `ExportJSON` | gRPC → REST |
| Metrics Dashboard | `dojo/metrics/` | report-service | `GetMetrics` | gRPC → REST |
| Finding Trends | `dojo/metrics/` | report-service | `GetTrends` | gRPC → REST |
| SBOM/VEX Report | `dojo/reports/` | report-service + scan-service | `GenerateSBOM` | gRPC |

### 8. Authentication & Authorization

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| Login (JWT) | `dojo/user/` | auth-service | `Login` | REST |
| Login (API Key) | `dojo/user/` | auth-service | `ValidateAPIKey` | gRPC |
| SSO (OAuth2) | `dojo/sso/` | auth-service | `OAuthCallback` | REST |
| TOTP/2FA | `dojo/user/` | auth-service | `EnableTOTP`, `ValidateTOTP` | REST |
| Token Refresh | `dojo/user/` | auth-service | `RefreshToken` | REST |
| Logout | `dojo/user/` | auth-service | `Logout` | REST |
| Register User | `dojo/user/` | auth-service | `CreateUser` | gRPC → REST |
| User Roles (RBAC) | `dojo/authorization/` | auth-service | `AssignRole` | gRPC → REST |
| Role-based Permissions | `dojo/authorization/` | auth-service | `CheckPermission` | gRPC (middleware) |
| API Key Management | `dojo/user/` | auth-service | `CreateAPIKey` | gRPC → REST |

### 9. Integration Features

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| JIRA Push Finding | `dojo/jira/` | integration-service | `PushToJIRA` | NATS |
| JIRA Sync | `dojo/jira/` | integration-service | `SyncJIRA` | NATS |
| JIRA Close Issue | `dojo/jira/` | integration-service | `CloseJIRAIssue` | NATS |
| GitHub Issues | `dojo/github/` | integration-service | `CreateGitHubIssue` | NATS |
| GitHub Close Issue | `dojo/github/` | integration-service | `CloseGitHubIssue` | NATS |
| Tool Product Config | `dojo/tool_product/` | integration-service | `LinkToolProduct` | gRPC → REST |
| Azure DevOps | `dojo/notifications/` | integration-service | `CreateADOWorkItem` | NATS |

### 10. Search & Query

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| Global Search | `dojo/search/` | search-service | `Search` | gRPC → REST |
| Finding Search | `dojo/filters.py` | finding-service + search-service | `SearchFindings` | gRPC → REST |
| CVE Search | `dojo/search/` | vulnerability-service | `SearchCVE` | gRPC → REST |
| Full-text Search | `dojo/search/` | search-service (OpenSearch) | `FullTextSearch` | gRPC |
| Vulnerability Lookup | `dojo/vuln_id/` | vulnerability-service | `LookupVuln` | gRPC → REST |

### 11. AI & Analytics

| DefectDojo Feature | Go Service | Method | Protocol |
|---|---|---|---|
| AI Finding Triage | ai-service | `TriageFinding` | NATS |
| AI Priority Score | ai-service | `ScorePriority` | NATS |
| AI Remediation Suggestion | ai-service | `SuggestRemediation` | NATS |
| CVSS Impact Assessment | impact-service | `AssessImpact` | NATS |
| Business Risk Score | impact-service | `CalculateRisk` | NATS |

### 12. Administration

| DefectDojo Feature | Django Module | Go Service | Method | Protocol |
|---|---|---|---|---|
| System Settings | `dojo/system_settings/` | unified-gateway + auth-service | `GetSystemSettings` | REST |
| User Management | `dojo/user/` | auth-service | `AdminListUsers` | gRPC → REST |
| Announcement | `dojo/announcement/` | notification-service | `CreateAnnouncement` | gRPC → REST |
| Development Env | `dojo/development_environment/` | product-service | `ListEnvironments` | gRPC → REST |
| Regulation | `dojo/regulations/` | product-service | `ListRegulations` | gRPC → REST |
| Note Types | `dojo/note_type/` | product-service | `ListNoteTypes` | gRPC → REST |
| Benchmark | `dojo/benchmark/` | report-service | `RunBenchmark` | gRPC → REST |

## Scan Parser Support Matrix

Ingestion-service supports các parser sau (tái sử dụng từ `scan-service/internal/parsers`):

| Scanner | Format | Parser |
|---|---|---|
| Trivy | JSON | `TrivyParser` |
| Grype | JSON | `GrypeParser` |
| Semgrep | JSON | `SemgrepParser` |
| Snyk | JSON | `SnykParser` |
| OWASP ZAP | XML/JSON | `ZAPParser` |
| Burp Suite | XML | `BurpParser` |
| Checkmarx | XML/JSON | `CheckmarxParser` |
| Sonatype | JSON | `SonatypeParser` |
| CycloneDX | JSON/XML | `CycloneDXParser` |
| SPDX | JSON | `SPDXParser` |
| Dependency-Track | JSON | `DependencyTrackParser` |
| GitLab SAST | JSON | `GitLabSASTParser` |
| GitLab Dependency | JSON | `GitLabDependencyParser` |
| GitHub Advanced Security | SARIF | `SARIFParser` |
| SARIF (generic) | JSON | `SARIFParser` |
| Nuclei | JSON | `NucleiParser` |
| Nessus | XML | `NessusParser` |
| OpenVAS | XML | `OpenVASParser` |
| AWS Inspector | JSON | `AWSInspectorParser` |
| Anchore | JSON | `AnchoreParser` |
