/**
 * Fixture dữ liệu cho Reports.
 * Dùng trong report.handlers.ts và các unit tests.
 *
 * ⚠️ Chỉ dùng trong src/mocks/
 */

export interface ReportFixture {
  id: string;
  name?: string;
  type: 'Executive' | 'Technical' | 'Compliance';
  format: 'pdf' | 'html' | 'csv' | 'excel' | 'json';
  status: 'pending' | 'generating' | 'completed' | 'failed';
  finding_count?: number;
  file_size_bytes?: number;
  generated_at?: string;
  artifact_url?: string;
  created_at: string;
  created_by: string;
}

export const reportsFixture: ReportFixture[] = [
  {
    id: 'R-047',
    name: 'Q2 2026 Executive Summary',
    type: 'Executive',
    format: 'pdf',
    status: 'completed',
    finding_count: 47,
    file_size_bytes: 2_516_582,
    generated_at: '2026-06-14T09:00:00Z',
    artifact_url: 'https://storage.company.com/reports/R-047.pdf',
    created_at: '2026-06-14T09:00:00Z',
    created_by: 'carol@company.com',
  },
  {
    id: 'R-046',
    name: 'Banking App Technical Report',
    type: 'Technical',
    format: 'pdf',
    status: 'completed',
    finding_count: 128,
    file_size_bytes: 9_123_456,
    generated_at: '2026-06-13T16:30:00Z',
    artifact_url: 'https://storage.company.com/reports/R-046.pdf',
    created_at: '2026-06-13T16:30:00Z',
    created_by: 'bob.chen@company.com',
  },
  {
    id: 'R-045',
    name: 'PCI DSS Compliance Q2',
    type: 'Compliance',
    format: 'pdf',
    status: 'completed',
    finding_count: 34,
    file_size_bytes: 4_300_000,
    generated_at: '2026-06-12T11:00:00Z',
    artifact_url: 'https://storage.company.com/reports/R-045.pdf',
    created_at: '2026-06-12T11:00:00Z',
    created_by: 'carol@company.com',
  },
];

export const reportTemplatesFixture = [
  {
    id: 'exec',
    name: 'Executive Summary',
    description: 'High-level overview for C-level presentations',
    type: 'Executive' as const,
  },
  {
    id: 'tech',
    name: 'Technical Report',
    description: 'Detailed findings with CVE details and remediation steps',
    type: 'Technical' as const,
  },
  {
    id: 'comp',
    name: 'Compliance Report',
    description: 'Mapped to PCI DSS, ISO 27001, SOC2, NIST frameworks',
    type: 'Compliance' as const,
  },
];
