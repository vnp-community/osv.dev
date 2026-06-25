/**
 * useReports — React Query hooks cho báo cáo.
 * TASK-P4-04: thay thế const reports[], const templates[] trong ReportCenter.tsx
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';

// ─── Types ────────────────────────────────────────────────────────────────────

export type ReportType = 'Executive' | 'Technical' | 'Compliance';
export type ReportFormat = 'pdf' | 'html' | 'csv' | 'excel' | 'json';
export type ReportStatus = 'pending' | 'generating' | 'completed' | 'failed';

export interface ReportRun {
  id: string;
  name?: string;
  type: ReportType;
  format: ReportFormat;
  status: ReportStatus;
  finding_count?: number;
  file_size_bytes?: number;
  generated_at?: string;
  artifact_url?: string;
  created_at: string;
  created_by: string;
  product_name?: string;
}

export interface ReportsResponse {
  reports: ReportRun[];
  total: number;
  last_generated_at?: string;
}

export interface ReportTemplate {
  id: string;
  name: string;
  description: string;
  type: ReportType;
}

export interface ReportTemplatesResponse {
  templates: ReportTemplate[];
}

export interface CreateReportRequest {
  template_id: string;
  type: ReportType;
  format: ReportFormat;
  product_id?: string;
  date_range?: string;
  severity_filter?: string;
  status_filter?: string;
}

// ─── Query Keys ──────────────────────────────────────────────────────────────

export const reportKeys = {
  all: ['reports'] as const,
  list: () => [...reportKeys.all, 'list'] as const,
  templates: () => [...reportKeys.all, 'templates'] as const,
};

// ─── GET /api/v1/reports ──────────────────────────────────────────────────────

export function useReports() {
  return useQuery<ReportsResponse>({
    queryKey: reportKeys.list(),
    queryFn: async () => {
      const { data } = await apiClient.get<ReportsResponse>(ENDPOINTS.reports.list);
      return data;
    },
    staleTime: 60_000,
  });
}

// ─── GET /api/v1/reports/templates ────────────────────────────────────────────

export function useReportTemplates() {
  return useQuery<ReportTemplatesResponse>({
    queryKey: reportKeys.templates(),
    queryFn: async () => {
      const { data } = await apiClient.get<ReportTemplatesResponse>(
        `${ENDPOINTS.reports.list}/templates`
      );
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

// ─── POST /api/v1/reports ────────────────────────────────────────────────────

export function useCreateReport() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (req: CreateReportRequest) => {
      const { data } = await apiClient.post<ReportRun>(ENDPOINTS.reports.create, req);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: reportKeys.all });
    },
  });
}
