# TASK-UI-001 — Cài đặt Dependencies

| Field | Value |
|-------|-------|
| **Task ID** | TASK-UI-001 |
| **Module** | `ui` |
| **Solution Ref** | [SOL-002 §2](../solutions/SOL-002-phase1-foundation.md#2-step-1-cài-đặt-dependencies) |
| **Priority** | 🔴 P0 |
| **Depends On** | — |
| **Estimated** | 30m |
| **Status** | ✅ Completed |
| **Completed** | 2026-06-17 |

---

## Context

`package.json` hiện tại đã có `react-router`, `react-hook-form`, `recharts`, `@radix-ui/*` nhưng **thiếu** các thư viện cốt lõi cho architecture mục tiêu: Zustand (global state), React Query (server state), Axios (HTTP), MSW (dev mock), Zod (validation), react-virtual (performance).

---

## Goal

Cài đặt tất cả dependencies còn thiếu vào `ui/package.json` via `pnpm`.

---

## Target Files

| Action | File Path |
|--------|-----------|
| MODIFY | `ui/package.json` (tự động bởi pnpm) |
| MODIFY | `ui/pnpm-workspace.yaml` (nếu cần) |

---

## Implementation

### Step 1: Runtime dependencies

```bash
cd /Users/binhnt/Lab/sec/cve/osv.dev/ui
pnpm add \
  zustand \
  @tanstack/react-query \
  @tanstack/react-query-devtools \
  @tanstack/react-virtual \
  axios \
  msw \
  zod \
  @hookform/resolvers
```

### Step 2: Dev dependencies

```bash
pnpm add -D \
  vitest \
  @testing-library/react \
  @testing-library/user-event \
  @testing-library/jest-dom \
  jsdom \
  @playwright/test
```

### Step 3: Update `vite.config.ts`

```typescript
// ui/vite.config.ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test/setup.ts',
  },
});
```

### Step 4: Update `tsconfig.json`

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "jsx": "react-jsx",
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    },
    "types": ["vitest/globals"]
  },
  "include": ["src", "vite.config.ts"]
}
```

### Step 5: Tạo `.env` files

```env
# ui/.env.development
VITE_API_BASE_URL=http://localhost:8080
VITE_APP_ENV=development
VITE_ENABLE_MSW=true
VITE_SENTRY_DSN=
```

```env
# ui/.env.development.local  (gitignored — override khi backend sẵn sàng)
VITE_ENABLE_MSW=false
VITE_API_BASE_URL=http://localhost:8080
```

```env
# ui/.env.production
VITE_API_BASE_URL=https://api.osv.internal
VITE_APP_ENV=production
VITE_ENABLE_MSW=false
```

### Step 6: Thêm `.env.development.local` vào `.gitignore`

```gitignore
# ui/.gitignore  (thêm dòng)
.env.development.local
.env.local
```

---

## Verification

```bash
cd ui/

# Verify packages installed
pnpm list zustand @tanstack/react-query axios msw zod

# Verify build still works
pnpm build

# Verify dev server starts
pnpm dev
```

**Expected output:**
- `pnpm list` shows zustand, @tanstack/react-query, axios, msw, zod, @hookform/resolvers
- `pnpm build` completes without errors
- `pnpm dev` starts on port 3000

---

## Checklist

- [x] `pnpm add zustand @tanstack/react-query @tanstack/react-query-devtools @tanstack/react-virtual axios msw zod @hookform/resolvers`
- [x] `pnpm add -D vitest @testing-library/react @testing-library/user-event @testing-library/jest-dom jsdom @playwright/test`
- [x] `vite.config.ts` updated với `@` alias + proxy + test config
- [x] `tsconfig.json` updated với `paths` và `baseUrl`
- [x] `.env.development` tạo với `VITE_ENABLE_MSW=true`
- [x] `.env.development.local` tạo (gitignored)
- [x] `.env.production` tạo với `VITE_ENABLE_MSW=false`
- [x] `pnpm build` thành công
- [x] `pnpm dev` khởi động thành công
