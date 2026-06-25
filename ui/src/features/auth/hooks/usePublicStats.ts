/**
 * usePublicStats — Query hook cho public platform stats trên LoginScreen.
 * TASK-P5-01: Dùng plain axios (không auth), không retry khi backend down.
 */
import { useQuery } from '@tanstack/react-query';
import axios from 'axios';

// ─── Types ────────────────────────────────────────────────────────────────────

export interface PublicStats {
  totalCVEs: string;
  scansToday: number;
  findingAccuracy: string;
  uptimeSLA: string;
  threatIndicators: {
    criticalThreats: number;
    kevActive: number;
    assetsAtRisk: number;
  };
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function usePublicStats() {
  return useQuery<PublicStats>({
    queryKey: ['public', 'stats'],
    queryFn: async () => {
      const { data } = await axios.get<PublicStats>('/api/v2/public/stats');
      return data;
    },
    staleTime: 5 * 60_000,  // 5 phút
    retry: false,            // Login page — không retry nếu backend down
    refetchOnWindowFocus: false,
  });
}
