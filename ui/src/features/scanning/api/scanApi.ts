import { apiClient } from '@/shared/api/client';
import { ENDPOINTS } from '@/shared/api/endpoints';
import type { Scan, NmapHost, ZAPAlert, ScheduledScan } from '@/shared/types/scan';

export interface CreateScanPayload {
  name: string;
  type: string;
  targets: string[];
  options?: {
    scan_profile?: string;
    port_range?: string;
    max_depth?: number;
    timeout?: number;
  };
  engagement_id?: string;
  schedule_frequency?: 'once' | 'daily' | 'weekly' | 'custom';
  schedule_cron_expr?: string;
}

export interface ScansListResponse {
  scans: Scan[];
  total: number;
  page: number;
  page_size: number;
  stats: {
    active_scans: number;
    completed_today: number;
    total_findings_today: number;
    scheduled_scans: number;
  };
}

export interface NmapResultsResponse {
  scan_id: string;
  hosts: NmapHost[];
  total_hosts: number;
  hosts_up: number;
  total_findings: number;
}

export interface ZAPResultsResponse {
  scan_id: string;
  target_url: string;
  alerts: ZAPAlert[];
  total: number;
  by_risk: Record<string, number>;
}

export interface ImportScanPayload {
  file: File;
  tool_name: string;
  engagement_id: string;
  test_title?: string;
}

export const scanApi = {
  list: async (params?: {
    status?: string;
    type?: string;
    page?: number;
    page_size?: number;
    sort_by?: string;
  }): Promise<ScansListResponse> => {
    const { data } = await apiClient.get(ENDPOINTS.scans.list, { params });
    
    const scansList = data?.scans || data?.items || [];
    const normalizedScans = scansList.map((s: any) => ({
      ...s,
      id: s.id || s.scan_id || s.ID || s._id,
    }));

    return {
      ...data,
      scans: normalizedScans,
    } as ScansListResponse;
  },

  create: async (payload: CreateScanPayload): Promise<Scan> => {
    const { data } = await apiClient.post<Scan>(ENDPOINTS.scans.create, payload);
    return {
      ...data,
      id: data.id || (data as any).scan_id || (data as any).ID || (data as any)._id,
    } as Scan;
  },

  getById: async (id: string): Promise<Scan> => {
    const { data } = await apiClient.get<Scan>(ENDPOINTS.scans.detail(id));
    return {
      ...data,
      id: data.id || (data as any).scan_id || (data as any).ID || (data as any)._id || id,
    } as Scan;
  },

  cancel: async (id: string): Promise<{ success: boolean; scan_id: string; status: string }> => {
    const { data } = await apiClient.post(ENDPOINTS.scans.cancel(id));
    return data as { success: boolean; scan_id: string; status: string };
  },

  getNmapResults: async (id: string): Promise<NmapResultsResponse> => {
    const { data } = await apiClient.get(ENDPOINTS.scans.nmap(id));
    return data as NmapResultsResponse;
  },

  getZAPResults: async (id: string): Promise<ZAPResultsResponse> => {
    const { data } = await apiClient.get(ENDPOINTS.scans.zap(id));
    return data as ZAPResultsResponse;
  },

  getScheduled: async (): Promise<{ scheduled_scans: ScheduledScan[]; total: number }> => {
    const { data } = await apiClient.get(ENDPOINTS.scans.scheduled);
    return data as { scheduled_scans: ScheduledScan[]; total: number };
  },

  importScan: async (payload: ImportScanPayload): Promise<{ import_id: string; status: string; findings_count: number | null }> => {
    const formData = new FormData();
    formData.append('file', payload.file);
    formData.append('tool_name', payload.tool_name);
    formData.append('engagement_id', payload.engagement_id);
    if (payload.test_title) formData.append('test_title', payload.test_title);

    const { data } = await apiClient.post(ENDPOINTS.scans.create + '/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return data as { import_id: string; status: string; findings_count: number | null };
  },

  /**
   * Stream scan progress via SSE.
   * Returns native EventSource for SSE streaming.
   */
  streamProgress: (id: string, token: string): EventSource => {
    const url = `${ENDPOINTS.scans.stream(id)}?token=${encodeURIComponent(token)}`;
    return new EventSource(url);
  },
};
