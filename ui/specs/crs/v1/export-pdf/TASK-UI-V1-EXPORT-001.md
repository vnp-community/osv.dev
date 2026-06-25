# TASK-UI-V1-EXPORT-001 — Integrate Server-Side PDF Export

| Field | Value |
|-------|-------|
| **Task ID** | TASK-UI-V1-EXPORT-001 |
| **Module** | `ui/src/features/dashboard/` |
| **Solution Ref** | [SOL-UI-V1-EXPORT-001](./SOL-UI-V1-EXPORT-001.md) |
| **Priority** | 🔴 P2 |
| **Estimated** | 2h |
| **Status** | 🕒 Pending |

## Context
Thực thi giải pháp xuất file PDF từ phía Backend. Xóa bỏ logic `window.print()` tạm thời và thay thế bằng việc kết nối API.

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `ui/src/features/dashboard/api/dashboardApi.ts` |
| MODIFY | `ui/src/features/dashboard/components/Dashboard.tsx` |
| MODIFY | `ui/src/features/dashboard/components/RiskOverview.tsx` |

## Implementation Steps

### Step 1: Thêm API endpoint
Cập nhật `dashboardApi.ts` để gọi API lấy định dạng PDF. (Lưu ý: set `responseType: 'blob'`)
```typescript
export const dashboardApi = {
  // ... getMetrics
  exportPDF: async (period: string): Promise<Blob> => {
    const response = await apiClient.get(ENDPOINTS.dashboard.export, {
      params: { period },
      responseType: 'blob',
    });
    return response.data as Blob;
  }
};
```

### Step 2: Tạo Mutation Hook
Tạo custom hook `useExportDashboard(period)` sử dụng `useMutation` từ `@tanstack/react-query` để quản lý trạng thái loading khi đang tải PDF.

### Step 3: Cập nhật sự kiện onClick tại Dashboard.tsx và RiskOverview.tsx
- Sửa hàm `onClick` của nút **Export PDF** và **Export Risk Report**.
- Khi mutation thành công, sử dụng `URL.createObjectURL(blob)` để sinh ra link tải.
- Tạo một thẻ `<a>` ảo, gán link vừa sinh ra và gọi lệnh `.click()` để trình duyệt tải file `dashboard-report.pdf` về máy.
- Bổ sung hiệu ứng loading (Spinner) trên nút Export PDF trong lúc chờ Backend phản hồi.

## Verification
1. Chọn khoảng thời gian "90d" trên Dashboard.
2. Bấm nút "Export PDF". Nút hiển thị spinner xoay vòng.
3. Kiểm tra trình duyệt tự động tải xuống file `dashboard-report.pdf`.
4. Mở file PDF và đảm bảo dữ liệu khớp với "90d" đã chọn.
