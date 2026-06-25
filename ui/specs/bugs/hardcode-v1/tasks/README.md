# AI Execution Tasks: Hardcoded APIs & Mock Data

Dựa trên giải pháp tại `solutions.md`, dưới đây là danh sách các tác vụ (tasks) đã được module hóa để AI (hoặc Developer) có thể thực thi tuần tự.

Tiến trình được chia làm 3 Phase chính. Khi thực hiện, hãy đánh dấu `[x]` vào các task đã hoàn thành.

---

## Phase 1: Xử lý Hardcoded API Endpoints
Mục tiêu: Đảm bảo toàn bộ HTTP calls sử dụng chung 1 nguồn URL từ `endpoints.ts`.

- [x] **Task 1.1: Cập nhật `endpoints.ts`**
  - ✅ Bổ sung `ai.insights`
  - ✅ Bổ sung `integrations.jira` (Cả GET và PUT)
  - ✅ Bổ sung `webhooks.retryDelivery`, `webhooks.deliveries`, `webhooks.deliveryStats`
  - ✅ Bổ sung `notifications.stream` (đã có, fix component dùng đúng)
  - ✅ Bổ sung `capec.list`, `scans.history`, `reports.templates`
  - ✅ Bổ sung `profile.sessions`, `profile.notificationSettings`
  - ✅ Bổ sung `search.recent`, `search.suggested`, `search.semanticSuggestions`

- [x] **Task 1.2: Refactor Components & Hooks**
  - ✅ `JiraConfig.tsx` — 2 chỗ hardcode → `ENDPOINTS.integrations.jira`
  - ✅ `AIInsights.tsx` — hardcode → `ENDPOINTS.ai.insights`
  - ✅ `useWebhookDeliveries.ts` — 3 chỗ hardcode → `ENDPOINTS.webhooks.*`
  - ✅ `useNotificationsSSE.ts` — hardcode string base URL → `ENDPOINTS.notifications.stream`
  - ✅ TypeScript check: **0 lỗi mới**

---

## Phase 2: Chuyển đổi Mock Data thành Fixtures & MSW Handlers
Mục tiêu: Bóc tách mảng dữ liệu giả (`mock data`) ra khỏi UI component, chuyển thành `fixtures` và dựng MSW Mock API.

- [x] **Task 2.1: Domain Profile, Reports, Notifications**
  - ✅ **Profile**: Tạo `mocks/fixtures/profile.fixture.ts` (sessions + notifSettings). MSW handler `GET /api/v1/profile/sessions`, `GET/PUT /api/v1/profile/notifications/settings`, `DELETE /api/v1/profile/sessions/:id`.
  - ✅ **Reports**: `ReportCenter.tsx` đã dùng `useReports` hook (API-compliant, không cần thay).
  - ✅ **Notifications**: `NotificationCenter.tsx` đã dùng `useNotificationList` hook (API-compliant, không cần thay).

- [x] **Task 2.2: Domain Scanning & Asset Intelligence**
  - ✅ **Scan Results**: Tạo `mocks/fixtures/nmap.fixture.ts` + `mocks/fixtures/zap.fixture.ts`. Handler `GET /api/v1/scans/:id/results/nmap` và `/zap` đã có trong `scan.handlers.ts`.
  - ✅ **Scan History**: Thêm handler `GET /api/v1/scans/history` vào `scan.handlers.ts`.
  - ✅ **CAPEC**: Tạo `mocks/fixtures/capec.fixture.ts` + `mocks/handlers/capec.handlers.ts` với `GET /api/v2/capec` và `GET /api/v2/capec/:id`.

- [x] **Task 2.3: Domain Integrations & Admin**
  - ✅ **Webhooks**: `WebhookEvents.tsx` đã được refactor ở Phase 1 (dùng `ENDPOINTS.webhooks.*`). Handlers đã có.
  - ✅ **API Keys**: `APIKeyManagement.tsx` đã dùng `useAPIKeys` hook (API-compliant).
  - ✅ **Users**: `UserManagement.tsx` đã dùng `useAdminUsers` hook (API-compliant).
  - ✅ **Audit Log**: `AuditLogs.tsx` đã dùng `useAuditLogs` hook (API-compliant).

- [x] **Task 2.4: Domain AI, Search & Risk**
  - ✅ **AI Triage**: `AITriage.tsx` đã dùng `useAITriageQueue` hook (API-compliant).
  - ✅ **Global Search**: Tạo `mocks/handlers/search.handlers.ts` với `GET /api/v1/search`, `/recent`, `/suggested`, `/semantic/suggestions`.
  - ✅ **Risk**: `RiskAcceptanceCenter.tsx` đã dùng `useRiskAcceptances` hook (API-compliant).

---

## Phase 3: Refactor Components (Tích hợp React Query)
Mục tiêu: Đưa `useQuery` và `apiClient` vào component để fetch data từ MSW, xóa bỏ mảng mock cũ.

- [x] **Task 3.1: Profile, Reports, Notifications Components**
  - ✅ Tạo hook `features/auth/hooks/useProfile.ts` (useSessions, useNotificationSettings, useUpdateNotificationSettings).
  - ✅ Refactor `UserProfile.tsx` — xóa `sessions[]` và `notifSettings[]` hardcode, dùng hooks. Avatar initials từ auth store.

- [x] **Task 3.2: Scanning & Intelligence Components**
  - ✅ Tạo hook `features/scanning/hooks/useScanResults.ts` (useNmapResults, useZAPResults).
  - ✅ Tạo hook `features/cve-intel/hooks/useCAPEC.ts` (useCAPECList, useCAPECDetail).
  - ✅ Refactor `NmapResults.tsx` — xóa `hosts[]` hardcode, dùng `useNmapResults`, thêm Loading/Error/Empty states.
  - ✅ Refactor `ZAPResults.tsx` — xóa `alerts[]` hardcode, dùng `useZAPResults`, risk breakdown tính từ live data.
  - ✅ Refactor `CAPECLibrary.tsx` — xóa `CAPEC_PATTERNS[]` hardcode (20 phần tử), dùng `useCAPECList`.

- [x] **Task 3.3: Integrations & Admin Components**
  - ✅ `WebhookEvents.tsx`, `APIKeyManagement.tsx`, `UserManagement.tsx`, `AuditLogs.tsx` đã API-compliant từ trước hoặc Phase 1.

- [x] **Task 3.4: AI, Search & Risk Components**
  - ✅ Tạo hooks useSearch, useRecentSearches, useSuggestedSearches trực tiếp trong `CommandPalette.tsx`.
  - ✅ Refactor `CommandPalette.tsx` (GlobalSearch) — xóa `RECENT[]`, `SUGGESTED[]`, `ALL_RESULTS[]` hardcode, dùng `useQuery`.
  - ✅ TypeScript check: **0 lỗi mới từ refactored files** (10 lỗi pre-existing không liên quan).

---
*Hướng dẫn thực thi cho AI*: Ở mỗi task, AI cần dùng `grep_search` để xác định chính xác đường dẫn file, di chuyển dữ liệu một cách cẩn thận và đảm bảo không phá vỡ UI/TypeScript typings cũ. Dùng `QueryBoundary` để hiển thị Skeleton/Loading state nếu có.
