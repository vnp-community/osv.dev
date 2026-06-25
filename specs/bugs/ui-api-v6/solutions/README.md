# Solutions — UI API v6

Giải pháp chi tiết cho từng nhóm bugs từ kết quả test `test_all_endpoints.py` và `run_all.py`.

## Danh sách Solutions

| Solution | File | Bugs | Mô tả |
|----------|------|------|-------|
| SOL-V6-001 | [SOL-V6-001](SOL-V6-001-fix-profile-500.md) | BUG-V6-018,019,020 | DB migration + handlers cho profile sessions & notification settings |
| SOL-V6-002 | [SOL-V6-002](SOL-V6-002-fix-admin-invite-500.md) | BUG-V6-021 | Decouple email từ invite creation, tạo bảng `user_invitations` |
| SOL-V6-003 | [SOL-V6-003](SOL-V6-003-fix-405-method-not-allowed.md) | BUG-V6-014,015,016,017 | Router fixes: thêm GET/PUT handlers, reorder routes |
| SOL-V6-004 | [SOL-V6-004](SOL-V6-004-fix-create-response-schema.md) | BUG-V6-022,023 | Fix response serialization để include `id` field |
| SOL-V6-005 | [SOL-V6-005](SOL-V6-005-implement-missing-routes.md) | BUG-V6-001→013 | Gateway route registration + service handler implementations |
| SOL-V6-006 | [SOL-V6-006](SOL-V6-006-fix-oauth-and-features.md) | BUG-V6-024→028 | OAuth config, /auth/callback middleware fix, MinIO setup |

## Thứ tự thực thi khuyến nghị

1. **SOL-V6-003** (405 fixes) — Router only, low risk, quick win
2. **SOL-V6-004** (schema fix) — Handler only, low risk
3. **SOL-V6-001** (profile 500) — DB migration required
4. **SOL-V6-002** (admin invite) — DB migration required
5. **SOL-V6-006 Part A** (OAuth /callback middleware) — Config only
6. **SOL-V6-005** (missing routes) — Gateway + service work
7. **SOL-V6-006 Part B,C** (scan import + MinIO) — Feature implementation
