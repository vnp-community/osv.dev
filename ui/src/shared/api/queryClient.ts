import { QueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { APIError } from '@/shared/types/api';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,          // 30s — default cho hầu hết queries
      gcTime: 5 * 60 * 1000,     // 5 min garbage collect
      retry: 2,
      refetchOnWindowFocus: false,
    },
    mutations: {
      onError: (error: unknown) => {
        const apiError = error as { response?: { data?: APIError } };
        const message =
          apiError?.response?.data?.message ?? 'An unexpected error occurred';
        toast.error(message);
      },
    },
  },
});

// ─── Query Key Factories ─────────────────────────────────────────────────────
// Pattern: [...baseKey, 'operation', params]

export const authKeys = {
  all: ['auth'] as const,
  me: () => [...authKeys.all, 'me'] as const,
};

export const dashboardKeys = {
  all: ['dashboard'] as const,
  metrics: (period: string) => [...dashboardKeys.all, 'metrics', period] as const,
};

export const cveKeys = {
  all: ['cves'] as const,
  search: (params: object) => [...cveKeys.all, 'search', params] as const,
  detail: (id: string) => [...cveKeys.all, 'detail', id] as const,
  kev: (params?: object) => [...cveKeys.all, 'kev', params] as const,
  kevStats: () => [...cveKeys.all, 'kev-stats'] as const,
  epss: (cveId: string) => [...cveKeys.all, 'epss', cveId] as const,
  semantic: (query: object) => [...cveKeys.all, 'semantic', query] as const,
  vendors: (params?: object) => [...cveKeys.all, 'vendors', params] as const,
  cwe: (id: string) => [...cveKeys.all, 'cwe', id] as const,
};

export const findingKeys = {
  all: ['findings'] as const,
  list: (filters: object) => [...findingKeys.all, 'list', filters] as const,
  detail: (id: string) => [...findingKeys.all, 'detail', id] as const,
  audit: (id: string) => [...findingKeys.all, 'audit', id] as const,
  riskAcceptances: (filters?: object) => [...findingKeys.all, 'risk-acceptances', filters] as const,
};

export const scanKeys = {
  all: ['scans'] as const,
  list: (filters?: object) => [...scanKeys.all, 'list', filters] as const,
  detail: (id: string) => [...scanKeys.all, 'detail', id] as const,
  nmapResults: (id: string) => [...scanKeys.all, 'results', 'nmap', id] as const,
  zapResults: (id: string) => [...scanKeys.all, 'results', 'zap', id] as const,
};

export const assetKeys = {
  all: ['assets'] as const,
  list: (filters?: object) => [...assetKeys.all, 'list', filters] as const,
  detail: (id: string) => [...assetKeys.all, 'detail', id] as const,
};

export const productKeys = {
  all: ['products'] as const,
  list: (filters?: object) => [...productKeys.all, 'list', filters] as const,
  detail: (id: string) => [...productKeys.all, 'detail', id] as const,
};

export const reportKeys = {
  all: ['reports'] as const,
  list: () => [...reportKeys.all, 'list'] as const,
};

export const adminKeys = {
  users: ['admin', 'users'] as const,
  audit: (filters?: object) => ['admin', 'audit', filters] as const,
  health: ['admin', 'health'] as const,
  settings: ['admin', 'settings'] as const,
};
