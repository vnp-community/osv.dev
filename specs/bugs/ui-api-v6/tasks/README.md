# Tasks — UI API v6 Bug Fixes

## Danh sách Tasks theo Priority

| Task ID | File | Priority | Bug IDs | Mô tả | Status |
|---------|------|----------|---------|-------|--------|
| TASK-V6-001 | [TASK-V6-001](TASK-V6-001-fix-profile-sessions-notif-settings-500.md) | 🔴 P0 | BUG-V6-018,019,020 | Fix 500 — Profile sessions & notification settings | ✅ DONE |
| TASK-V6-002 | [TASK-V6-002](TASK-V6-002-fix-admin-user-invite-500.md) | 🔴 P0 | BUG-V6-021 | Fix 500 — Admin user invite | ✅ DONE |
| TASK-V6-003 | [TASK-V6-003](TASK-V6-003-fix-405-method-not-allowed.md) | 🟠 P1 | BUG-V6-014,015,016,017 | Fix 405 — 4 endpoints wrong method | ✅ DONE |
| TASK-V6-004 | [TASK-V6-004](TASK-V6-004-fix-create-response-missing-id.md) | 🟠 P1 | BUG-V6-022,023 | Fix response schema — Scan & Webhook create missing `id` | ✅ DONE |
| TASK-V6-005 | [TASK-V6-005](TASK-V6-005-implement-missing-routes.md) | 🟡 P2 | BUG-V6-001→013 | Implement 13 missing routes (404) | ✅ DONE |
| TASK-V6-006 | [TASK-V6-006](TASK-V6-006-fix-oauth-configuration.md) | 🔵 P3 | BUG-V6-024,025,026 | Fix OAuth configuration | ✅ DONE (cần OAuth credentials) |
| TASK-V6-007 | [TASK-V6-007](TASK-V6-007-planned-features-scan-import-report-download.md) | ⚪ P4 | BUG-V6-027,028 | Document planned features (501/503) | ✅ DONE (cần MinIO config) |

## Trạng thái tổng hợp

```
Tổng bugs: 28 (BUG-V6-001 → BUG-V6-028)
Tổng tasks: 7 (TASK-V6-001 → TASK-V6-007)
```

| Hạng mục | Số lượng | Status |
|---------|---------|--------|
| Code changes hoàn tất | 24/28 bugs | ✅ Build verified |
| Migration files tạo mới | 1 | `007_notification_preferences.sql` |
| Routes thêm mới vào gateway | 3 | ai/insights, integrations/jira (GET+PUT) |
| Cần cấu hình server | 4 bugs | OAuth credentials, MinIO |
| Cần deploy migration | 1 | `007_notification_preferences.sql` |

## Build Status

```
✅ apps/osv/              — go build ./... PASS
✅ services/identity-service — go build ./... PASS
✅ services/finding-service  — go build ./... PASS
✅ services/scan-service     — go build ./... PASS
✅ services/notification-service — go build ./... PASS
✅ services/sla-service      — go build ./... PASS
```

## Các bước còn lại trước khi close

### Deploy (DevOps)
- [ ] Chạy migration: `psql $DATABASE_URL -f 007_notification_preferences.sql` (identity-service DB)
- [ ] Deploy tất cả services lên server sau khi có code mới
- [ ] Cấu hình OAuth credentials (GOOGLE_CLIENT_ID/SECRET, GITHUB_CLIENT_ID/SECRET)
- [ ] Cấu hình MinIO cho report download

### Verify sau deploy
- [ ] Chạy lại `python3 test_all_endpoints.py` và kiểm tra không còn lỗi trong BUG-V6-001→028
- [ ] Các bug OAuth/MinIO (V6-024→028) skip nếu chưa cấu hình (`SKIP_OAUTH=true`, `SKIP_STORAGE=true`)

## Legend

- ✅ DONE — Code implemented và build pass
- 🔴 P0 — Critical, cần fix ngay
- 🟠 P1 — High priority, fix trong sprint
- 🟡 P2 — Medium priority
- 🔵 P3 — Cần cấu hình (không phải bug code)
- ⚪ P4 — Planned feature / infra setup
