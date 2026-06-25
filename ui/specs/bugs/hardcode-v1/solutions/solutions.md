# Giải pháp xử lý Hardcoded APIs & Mock Data (v1)

Tài liệu này đề xuất giải pháp kỹ thuật dựa trên `architecture.md` (Version 1.1) để xử lý các vấn đề đã được nêu trong `report.md`. Mọi giải pháp tuân thủ tuyệt đối nguyên tắc **API-First**: Không có mock data cứng trong component, mọi data phải được fetch qua server (sử dụng MSW ở môi trường dev).

---

## Phần 1: Giải quyết Hardcoded API Endpoints

**Nguyên nhân gốc rễ**: Lập trình viên quên cập nhật file `shared/api/endpoints.ts` và gõ thẳng URL vào Component.

### Giải pháp:
1. **Cập nhật `endpoints.ts`**: Bổ sung các endpoint bị thiếu.
```typescript
// ui/src/shared/api/endpoints.ts
export const ENDPOINTS = {
  // ...
  ai: {
    insights: '/api/v1/ai/insights',
    // ...
  },
  integrations: {
    jira: '/api/v1/integrations/jira', // Cập nhật lại cho đúng với Frontend đang gọi
  },
  webhooks: {
    retryDelivery: (id: string) => `/api/v1/webhooks/deliveries/${id}/retry`,
  },
  notifications: {
    stream: '/api/v1/notifications/stream', // Dùng hàm thay vì literal nếu cần params
  }
};
```
2. **Refactor Components**: Thay thế literal strings bằng `ENDPOINTS...` kết hợp với `apiClient`.

---

## Phần 2: Giải quyết Hardcoded Mock Data trong Components

Theo `architecture.md` (Section 5.5), việc để `const mockData = [...]` trong React Component bị nghiêm cấm. 

### Quy trình giải quyết chung:
1. **Tạo Fixture**: Cắt các mảng dữ liệu giả ra và lưu vào thư mục `ui/src/mocks/fixtures/`.
2. **Setup MSW Handler**: Viết endpoint ảo trong `ui/src/mocks/handlers/` trả về dữ liệu từ Fixture.
3. **Thêm API Hook**: Định nghĩa logic gọi data bằng Axios (`apiClient`) kết hợp `React Query` (`useQuery`).
4. **Refactor Component**: Thay vì lặp qua mảng hardcode, sử dụng kết quả từ hook `const { data, isLoading } = useQuery...` kết hợp với `QueryBoundary` để handle trạng thái Loading/Error.

---

## Phần 3: API Request Specs (Backend Requirements)

Dưới đây là danh sách các API cần Backend phát triển để phục vụ cho các Mock Data hiện tại đang nằm trên Frontend. MSW sẽ implement các endpoint này trước để UI chạy mượt mà.

### 1. Reports & Templates
- **Lấy danh sách Report Templates**:
  - `GET /api/v1/reports/templates`
  - Response: `{ items: ReportTemplate[] }`
- **Lấy danh sách Reports đã tạo**:
  - `GET /api/v1/reports`
  - Response: `{ items: Report[], total: number }`

### 2. User Profile & Settings
- **Lấy danh sách Sessions (Thiết bị đăng nhập)**:
  - `GET /api/v1/profile/sessions`
  - Response: `{ items: Session[] }`
- **Lấy/Cập nhật cấu hình Notifications**:
  - `GET /api/v1/profile/notifications/settings`
  - `PUT /api/v1/profile/notifications/settings`
  - Response: `{ email: boolean, inApp: boolean, ... }`

### 3. Scanning Results (Nmap & ZAP)
- **Lấy kết quả Nmap của 1 Scan**:
  - `GET /api/v1/scans/{scanId}/results/nmap`
  - Response: `{ hosts: NmapHost[] }`
- **Lấy kết quả ZAP của 1 Scan**:
  - `GET /api/v1/scans/{scanId}/results/zap`
  - Response: `{ alerts: ZapAlert[], riskBreakdown: {...} }`
- **Lấy lịch sử Scans**:
  - `GET /api/v1/scans/history`
  - Response: `{ items: ScanHistoryRecord[], total: number }`

### 4. Risk Acceptance
- **Lấy danh sách Risk Acceptances**:
  - `GET /api/v1/risk-acceptances`
  - Response: `{ items: RiskAcceptance[], total: number }`

### 5. Webhooks & API Keys
- **Lấy lịch sử Delivery của Webhook**:
  - `GET /api/v1/webhooks/deliveries` (có hỗ trợ filter theo webhookId)
  - Response: `{ items: WebhookDelivery[], total: number }`
- **Lấy biểu đồ hoạt động Webhook (Activity Chart)**:
  - `GET /api/v1/webhooks/stats`
  - Response: `{ timeline: { date: string, count: number }[] }`
- **Lấy danh sách API Keys**:
  - `GET /api/v1/api-keys`
  - Response: `{ items: ApiKey[] }`

### 6. Admin & Audit
- **Lấy danh sách Users (Admin)**:
  - `GET /api/v1/admin/users`
  - Response: `{ items: User[], total: number }`
- **Lấy Audit Logs hệ thống**:
  - `GET /api/v1/audit-log`
  - Query: `?page=1&limit=50`
  - Response: `{ items: AuditLogEntry[], total: number }`

### 7. AI Triage & Semantic Search
- **Lấy hàng đợi AI Triage (Queue)**:
  - `GET /api/v1/ai/triage/queue`
  - Response: `{ items: TriageTask[] }`
- **Lấy lịch sử/gợi ý tìm kiếm ngữ nghĩa**:
  - `GET /api/v2/cves/search/semantic/suggestions`
  - Response: `{ suggestions: string[] }`

### 8. Global Search / Command Palette
- **Lấy dữ liệu Global Search**:
  - `GET /api/v1/search/recent`
  - `GET /api/v1/search/suggested`
  - Response: `{ items: SearchResultItem[] }`

### 9. Notifications
- **Lấy danh sách Notifications**:
  - `GET /api/v1/notifications`
  - Response: `{ items: Notification[], total: number, unreadCount: number }`

### 10. CAPEC Library
- **Lấy danh sách mẫu tấn công CAPEC**:
  - `GET /api/v2/capec`
  - Query: `?page=1&limit=50&search=...`
  - Response: `{ items: CapecPattern[], total: number }`

---
*Ghi chú cho Backend Team*: Hãy tham khảo cấu trúc DTO hiện tại trong `ui/src/shared/types/` để định nghĩa model khớp với mong đợi của Frontend.
