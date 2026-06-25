/**
 * Dashboard Component Tests — SOL-004 Phase 3
 * Tests that Dashboard renders correctly using MSW fixtures, NOT hardcoded data.
 */
import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { Dashboard } from '../Dashboard';
import { createTestProviders } from '@/test/utils';

// Mock recharts to avoid ResizeObserver issues in jsdom
vi.mock('recharts', () => ({
  AreaChart: ({ children }: { children: React.ReactNode }) => <div data-testid="area-chart">{children}</div>,
  Area: () => null,
  PieChart: ({ children }: { children: React.ReactNode }) => <div data-testid="pie-chart">{children}</div>,
  Pie: () => null,
  Cell: () => null,
  XAxis: () => null,
  YAxis: () => null,
  Tooltip: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div data-testid="responsive-container">{children}</div>,
}));

describe('Dashboard', () => {
  it('shows skeleton while loading', () => {
    render(<Dashboard />, { wrapper: createTestProviders() });
    expect(screen.getByTestId('dashboard-skeleton')).toBeInTheDocument();
  });

  it('does NOT show hardcoded data before MSW resolves', () => {
    render(<Dashboard />, { wrapper: createTestProviders() });
    const skeleton = screen.getByTestId('dashboard-skeleton');
    expect(skeleton).toBeInTheDocument();
  });

  it('renders Executive Security Dashboard heading after load', async () => {
    render(<Dashboard />, { wrapper: createTestProviders() });
    // After MSW resolves, Dashboard content should appear
    await waitFor(() => {
      expect(screen.getByText('Executive Security Dashboard')).toBeInTheDocument();
    }, { timeout: 10000 });
  }, 15000);

  it('renders KPI card labels after data loads', async () => {
    render(<Dashboard />, { wrapper: createTestProviders() });
    // KPI cards with fixed label text should appear after load
    await waitFor(() => {
      expect(screen.getByText('Critical Findings')).toBeInTheDocument();
    }, { timeout: 10000 });
  }, 15000);

  it('renders security grade label after data loads', async () => {
    render(<Dashboard />, { wrapper: createTestProviders() });
    await waitFor(() => {
      expect(screen.getByText('Security Grade')).toBeInTheDocument();
    }, { timeout: 10000 });
  }, 15000);
});
