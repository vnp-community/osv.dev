# Frontend API Endpoints

This document lists all API endpoints (paths and methods) that the frontend application calls to the backend. It is based on the definitions in `ui/src/shared/api/endpoints.ts` and their corresponding usages across the codebase.

## Authentication & Authorization
| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/auth/login` | User login |
| `POST` | `/api/v1/auth/refresh` | Refresh access token |
| `POST` | `/api/v1/auth/logout` | User logout |
| `GET`  | `/api/v1/auth/me` | Get current user profile |
| `GET` | `/api/v1/auth/mfa/setup` | Setup MFA |
| `POST` | `/api/v1/auth/mfa/confirm` | Confirm MFA |
| `GET`  | `/api/v1/auth/oauth/google` | OAuth with Google |
| `GET`  | `/api/v1/auth/oauth/github` | OAuth with GitHub |
| `GET`  | `/api/v1/auth/callback` | OAuth Callback |

## Dashboard & Metrics
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/dashboard` | Main dashboard metrics |
| `GET`  | `/api/v1/dashboard/sla` | SLA metrics for dashboard |

## Notifications
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/notifications/stream` | Notifications SSE Stream |
| `GET`  | `/api/v1/notifications` | List notifications |
| `GET`  | `/api/v1/notifications/unread-count` | Get unread notifications count |
| `PATCH`| `/api/v1/notifications/{id}/read` | Mark a notification as read |
| `POST` | `/api/v1/notifications/mark-all-read` | Mark all notifications as read |

## CVE Intelligence (v2)
| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v2/cves/search` | Full-text search for CVEs |
| `POST` | `/api/v2/cves/search/semantic` | Semantic search for CVEs |
| `GET`  | `/api/v2/cves/search/semantic/suggestions` | Semantic search suggestions |
| `GET`  | `/api/v2/cves/{id}` | Get CVE details |
| `GET`  | `/api/v2/cves/aggregations` | Get CVE aggregations |
| `GET`  | `/api/v2/cves/export` | Export CVE data |
| `GET`  | `/api/v2/kev` | List CISA KEVs |
| `GET`  | `/api/v2/kev/stats` | KEV statistics |
| `GET`  | `/api/v2/kev/ransomware` | KEV ransomware info |
| `GET`  | `/api/v2/epss/{cveId}` | Get EPSS score by CVE |
| `GET`  | `/api/v2/epss/top` | Top EPSS scores |
| `GET`  | `/api/v2/epss/distribution` | EPSS distribution |
| `GET`  | `/api/v2/cwe` | List CWEs |
| `GET`  | `/api/v2/cwe/{id}` | Get CWE details |
| `GET`  | `/api/v2/capec` | List CAPEC patterns |
| `GET`  | `/api/v2/capec/{id}` | Get CAPEC details |
| `GET`  | `/api/v2/vendors` | List vendors |
| `GET`  | `/api/v2/browse` | Root vendor browse |
| `GET`  | `/api/v2/browse/{vendor}` | Browse by vendor |
| `GET`  | `/api/v2/browse/{vendor}/{product}` | Browse by vendor and product |
| `GET`  | `/api/v2/dbinfo` | Database info |

## Scans (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/scans` | List scans |
| `POST` | `/api/v1/scans` | Create a scan |
| `GET`  | `/api/v1/scans/history` | Get scan history |
| `GET`  | `/api/v1/scans/{id}` | Get scan detail |
| `GET`  | `/api/v1/scans/{id}/stream` | Scan logs SSE stream |
| `POST` | `/api/v1/scans/{id}/cancel` | Cancel scan |
| `GET`  | `/api/v1/scans/{id}/results/nmap` | Get Nmap scan results |
| `GET`  | `/api/v1/scans/{id}/results/zap` | Get ZAP scan results |
| `GET`  | `/api/v1/scans/scheduled` | Get scheduled scans |
| `POST` | `/api/v1/scans/import` | Import scan results |

## Findings & Risk Acceptances (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/findings` | List findings |
| `GET`  | `/api/v1/findings/stats` | Finding statistics |
| `GET`  | `/api/v1/findings/{id}` | Get finding details |
| `PATCH`| `/api/v1/findings/{id}` | Update a finding |
| `GET`  | `/api/v1/findings/{id}/notes` | Get finding notes |
| `POST` | `/api/v1/findings/{id}/notes` | Add a finding note |
| `GET`  | `/api/v1/findings/{id}/audit` | Get finding audit trail |
| `POST` | `/api/v1/findings/bulk/close` | Bulk close findings |
| `POST` | `/api/v1/findings/bulk/reopen`| Bulk reopen findings |
| `POST` | `/api/v1/findings/bulk/assign`| Bulk assign findings |
| `GET`  | `/api/v1/risk-acceptances` | List risk acceptances |
| `POST` | `/api/v1/risk-acceptances` | Create a risk acceptance |
| `DELETE`| `/api/v1/risk-acceptances/{id}` | Delete a risk acceptance |

## SLA Configuration (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/sla/config` | Get SLA configuration |
| `PUT`  | `/api/v1/sla/config` | Update SLA configuration |
| `GET`  | `/api/v1/sla/overview` | SLA overview report |

## Assets (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/assets` | List assets |
| `GET`  | `/api/v1/assets/{id}` | Get asset details |
| `GET`  | `/api/v1/assets/{id}/findings`| Get asset findings |
| `PATCH`| `/api/v1/assets/{id}` | Update asset details |
| `GET`  | `/api/v1/assets/tags` | List asset tags |

## Products & Engagements (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/products` | List products |
| `POST` | `/api/v1/products` | Create a product |
| `GET`  | `/api/v1/products/{id}` | Get product details |
| `PATCH`| `/api/v1/products/{id}` | Update a product |
| `GET`  | `/api/v1/products/{id}/engagements` | Get engagements for a product |
| `GET`  | `/api/v1/products/grades` | Product grades |
| `GET`  | `/api/v1/products/types` | Product types |
| `GET`  | `/api/v1/engagements/{engId}/tests` | Get tests for an engagement |

## AI (v1)
| Method | Path | Description |
|---|---|---|
| `POST`  | `/api/v1/ai/triage/{findingId}` | AI triage a finding |
| `POST` | `/api/v1/ai/triage/{findingId}/review`| Review AI triage |
| `GET`  | `/api/v1/ai/triage/queue` | AI triage queue |
| `GET`  | `/api/v1/ai/enrichment` | AI enrichment data |
| `POST` | `/api/v1/ai/enrichment/trigger`| Trigger AI enrichment |
| `GET`  | `/api/v1/ai/enrichment/{cveId}`| Enrich specific CVE |
| `GET`  | `/api/v1/ai/insights` | Get AI insights |

## Reports (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/reports` | List reports |
| `GET`  | `/api/v1/reports/templates` | List report templates |
| `POST` | `/api/v1/reports` | Create a report |
| `GET`  | `/api/v1/reports/{id}` | Get report details |
| `GET`  | `/api/v1/reports/{id}/download`| Download report |
| `DELETE`| `/api/v1/reports/{id}` | Delete a report |

## Integrations & Webhooks (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/webhooks` | List webhooks |
| `GET`  | `/api/v1/webhooks/deliveries` | List webhook deliveries |
| `GET`  | `/api/v1/webhooks/stats` | Get webhook delivery stats |
| `POST` | `/api/v1/webhooks` | Create a webhook |
| `DELETE`| `/api/v1/webhooks/{id}` | Delete a webhook |
| `POST` | `/api/v1/webhooks/{id}/test` | Test a webhook |
| `POST` | `/api/v1/webhooks/deliveries/{deliveryId}/retry` | Retry webhook delivery |
| `GET`  | `/api/v1/api-keys` | List API Keys |
| `POST` | `/api/v1/api-keys` | Create API Key |
| `DELETE`| `/api/v1/api-keys/{id}` | Revoke API Key |
| `GET`  | `/api/v1/jira/config` | JIRA integration config |
| `POST` | `/api/v1/jira/config/test` | Test JIRA config |
| `GET`  | `/api/v1/integrations/jira` | JIRA integration config (hardcoded) |
| `PUT`  | `/api/v1/integrations/jira` | Update JIRA integration config |

## Profile & Admin (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/profile` | Get user profile |
| `PATCH`| `/api/v1/profile` | Update user profile |
| `POST` | `/api/v1/profile/change-password` | Change password |
| `GET`  | `/api/v1/profile/sessions` | List user sessions |
| `GET`  | `/api/v1/profile/notifications/settings` | Get notification settings |
| `PUT`  | `/api/v1/profile/notifications/settings` | Update notification settings |
| `GET`  | `/api/v1/admin/users` | List users |
| `GET`  | `/api/v1/admin/users/{id}` | Get user details |
| `POST` | `/api/v1/admin/users/invite` | Invite user |
| `POST` | `/api/v1/admin/users/{id}/unlock`| Unlock user |
| `POST` | `/api/v1/admin/users/{id}/reset-password` | Reset user password |
| `GET`  | `/api/v1/admin/roles` | List roles |
| `GET`  | `/api/v1/admin/health` | System health status |
| `GET`  | `/api/v1/admin/settings` | Admin settings |

## Audit (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/audit-log` | Get system audit log |

## Global Search (v1)
| Method | Path | Description |
|---|---|---|
| `GET`  | `/api/v1/search/recent` | Recent searches |
| `GET`  | `/api/v1/search/suggested` | Suggested searches |
