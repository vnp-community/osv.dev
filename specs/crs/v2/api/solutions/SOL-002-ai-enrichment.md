# Solution 002: Implement AI Enrichment

**Status**: Proposed
**Target Service**: `ai-service`, `apps/osv` (Gateway), NATS
**Related CR**: [CR-002-ai-enrichment.md](../CR-002-ai-enrichment.md)

## 1. Giới thiệu Dịch vụ Mới (`ai-service`)
Theo thiết kế tổng thể (TDD - Phần 1.2), `ai-service` được quy hoạch cho phiên bản v3.0. Ta sẽ xây dựng service này tuân thủ Clean Architecture.

*   **Trách nhiệm**: Gọi API LLM (OpenAI, Gemini hoặc Local Llama), xử lý prompt engineering, update embeddings.
*   **Database**: Không có Schema riêng, sử dụng Redis (`osv:embed:{id}`) làm cache. Truy xuất trực tiếp API hoặc giao tiếp với Data/Finding service qua NATS.

## 2. API Design & Gateway Router
Bổ sung cấu hình routing vào `apps/osv/internal/gateway/router.go`:
```go
// Triage Endpoints
aiRouter.Post("/triage/{findingId}", proxy.Forward("ai-service:8089"))
aiRouter.Post("/triage/{findingId}/review", proxy.Forward("ai-service:8089"))
aiRouter.Get("/triage/queue", proxy.Forward("ai-service:8089"))

// Enrichment Endpoints
aiRouter.Get("/enrichment", proxy.Forward("ai-service:8089"))
aiRouter.Post("/enrichment/trigger", proxy.Forward("ai-service:8089"))
aiRouter.Get("/enrichment/{cveId}", proxy.Forward("ai-service:8089"))
```

## 3. Kiến trúc Luồng Dữ Liệu (Data Flow)

### 3.1 AI Triage (Asynchronous via NATS)
Để không làm treo Frontend khi gọi API LLM tốn thời gian:
1.  **POST `/api/v1/ai/triage/{id}`**: 
    *   `ai-service` nhận request, trả về `HTTP 202 Accepted` ngay lập tức.
    *   Tạo job ném vào goroutine hoặc queue nội bộ.
2.  **Processing**:
    *   `ai-service` gọi `finding-service` nội bộ (hoặc qua gRPC) để lấy thông tin chi tiết của Finding.
    *   Gửi prompt kèm dữ liệu Finding lên LLM (VD: Đánh giá xem đây là False Positive hay không?).
3.  **Result Delivery**:
    *   Khi có kết quả, `ai-service` publish NATS event `finding.ai_triaged`.
    *   `finding-service` subscribe event này, cập nhật Database (thêm note hoặc thay đổi State).
    *   `notification-service` bắn In-app alert cho user biết quá trình Triage đã xong.

### 3.2 AI Enrichment (Embedding & CVE Data)
Đồng bộ với quá trình Ingestion từ `data-service`:
1.  **Trigger**: Lắng nghe event `ingestion.cve.synced` từ NATS.
2.  **Processing**: `ai-service` lấy description của CVE, tạo Embedding 1536-dim.
3.  **Update**: Cập nhật lại vào PostgreSQL bảng `cves` cột `embedding`.
