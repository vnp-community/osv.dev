# T01 — identity-service ✅ DONE

**Phase**: 1
**Depends on**: T00
**Status**: ✅ Completed — 2026-06-12
**Spec**: [01_identity-service.md](../../../services/01_identity-service.md)
**Estimated effort**: 2-3 hours

---

## Mục tiêu

Tạo `services/identity-service/` bằng cách **rename** `auth-service` và bổ sung code từ `archive/identity`, `archive/admin`.

---

## Nguồn merge

| Nguồn | Path | Vai trò |
|-------|------|---------|
| **BASE** | `services/auth-service/` | Code chính, giữ nguyên |
| ARCHIVE | `archive/identity/internal/` | Bổ sung domain entities nếu có phần chưa có trong auth-service |
| ARCHIVE | `archive/admin/internal/` | Bổ sung admin handlers |

---

## Tác vụ chi tiết

### Bước 1: Copy auth-service làm base

```bash
SVC_ROOT="/Users/binhnt/Lab/sec/cve/osv.dev/services"

# Copy toàn bộ auth-service vào identity-service
cp -r "$SVC_ROOT/auth-service/." "$SVC_ROOT/identity-service/"

# Xoá thư mục skeleton rỗng đã tạo ở T00 (vì đã copy)
echo "Copied auth-service → identity-service"
```

### Bước 2: Đổi tên Go module

```bash
SVC="$SVC_ROOT/identity-service"

# Sửa go.mod: đổi module name
sed -i '' 's|module github.com/osv/auth-service|module github.com/osv/identity-service|g' "$SVC/go.mod"

# Sửa tất cả import paths trong Go files
find "$SVC" -name "*.go" -exec sed -i '' \
  's|github.com/osv/auth-service|github.com/osv/identity-service|g' {} \;

echo "Module renamed to github.com/osv/identity-service"
```

### Bước 3: Tổ chức lại cấu trúc domain theo spec

Đảm bảo cấu trúc `internal/domain/` theo spec:
```
internal/domain/
├── user/           (từ auth-service entity/)
├── token/          (từ auth-service valueobject/)
├── role/           (từ auth-service identity/ nếu có)
├── session/        (NEW — tạo mới)
└── errors/         (từ auth-service error/)
```

**Hành động cụ thể**:
```bash
cd "$SVC/internal/domain"

# Rename entity/ → user/ nếu cần
[ -d "entity" ] && mv entity user || echo "entity dir not found, check structure"

# Rename valueobject/ → token/ nếu phù hợp
# (kiểm tra content trước)
```

> **NOTE**: Kiểm tra từng file trong `internal/domain/entity/` và `internal/domain/valueobject/`. Nếu entity/ chứa User entity → rename thành `user/`. Nếu valueobject/ chứa token types → rename thành `token/`.

### Bước 4: Bổ sung từ archive/identity

```bash
ARCHIVE="$SVC_ROOT/../archive"

# Kiểm tra archive/identity có gì thêm
ls "$ARCHIVE/identity/internal/domain/"
# So sánh với identity-service/internal/domain/
# Nếu archive có usecase/role hoặc usecase/permission chưa có → copy sang
```

Các file cần copy từ `archive/identity`:
- `internal/domain/entity/` — nếu có thêm entities (Org, Tenant) chưa có trong auth-service
- `internal/usecase/` — usecase nào không có trong auth-service

### Bước 5: Bổ sung admin handlers từ archive/admin

```bash
# Kiểm tra archive/admin có gì
ls "$ARCHIVE/admin/internal/"

# Copy admin handlers nếu chưa có trong auth-service
# Thường là: list_users, suspend_user, assign_role
cp -r "$ARCHIVE/admin/internal/." "$SVC/internal/" 2>/dev/null || echo "Check structure manually"
```

### Bước 6: Tạo/cập nhật cmd/server/main.go

Đảm bảo `main.go` khởi động đúng với module name mới:
```go
// cmd/server/main.go
package main

// Import paths phải dùng: github.com/osv/identity-service/...
```

### Bước 7: Build check

```bash
cd "$SVC_ROOT/identity-service"
go mod tidy
go build ./...
```

### Bước 8: Xoá service cũ

```bash
# Chỉ xoá sau khi build pass
rm -rf "$SVC_ROOT/auth-service"
echo "Removed auth-service"
```

---

## Điều kiện hoàn thành

- [x] `services/identity-service/` tồn tại với đầy đủ code
- [x] `go.mod` có module name `github.com/osv/identity-service`
- [x] `go build ./...` pass không có lỗi
- [x] `go vet ./...` pass
- [x] Cấu trúc domain: `entity/` (user, session, api_key), `identity/` (role), `valueobject/`, `repository/`, `error/`, `event/`
- [x] `services/auth-service/` đã được xóa
- [x] Tất cả usecases từ auth-service vẫn còn: `login`, `logout`, `register`, `oauth`, `refresh_token`, `validate_token`, `manage_api_key`

---

## Files cần tạo mới (nếu không có trong auth-service)

| File | Nội dung |
|------|---------|
| `internal/domain/session/entity.go` | Session entity với ID, UserID, Token, ExpiresAt |
| `internal/domain/role/entity.go` | Role và Permission entities |
| `internal/delivery/http/admin_handler.go` | Admin user management handlers |

---

## Commit message

```
feat(identity-service): merge auth-service + admin into identity-service

- Renamed module to github.com/osv/identity-service
- Migrated all usecases: login, logout, register, oauth, refresh, validate, api-key
- Added admin handlers from archive/admin
- Organized domain: user/, token/, role/, session/, errors/
```
