# TASK-H2-001 — Fix BUG-H2-001,002,003: asset-service handler stubs

> **Bugs**: BUG-H2-001 (GetAsset shell), BUG-H2-002 (GetHistory stub), BUG-H2-003 (GetFindings stub)
> **Solution**: [SOL-H2-A](../solutions/SOL-H2-A-asset-service-stubs.md)
> **Status**: ✅ Done — Build verified ✓

## Checklist

- [x] Thêm `Get(ctx, id)` vào `AssetCRUDUseCase.crud.go`
- [x] Implement `GetAsset` handler thực sự (query DB qua `crudUC.Get`)
- [x] Fix `GetHistory` trả đúng format `{"history":[],"total":0}`
- [x] Fix `GetFindings` trả đúng format với graceful fallback

## Files Modified

- `services/asset-service/internal/usecase/asset/crud.go` [MODIFY]
- `services/asset-service/internal/delivery/http/handlers.go` [MODIFY]
