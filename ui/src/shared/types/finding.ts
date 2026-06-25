import type { Severity } from './cve';

export type FindingStatus =
  | 'active'
  | 'mitigated'
  | 'false_positive'
  | 'risk_accepted'
  | 'out_of_scope'
  | 'duplicate';

export type SLAStatus = 'ok' | 'at_risk' | 'breached';

export interface Finding {
  id: string;                    // "F-2847"
  title: string;
  description: string;
  cveId?: string;
  severity: Severity;
  cvssV3?: number;
  cvssV4?: number;
  epssScore?: number;
  isKEV: boolean;
  status: FindingStatus;
  isDuplicate: boolean;
  duplicateFindingId?: string;

  // Hierarchy
  productId: string;
  productName: string;
  engagementId: string;
  testId: string;

  // Asset
  assetIp?: string;
  assetHostname?: string;
  componentName?: string;
  componentVersion?: string;

  // SLA
  slaExpirationDate?: string;
  slaStatus: SLAStatus;
  slaDaysLeft?: number;

  // Metadata
  createdAt: string;
  updatedAt: string;
  mitigatedAt?: string;
  createdBy: string;
  assignedTo?: string;

  // AI
  aiTriageResult?: AITriageResult;

  // VEX
  vexJustification?: string;

  // JIRA
  jiraIssueKey?: string;
  jiraUrl?: string;
}

export interface AITriageResult {
  remarks: 'Confirmed' | 'FalsePositive' | 'NotAffected' | 'Unexplored';
  confidence: number;            // 0.0 - 1.0
  justification: string;
  actions: string[];
  generatedAt: string;
}

export interface FindingAudit {
  id: string;
  findingId: string;
  action: string;
  beforeState?: Partial<Finding>;
  afterState?: Partial<Finding>;
  userId: string;
  userName: string;
  comment?: string;
  timestamp: string;
}

export interface RiskAcceptance {
  id: string;
  productId: string;
  findingIds: string[];
  expirationDate: string;
  retestDate?: string;
  reason: string;
  approvedBy: string;
  isExpired: boolean;
  createdAt: string;
}

export interface SLAConfig {
  productId?: string;            // null = global default
  criticalDays: number;          // default: 7
  highDays: number;              // default: 30
  mediumDays: number;            // default: 90
  lowDays: number;               // default: 180
}
