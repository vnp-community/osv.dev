# CR-HC-004: gateway-service — Admin Settings không đọc từ DB

## Trạng thái: 🟠 High

## Vấn đề
File: `services/gateway-service/internal/bff/handlers/handler_ui_api.go` (GetAdminSettings)

Handler trả về admin settings với nhiều giá trị static/hardcoded thay vì đọc từ database:
```go
"smtp_enabled":     false,
"smtp_host":        "",
"mfa_required":     false,
"session_timeout":  3600,
"password_policy":  "medium",
"model":            aiModel,  // đọc từ env — ok
```

Các settings này sẽ không thay đổi được từ Admin UI trừ khi được lưu vào DB.

## Giải pháp

### 1. Schema `platform_settings` trong PostgreSQL
```sql
-- apps/osv/migrations/001_platform_settings.up.sql (đã tồn tại, cần kiểm tra)
CREATE TABLE IF NOT EXISTS platform_settings (
    key         VARCHAR(255) PRIMARY KEY,
    value       JSONB NOT NULL,
    description TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by  UUID
);

-- Default values
INSERT INTO platform_settings (key, value) VALUES
    ('smtp.enabled',       'false'),
    ('smtp.host',          '""'),
    ('smtp.port',          '587'),
    ('smtp.from',          '"noreply@example.com"'),
    ('mfa.required',       'false'),
    ('session.timeout',    '3600'),
    ('password.policy',    '"medium"'),
    ('ai.model',           '"llama3"')
ON CONFLICT (key) DO NOTHING;
```

### 2. Repository interface (identity-service hoặc shared)
```go
type SystemSettingsRepository interface {
    Get(ctx context.Context, key string) (json.RawMessage, error)
    GetAll(ctx context.Context) (map[string]json.RawMessage, error)
    Set(ctx context.Context, key string, value interface{}, updatedBy uuid.UUID) error
}
```

### 3. AdminSettings usecase
```go
type GetAdminSettingsUseCase struct {
    repo SystemSettingsRepository
}

func (uc *GetAdminSettingsUseCase) Execute(ctx context.Context) (*AdminSettings, error) {
    all, err := uc.repo.GetAll(ctx)
    if err != nil {
        return nil, err
    }
    return mapToAdminSettings(all), nil
}
```

### 4. PUT /admin/settings — lưu vào DB
```go
func (h *AdminHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
    var req AdminSettingsUpdateRequest
    // decode và validate
    for k, v := range req.ToMap() {
        if err := h.settingsUC.Set(ctx, k, v, userID); err != nil {
            writeError(w, err)
            return
        }
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
```

## Files cần thay đổi
- `services/identity-service/internal/infra/postgres/system_settings_repo.go` [NEW]
- `services/identity-service/internal/usecase/admin_settings/get.go` [NEW]
- `services/identity-service/internal/usecase/admin_settings/update.go` [NEW]
- `services/identity-service/adapter/handler/http/admin_handler.go` — wire usecase
- `services/gateway-service/internal/bff/handlers/handler_ui_api.go` — proxy to identity-service
- `apps/osv/migrations/001_platform_settings.up.sql` — kiểm tra đã tồn tại chưa

## Acceptance Criteria
- [ ] `GET /admin/settings` đọc từ `platform_settings` table
- [ ] `PUT /admin/settings` lưu vào DB và persist sau restart
- [ ] Default values được seed khi migration chạy
