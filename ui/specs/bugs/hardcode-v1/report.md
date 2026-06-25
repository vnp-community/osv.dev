# Bug Report: Hardcoded APIs & Mock Data

## Vấn đề 1: Hardcoded API Endpoints
Trong quá trình rà soát lại mã nguồn Frontend (`ui/src`), phát hiện một số Component và Hook không sử dụng biến tổng hợp `ENDPOINTS` (từ `shared/api/endpoints.ts`) mà trực tiếp hardcode các string literal `/api/v1/...` vào code. Việc này vi phạm nguyên tắc "Single Source of Truth".

### Danh sách:
1. **JIRA Integration Config**
   - **File**: `ui/src/features/integrations/components/JiraConfig.tsx` 
   - **API**: `GET /api/v1/integrations/jira` và `PUT /api/v1/integrations/jira`
2. **AI Insights**
   - **File**: `ui/src/features/ai-center/components/AIInsights.tsx`
   - **API**: `GET /api/v1/ai/insights`
3. **Webhook Delivery Retry**
   - **File**: `ui/src/features/integrations/hooks/useWebhookDeliveries.ts` 
   - **API**: `POST /api/v1/webhooks/deliveries/${deliveryId}/retry`
4. **Notifications SSE Stream**
   - **File**: `ui/src/features/notifications/hooks/useNotificationsSSE.ts`
   - **API**: `GET /api/v1/notifications/stream` (Hardcode string url parse query param).

---

## Vấn đề 2: Hardcoded Mock Data trong Components
Chiếu theo nguyên tắc "Mọi mock data phải lấy từ server và trả về thông qua server API", tất cả dữ liệu giả (arrays/objects) fix cứng bên trong các file component UI đều bị coi là BUG. Dữ liệu này phải được fetch qua `apiClient` từ Mock Server (MSW).

### Danh sách các file chứa Mock Arrays vi phạm:

**Trong `app/components/` (Cấu trúc cũ/chung):**
- `ReportCenter.tsx` (Chứa: `templates`, `reports`)
- `NotificationCenter.tsx` (Chứa: `notifications`)
- `UserProfile.tsx` (Chứa: `sessions`, `notifSettings`)
- `RiskAcceptanceCenter.tsx` (Chứa: `acceptances`)
- `NmapResults.tsx` (Chứa: `hosts`)
- `ZAPResults.tsx` (Chứa: `alerts`)
- `UserManagement.tsx` (Chứa: `users`)
- `WebhookEvents.tsx` (Chứa: `DELIVERY_HISTORY`, `ACTIVITY_CHART`)
- `ScanHistory.tsx` (Chứa: `history`)
- `GlobalSearch.tsx` (Chứa: `RECENT`, `SUGGESTED`, `ALL_RESULTS`)
- `APIKeyManagement.tsx` (Chứa: `apiKeys`)
- `AuditLogs.tsx` (Chứa: `auditLogs`)
- `AITriage.tsx` (Chứa: `queue`)

**Trong `features/*/components/` (Cấu trúc mới theo domains):**
- `auth/components/UserProfile.tsx` (Chứa: `sessions`, `notifSettings`)
- `cve-intel/components/CAPECLibrary.tsx` (Chứa: `CAPEC_PATTERNS`)
- `cve-intel/components/SemanticSearch.tsx` (Chứa: `SUGGESTIONS`)
- `scanning/components/NmapResults.tsx` (Chứa: `hosts`)
- `scanning/components/ZAPResults.tsx` (Chứa: `alerts`)
- `shared/components/global/CommandPalette.tsx` (Chứa: `RECENT`, `SUGGESTED`, `ALL_RESULTS`)

### Giải pháp Yêu cầu (Action Items)
1. Cắt bỏ toàn bộ mảng dữ liệu (mock data) ra khỏi các Components trên.
2. Di chuyển dữ liệu vào thư mục `ui/src/mocks/handlers/` để mock response từ phía server.
3. Thay thế component bằng `useQuery` và `apiClient.get(...)` để lấy dữ liệu.
