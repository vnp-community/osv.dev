// API trả về snake_case — map đúng vào TypeScript
export type SecurityGrade = 'A' | 'A-' | 'B+' | 'B' | 'B-' | 'C+' | 'C' | 'D' | 'F';

export interface DashboardKPIs {
  critical_findings: number;
  high_findings: number;
  total_assets: number;
  high_risk_assets: number;
  active_scans: number;
  queued_scans: number;
  security_grade: SecurityGrade;
  security_score: number;       // 0-100
  sla_compliance: number;       // percentage
  sla_at_risk: number;
  sla_breached: number;
}

export interface RiskTrendPoint {
  month: string;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface SeverityDistribution {
  critical: number;
  high: number;
  medium: number;
  low: number;
  total: number;
}

export interface ProductGradeItem {
  id: string;
  name: string;
  grade: string;
  score: number;
  critical_count: number;
  high_count: number;
}

export interface KEVAlert {
  cve_id: string;
  vendor: string;
  product: string;
  date_added: string;
  is_ransomware: boolean;
}

export interface RecentScan {
  id: string;
  name: string;
  type: string;
  status: string;
  targets: string[];
  finding_count: number;
  started_at: string;
  completed_at: string | null;
  duration_ms: number | null;
  created_by: string;
}

export interface SLABreach {
  finding_id: string;
  title: string;
  cve_id: string;
  severity: string;
  product_name: string;
  sla_expiration_date: string;
  days_overdue: number;
}

export interface DashboardData {
  kpis: DashboardKPIs;
  risk_trend: RiskTrendPoint[];
  severity_distribution: SeverityDistribution;
  product_grades: ProductGradeItem[];
  kev_alerts: KEVAlert[];
  recent_scans: RecentScan[];
  sla_breaches: SLABreach[];
}

export interface SLASummary {
  total_active_findings: number;
  compliance_percent: number;
  breached: number;
  at_risk: number;
  ok: number;
}

export interface SLADashboardData {
  summary: SLASummary;
  compliance_trend: Array<{ month: string; compliance_percent: number }>;
  breached_findings: SLABreach[];
  at_risk_findings: Array<{
    finding_id: string;
    title: string;
    severity: string;
    product_name: string;
    sla_expiration_date: string;
    hours_remaining: number;
  }>;
  by_product: Array<{
    product_id: string;
    product_name: string;
    compliance_percent: number;
    breached: number;
    at_risk: number;
    ok: number;
  }>;
  total_breached: number;
  total_at_risk: number;
  page: number;
  page_size: number;
}
