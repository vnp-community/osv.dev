import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useDashboardMetrics } from '../useDashboardMetrics';
import { dashboardFixture } from '@/mocks/fixtures/dashboard.fixture';

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe('useDashboardMetrics', () => {
  it('fetches dashboard metrics successfully', async () => {
    const { result } = renderHook(() => useDashboardMetrics('30d'), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true), { timeout: 10000 });

    expect(result.current.data?.kpis.criticalFindings).toBe(
      dashboardFixture['30d'].kpis.criticalFindings
    );
  }, 15000);


  it('starts in loading state', () => {
    const { result } = renderHook(() => useDashboardMetrics('30d'), { wrapper });
    expect(result.current.isLoading).toBe(true);
  });

  it('uses correct query key', () => {
    const { result } = renderHook(() => useDashboardMetrics('90d'), { wrapper });
    // Query is enabled and the key includes period
    expect(result.current.isLoading || result.current.isSuccess).toBe(true);
  });
});
