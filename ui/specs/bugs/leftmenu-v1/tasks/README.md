# Tasks — Fix Left Menu Bugs (leftmenu-v1)

Các tác vụ được tách ra từ [`solutions.md`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/solutions/solutions.md) để AI thực thi độc lập.

## Danh sách Tasks

| Task | Tên | Độ ưu tiên | File thay đổi chính | Trạng thái |
|---|---|---|---|---|
| [TASK-01](./TASK-01-parent-nav.md) | Thêm điều hướng cho 9 mục cha | 🟡 Medium | `Sidebar.tsx` | ✅ DONE |
| [TASK-02](./TASK-02-mitigated-route.md) | Fix item "Mitigated" không có path | 🔴 High | `Sidebar.tsx` | ✅ DONE |
| [TASK-03](./TASK-03-nmap-zap-redirect.md) | Fix Nmap/ZAP Results dùng path sai | 🟡 Medium | `router.tsx`, new components | ✅ DONE |
| [TASK-04](./TASK-04-asset-detail.md) | Fix "Asset Detail" trỏ sai trang | 🔴 High | `Sidebar.tsx` | ✅ DONE |
| [TASK-05](./TASK-05-jira-route.md) | Fix "Jira" trỏ nhầm Webhooks | 🔴 High | `Sidebar.tsx`, `router.tsx`, new component | ✅ DONE |
| [TASK-06](./TASK-06-duplicate-routes.md) | Fix Duplicate Routes bằng URL params | 🟡 Medium | `Sidebar.tsx`, feature components | ✅ DONE |
| [TASK-07](./TASK-07-cleanup-orphan-ids.md) | Dọn dẹp Orphan IDs dư thừa | 🟢 Low | `Sidebar.tsx` | ✅ DONE |

## Thứ tự thực thi khuyến nghị

```
TASK-04 → TASK-02 → TASK-05  (High priority — fix ngay)
    ↓
TASK-01 → TASK-06 → TASK-03  (Medium priority)
    ↓
TASK-07                        (Low priority — cleanup)
```

## Nguồn tham chiếu

- **Bugs:** [`bugs.md`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/bugs.md)
- **Solutions:** [`solutions.md`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/bugs/leftmenu-v1/solutions/solutions.md)
- **Architecture:** [`architecture.md`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/specs/architecture.md)
- **Sidebar source:** [`Sidebar.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/components/Sidebar.tsx)
- **Router source:** [`router.tsx`](file:///Users/binhnt/Lab/sec/cve/osv.dev/ui/src/app/router.tsx)
