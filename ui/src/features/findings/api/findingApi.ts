import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Finding, FindingStatus, RiskAcceptance, SLAConfig } from '@/shared/types/finding';

export interface FindingsListParams {
  status?: FindingStatus[];
  severity?: string[];
  productId?: string;
  engagementId?: string;
  testId?: string;
  cveId?: string;
  slaStatus?: 'ok' | 'at_risk' | 'breached';
  assignedTo?: string;
  isKev?: boolean;
  dateFrom?: string;
  dateTo?: string;
  page?: number;
  pageSize?: number;
  sortBy?: 'severity_desc' | 'sla_asc' | 'created_desc' | 'epss_desc';
  q?: string;
}

export interface FindingsListResponse {
  findings: Finding[];
  total: number;
  page: number;
  page_size: number;
  by_severity: Record<string, number>;
  by_status: Record<string, number>;
  sla_stats: { breached: number; at_risk: number; ok: number };
}

export interface BulkOperationResponse {
  success_count: number;
  failed_ids: string[];
  errors: string[];
}

export interface FindingNote {
  id: string;
  finding_id: string;
  content: string;
  created_by: string;
  created_at: string;
}

export const findingApi = {
  list: async (params: FindingsListParams): Promise<FindingsListResponse> => {
    const { data } = await apiClient.get<FindingsListResponse>(
      ENDPOINTS.findings.list,
      { params }
    );
    // Normalize — đảm bảo findings luôn là array dù server trả partial/null
    return {
      findings:    Array.isArray(data?.findings)   ? data.findings   : [],
      total:       typeof data?.total === 'number' ? data.total      : 0,
      page:        data?.page       ?? 1,
      page_size:   data?.page_size  ?? 20,
      by_severity: data?.by_severity ?? {},
      by_status:   data?.by_status   ?? {},
      sla_stats:   data?.sla_stats   ?? { breached: 0, at_risk: 0, ok: 0 },
    };
  },

  getStats: async (productId?: string) => {
    const { data } = await apiClient.get(ENDPOINTS.findings.stats, {
      params: productId ? { product_id: productId } : undefined,
    });
    return data;
  },

  getById: async (id: string): Promise<Finding> => {
    const { data } = await apiClient.get<Finding>(ENDPOINTS.findings.detail(id));
    return data;
  },

  update: async (
    id: string,
    payload: { status?: FindingStatus; comment?: string; assigned_to?: string; vex_justification?: string }
  ): Promise<Finding> => {
    const { data } = await apiClient.patch<Finding>(
      ENDPOINTS.findings.patch(id),
      payload
    );
    return data;
  },

  bulkClose: async (finding_ids: string[], comment?: string): Promise<BulkOperationResponse> => {
    const { data } = await apiClient.post(ENDPOINTS.findings.bulkClose, {
      finding_ids,
      comment,
    });
    return data as BulkOperationResponse;
  },

  bulkReopen: async (finding_ids: string[], comment?: string): Promise<BulkOperationResponse> => {
    const { data } = await apiClient.post(ENDPOINTS.findings.bulkReopen, {
      finding_ids,
      comment,
    });
    return data as BulkOperationResponse;
  },

  bulkAssign: async (finding_ids: string[], assigned_to: string): Promise<BulkOperationResponse> => {
    const { data } = await apiClient.post(ENDPOINTS.findings.bulkAssign, {
      finding_ids,
      assigned_to,
    });
    return data as BulkOperationResponse;
  },

  getAudit: async (id: string) => {
    const { data } = await apiClient.get(ENDPOINTS.findings.audit(id));
    return data;
  },

  getNotes: async (id: string): Promise<{ notes: FindingNote[] }> => {
    const { data } = await apiClient.get(ENDPOINTS.findings.notes(id));
    return data as { notes: FindingNote[] };
  },

  addNote: async (id: string, content: string): Promise<FindingNote> => {
    const { data } = await apiClient.post(ENDPOINTS.findings.notes(id), { content });
    return data as FindingNote;
  },

  // Risk Acceptances
  listRiskAcceptances: async (params?: { product_id?: string; is_expired?: boolean; page?: number }) => {
    const { data } = await apiClient.get(ENDPOINTS.riskAcceptances.list, { params });
    return data;
  },

  createRiskAcceptance: async (payload: {
    product_id: string;
    finding_ids: string[];
    expiration_date: string;
    retest_date?: string;
    reason: string;
    approved_by: string;
  }): Promise<RiskAcceptance> => {
    const { data } = await apiClient.post(ENDPOINTS.riskAcceptances.create, payload);
    return data as RiskAcceptance;
  },

  deleteRiskAcceptance: async (id: string): Promise<{ success: boolean; reopened_finding_ids: string[] }> => {
    const { data } = await apiClient.delete(ENDPOINTS.riskAcceptances.delete(id));
    return data as { success: boolean; reopened_finding_ids: string[] };
  },

  // SLA Config
  getSLAConfig: async (): Promise<{ global: SLAConfig; product_overrides: (SLAConfig & { product_id: string; product_name: string })[] }> => {
    const { data } = await apiClient.get(ENDPOINTS.sla.config);
    return data as { global: SLAConfig; product_overrides: (SLAConfig & { product_id: string; product_name: string })[] };
  },

  updateSLAConfig: async (config: { global: SLAConfig; product_overrides?: unknown[] }) => {
    const { data } = await apiClient.put(ENDPOINTS.sla.config, config);
    return data;
  },

  // Audit Log (admin)
  getAuditLog: async (params?: {
    user_id?: string;
    action?: string;
    entity_type?: string;
    entity_id?: string;
    page?: number;
    page_size?: number;
  }) => {
    const { data } = await apiClient.get(ENDPOINTS.audit.log, { params });
    return data;
  },
};
