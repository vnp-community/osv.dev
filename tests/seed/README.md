# OSV Seed Scripts

Scripts Python để generate, push và verify seed data cho toàn bộ backend services của hệ thống OSV.

## Cấu trúc thư mục

```
tests/seed/
├── .env.example              # Template cấu hình — copy thành .env và điền giá trị thực
├── requirements.txt          # Python dependencies
├── seed_config.py            # Module đọc config từ .env (shared)
├── seed_client.py            # HTTP client với auth và retry (shared)
│
├── 01_generate_seed_data.py  # Script 1: Generate dữ liệu theo models
├── 02_push_seed_data.py      # Script 2: Push dữ liệu lên server qua API
├── 03_verify_seed_data.py    # Script 3: Lấy dữ liệu từ server và đối chiếu
│
└── data/                     # Thư mục dữ liệu (tạo bởi Script 1)
    ├── identity/
    │   ├── users.json
    │   ├── api_keys.json
    │   ├── platform_settings.json        # TASK-HC-009: 15 keys → PUT /admin/settings
    │   ├── rbac_roles.json               # TASK-HC-010: 4 system roles
    │   ├── rbac_permission_categories.json # TASK-HC-010: 7 categories
    │   └── user_invitations.json         # TASK-HC-014: 4 records (3 pending + 1 accepted)
    ├── products/
    │   ├── product_types.json
    │   ├── products.json
    │   ├── engagements.json
    │   └── tests.json
    ├── findings/
    │   ├── findings.json
    │   ├── finding_notes.json
    │   └── finding_groups.json
    ├── sla/
    │   └── sla_configurations.json
    ├── cves/
    │   ├── custom_cves.json       # Internal CVE records (SEED-004)
    │   └── cve_triages.json       # Triage decisions cho known CVEs
    ├── ranking/
    │   └── ranking_entries.json   # CPE ranking per org group (SEED-004.6)
    ├── notifications/
    │   ├── notification_rules.json
    │   ├── subscriptions.json
    │   └── webhooks.json
    ├── assets/
    │   ├── assets.json
    │   └── asset_vulnerabilities.json  # Inject CVEs into assets (SEED-005.4)
    ├── agents/
    │   ├── agents.json            # Scan agents (SEED-005.5)
    │   └── agent_reports.json     # Package reports (SEED-005.6)
    ├── scans/
    │   └── scheduled_scans.json   # Cron-based scan configs (SEED-005.7, TASK-HC-011)
    ├── config/
    │   ├── sla_assignments.json          # SLA → Product assignments (SEED-006.2)
    │   ├── jira_configurations.json      # JIRA per-product configs (SEED-006.5)
    │   ├── jira_issue_mappings.json      # TASK-HC-013: Finding ↔ JIRA key mappings
    │   └── system_notification_rules.json # Global notif rules (SEED-006.7)
    ├── ai/
    │   ├── triage_queue.json      # AI triage queue entries (CR-014)
    │   └── batch_enrich_targets.json  # TASK-HC-012: 8 priority CVEs for batch enrich
    ├── search/
    │   └── search_history.json    # TASK-HC-007: 25 realistic search queries
    └── output/                    # Tạo bởi Script 2 & 3
        ├── id_map.json            # local_id → server_id mapping
        ├── push_results.json      # Kết quả của Script 2
        └── verify_report.json     # Báo cáo đối chiếu của Script 3
```

## Cài đặt

```bash
cd tests/seed
pip install -r requirements.txt
```

## Cấu hình

```bash
cp .env.example .env
# Mở .env và điền:
#   GATEWAY_URL=http://localhost:8080
#   ADMIN_EMAIL=admin@company.com
#   ADMIN_PASSWORD=your_password
```

## Workflow

### Bước 1: Generate dữ liệu

```bash
python3 01_generate_seed_data.py

# Tuỳ chỉnh số lượng bản ghi
python3 01_generate_seed_data.py --count 20

# Chỉ định output directory
python3 01_generate_seed_data.py --out ./data/custom
```

Output: Các file JSON trong `./data/` — **34 loại data** trải dài 14 domains.

### Bước 2: Push dữ liệu lên server

```bash
python3 02_push_seed_data.py

# Dry-run (không gửi request thực)
python3 02_push_seed_data.py --dry-run

# Bỏ qua một số domains
python3 02_push_seed_data.py --skip users api_keys ai_triage

# Chỉ seed một số domains
python3 02_push_seed_data.py --skip sla product_types products engagements tests sla_assignments findings notes groups cves cve_triages ranking assets asset_vulns agents agent_reports scheduled_scans notifications subscriptions webhooks jira system_rules ai_triage
```

Output:
- `./data/output/id_map.json` — mapping local_id → server_id
- `./data/output/push_results.json` — chi tiết kết quả

### Bước 3: Verify (đối chiếu với server)

```bash
python3 03_verify_seed_data.py

# Chỉ verify một số domains
python3 03_verify_seed_data.py --domain users,products,findings

# Verify AI triage schema (CR-014)
python3 03_verify_seed_data.py --domain ai_triage

# Verify agents và CVEs
python3 03_verify_seed_data.py --domain agents,custom_cves,subscriptions
```

Output:
- `./data/output/verify_report.json` — báo cáo JSON đầy đủ
- Console report: mismatches và bugs được flag rõ ràng

**Exit codes:**
- `0` — tất cả checks passed
- `1` — có mismatches hoặc resources bị thiếu trên server (khả năng bug backend)
- `2` — không thể kết nối / auth thất bại

## Thứ tự seeding (dependency order)

```
1.  Identity        → users, api_keys
2.  SLA             → sla_configurations (bulk endpoint)
3.  Products        → product_types, products, engagements, tests
4.  SLA Assign      → sla_assignments (assign SLA to products)
5.  Findings        → findings (bulk), finding_notes, finding_groups
6.  CVEs            → custom_cves, cve_triages
7.  Ranking         → ranking_entries
8.  Assets          → assets (bulk), asset_vulnerabilities
9.  Agents          → agents, agent_reports
10. Scans           → scheduled_scans (TASK-HC-011)
11. Notifications   → notification_rules (bulk), subscriptions (bulk), webhooks (bulk)
12. Config          → jira_configurations, system_notification_rules
13. AI              → ai_triage_queue
--- NEW ---
14. Settings        → platform_settings (TASK-HC-009) → PUT /admin/settings
15. RBAC            → rbac_roles (TASK-HC-010)         → GET /admin/roles + verify
16. Invitations     → user_invitations (TASK-HC-014)   → POST /admin/users/invite
17. JIRA Mappings   → jira_issue_mappings (TASK-HC-013)→ POST /api/v2/jira-issues
18. Search History  → search_history (TASK-HC-007)     → POST /api/v1/search/history
19. Batch Enrich    → batch_enrich_targets (TASK-HC-012)→ POST /ai/enrichment/batch
```

> **Dependency**: Steps 17 (jira_mappings) phải chạy AFTER step 5 (findings) và step 12 (jira configs).

## Acceptance Criteria được kiểm tra

| SEED Spec | Domain | Kiểm tra |
|-----------|--------|---------|
| SEED-001 | users, api_keys | User tồn tại, fields khớp, role đúng |
| SEED-002 | products | ProductType/Product/Engagement/Test tồn tại |
| SEED-003 | findings | Findings tồn tại, `sla_expiration_date` auto-computed |
| SEED-004 | cves, ranking | Custom CVEs tồn tại, triage decisions ghi đúng |
| SEED-005 | assets, agents | Assets/agents tồn tại, IP/hostname đúng |
| SEED-006 | sla, notifications | SLA configs đúng days, bulk webhooks/subscriptions |
| CR-011 | users | `login_attempts`, `is_locked` fields present |
| CR-014 | ai_triage | `ai_result.remarks/confidence/justification/actions`, `stats` block |
| **TASK-HC-007** | search_history | `GET /api/v1/search/recent` trả đúng queries đã seed |
| **TASK-HC-009** | platform_settings | `GET /api/v1/admin/settings` trả đủ 15 keys; value khớp |
| **TASK-HC-010** | rbac_roles | `GET /api/v1/admin/roles` trả 4 system roles + permissions |
| **TASK-HC-012** | batch_enrich | `POST /ai/enrichment/batch` trả 202 + job_id |
| **TASK-HC-013** | jira_mappings | `GET /api/v2/jira-issues` trả mappings đã seed |
| **TASK-HC-014** | invitations | `POST /admin/users/invite` tạo invitation record + token |

## Bulk Endpoints — Strategy

Script 2 luôn thử **bulk endpoint trước**, fallback về individual POST nếu endpoint chưa implement:

| Domain | Bulk endpoint | Fallback |
|--------|-------------|---------|
| SLA configs | `POST /api/v2/sla-configurations/bulk` | `POST /api/v2/sla-configurations` |
| SLA assign | `POST /api/v2/sla-configurations/assign-bulk` | `POST /api/v2/sla-configurations/{id}/assign/{product_id}` |
| Findings | `POST /api/v2/findings/bulk-create` | `POST /api/v2/findings` |
| Assets | `POST /api/v1/assets/bulk` | `POST /api/v1/assets` |
| Notif rules | `POST /api/v2/notification-rules/bulk` | `POST /api/v2/notification-rules` |
| Subscriptions | `POST /api/v2/subscriptions/bulk` | `POST /api/v2/subscriptions` |
| Webhooks | `POST /api/v2/webhooks/bulk` | `POST /api/v1/webhooks` |
| JIRA configs | `POST /api/v2/jira-configurations/bulk` | `POST /api/v2/jira-configurations` |
| Ranking | `POST /api/v1/ranking/bulk` | `POST /api/v1/ranking` |
| AI triage | `POST /api/v1/ai/triage/bulk-seed` | `POST /api/v1/ai/triage/{id}/review` |

## Chạy toàn bộ pipeline

```bash
# 1. Generate
python3 01_generate_seed_data.py --count 15

# 2. Push (dry-run trước)
python3 02_push_seed_data.py --dry-run

# 3. Push (thực tế)
python3 02_push_seed_data.py

# 4. Verify
python3 03_verify_seed_data.py
echo "Exit: $?"
```

## Lưu ý

- **Thứ tự chạy**: Script 1 → Script 2 → Script 3. Script 3 phụ thuộc vào `id_map.json` của Script 2.
- **Idempotency**: Script 2 xử lý `409 Conflict` gracefully — chạy lại sẽ skip items đã tồn tại.
- **Bulk endpoints**: Script 2 thử bulk endpoints trước, fallback về individual POST nếu endpoint chưa implement (SEED-001 đến SEED-006 đều là *proposed* endpoints).
- **AI triage**: AI triage queue thường được tạo tự động bởi ai-service sau khi scan. Bulk seed endpoint (`/api/v1/ai/triage/bulk-seed`) là internal/admin endpoint — nếu không có, chỉ review items có `human_decision` mới được seed.
- **Sensitive data**: File `.env` không được commit vào git — đã thêm vào `.gitignore`.
