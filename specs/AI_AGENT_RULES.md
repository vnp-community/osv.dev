# OSV Platform — AI Agent Rules

## Phạm vi áp dụng

File này định nghĩa các quy tắc bắt buộc cho AI agent khi làm việc trong repository `/Users/binhnt/Lab/sec/cve/osv.dev`.

---

## MANDATORY RULES

### RULE-OSV-001: Zero Mock / Zero Hardcode (Severity: CRITICAL)

Mọi code trong `services/*/internal/` **bắt buộc**:
- UseCase KHÔNG được `return nil, nil` hoặc `return errors.New("not implemented")`
- Handler KHÔNG được return hardcode strings, dates, enum values
- Repository KHÔNG được để method rỗng hoặc unimplemented
- Router KHÔNG được đăng ký `nil` handler (dùng `notImplemented("feature")`)
- Config KHÔNG được hardcode — phải đọc từ environment variables

**Reference:** `specs/01-architecture.md § 11. Enterprise Architecture Standards`

---

### RULE-OSV-002: Data Must Be Persisted (Severity: HIGH)

Mọi data state thay đổi **phải được lưu vào database phù hợp**:
- Transactional data → PostgreSQL (via Repository pattern)
- Embedding vectors → PostgreSQL pgvector
- Cache/session → Redis
- Events → NATS JetStream
- Binary files → MinIO

Không được trả response mà không persist (trừ GET operations).

**Reference:** `specs/01-architecture.md § 11.1 RULE-002`

---

### RULE-OSV-003: Clean Architecture Layer Contract (Severity: HIGH)

```
Handler → UseCase → Repository Interface ← Repository Implementation
```

- Handler KHÔNG gọi repository trực tiếp
- UseCase KHÔNG import infra packages trực tiếp
- Domain KHÔNG import từ infra/delivery/usecase
- Infrastructure implements domain interfaces

**Reference:** `specs/02-technical-design.md § 2. Clean Architecture Design`

---

### RULE-OSV-004: Implementation Order (Severity: HIGH)

Khi implement feature mới, phải theo đúng thứ tự:
1. Domain entity + interface
2. SQL Migration (IF NOT EXISTS)
3. Repository implementation
4. UseCase implementation
5. Handler implementation
6. Router registration
7. Wire trong embedded.go
8. Integration test verification

**KHÔNG được** bỏ qua bước nào, đặc biệt là migration.

---

### RULE-OSV-005: Test Verification Required (Severity: HIGH)

Sau mỗi implementation hoặc code change:
```bash
cd tests/client && python3 run_all.py --no-stop-on-fail
```
Pass rate phải ≥ 80%. Nếu fail → phải fix, không skip.

---

### RULE-OSV-006: Error Handling (Severity: HIGH)

- KHÔNG suppress errors: `_, _ = repo.Create(...)` là forbidden
- PHẢI wrap errors: `fmt.Errorf("service.Method: %w", err)`
- Handler phải map đúng domain errors → HTTP status codes
- KHÔNG log stack trace ra client response

---

### RULE-OSV-007: Skill Reference (Severity: INFO)

Trước khi implement bất kỳ service nào, đọc:
- `knowledge/osv-platform-standards/artifacts/clean_architecture_skill.md`
- `knowledge/osv-platform-standards/artifacts/enforcement_rules.md`
- `knowledge/osv-platform-standards/artifacts/production_grade_standards.md`
- `specs/01-architecture.md § 11`
- `specs/02-technical-design.md § 11`
