/**
 * Fixture dữ liệu cho Risk Acceptances.
 * Dùng trong finding.handlers.ts và các unit tests.
 *
 * ⚠️ Chỉ dùng trong src/mocks/
 */

export interface RiskAcceptanceFixture {
  id: string;
  product_id: string;
  product_name: string;
  finding_ids: string[];
  finding_title: string;
  cve_id?: string;
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  expiration_date: string;
  retest_date?: string;
  reason: string;
  approved_by: string;
  approved_by_id: string;
  is_expired: boolean;
  days_left: number;
  created_at: string;
}

export const riskAcceptancesFixture: RiskAcceptanceFixture[] = [
  {
    id: 'RA-012',
    product_id: 'p-1',
    product_name: 'Banking App',
    finding_ids: ['F-2831'],
    finding_title: 'Cisco IOS XE Web UI Privilege Escalation',
    cve_id: 'CVE-2023-20198',
    severity: 'Critical',
    expiration_date: '2026-09-19T00:00:00Z',
    retest_date: '2026-08-01T00:00:00Z',
    reason: 'Network segmentation and monitoring controls reduce exploitability. Vendor patch to be applied during scheduled maintenance window.',
    approved_by: 'carol@company.com',
    approved_by_id: 'u-1',
    is_expired: false,
    days_left: 92,
    created_at: '2026-06-19T00:00:00Z',
  },
  {
    id: 'RA-011',
    product_id: 'p-2',
    product_name: 'Network Infra',
    finding_ids: ['F-2819', 'F-2820'],
    finding_title: 'Apache HTTP Server Path Traversal',
    cve_id: 'CVE-2021-41773',
    severity: 'High',
    expiration_date: new Date(Date.now() + 15 * 86_400_000).toISOString(),
    reason: 'Legacy system cannot be updated immediately. WAF rules deployed to block exploitation.',
    approved_by: 'carol@company.com',
    approved_by_id: 'u-1',
    is_expired: false,
    days_left: 15,
    created_at: '2026-05-01T00:00:00Z',
  },
  {
    id: 'RA-009',
    product_id: 'p-1',
    product_name: 'Banking App',
    finding_ids: ['F-2798'],
    finding_title: 'Spring Framework Expression Injection',
    cve_id: 'CVE-2022-22963',
    severity: 'High',
    expiration_date: '2026-06-01T00:00:00Z',
    reason: 'Awaiting vendor-supplied patch. Application firewall rules mitigate risk.',
    approved_by: 'bob.chen@company.com',
    approved_by_id: 'u-2',
    is_expired: true,
    days_left: -18,
    created_at: '2026-03-01T00:00:00Z',
  },
];
