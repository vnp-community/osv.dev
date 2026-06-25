# TASK-UI-002 — Shared TypeScript Types

| Field | Value |
|-------|-------|
| **Task ID** | TASK-UI-002 |
| **Module** | `ui/src/shared/types/` |
| **Solution Ref** | [SOL-002 §3](../solutions/SOL-002-phase1-foundation.md#3-step-2-typescript-shared-types), [TDD.md §2-11](../../../TDD.md) |
| **Priority** | 🔴 P0 |
| **Depends On** | TASK-UI-001 |
| **Estimated** | 1h |
| **Status** | ✅ Completed |
| **Completed** | 2026-06-17 |

---

## Context

Toàn bộ features và shared components đều cần TypeScript types nhất quán. Hiện tại không có file types nào, mỗi component tự định nghĩa inline (hoặc dùng `any`). Cần tạo single source of truth cho tất cả domain types trước khi viết bất kỳ API layer hay hook nào.

---

## Goal

Tạo 5 shared type files trong `src/shared/types/` bao phủ toàn bộ domain models của OSV Platform.

---

## Target Files

| Action | File Path |
|--------|-----------|
| CREATE | `ui/src/shared/types/api.ts` |
| CREATE | `ui/src/shared/types/auth.ts` |
| CREATE | `ui/src/shared/types/cve.ts` |
| CREATE | `ui/src/shared/types/finding.ts` |
| CREATE | `ui/src/shared/types/scan.ts` |

---

## Implementation

### File 1: `ui/src/shared/types/api.ts`

```typescript
// Base API response types

export interface APIError {
  error: string;      // Machine-readable: "NOT_FOUND", "UNAUTHORIZED"
  message: string;    // Human-readable
  details?: unknown;
  traceId?: string;   // For support tracking
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
}
```

### File 2: `ui/src/shared/types/auth.ts`

```typescript
export type UserRole = 'admin' | 'user' | 'readonly' | 'agent';

export type Permission =
  | 'scan:create' | 'scan:read'
  | 'asset:write' | 'asset:read'
  | 'user:manage'
  | 'report:download'
  | 'system:configure'
  | 'finding:write' | 'finding:read'
  | 'agent:report';

export interface User {
  id: string;
  email: string;
  name: string;
  role: UserRole;
  permissions: Permission[];
  mfaEnabled: boolean;
  avatarUrl?: string;
  createdAt: string;
}

export interface AuthTokens {
  accessToken: string;    // JWT RS256, 15min TTL
  expiresIn: number;      // seconds
  // refresh_token via httpOnly cookie (server-managed)
}

export interface AuthState {
  user: User | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  setUser: (user: User) => void;
  setAccessToken: (token: string) => void;
  logout: () => void;
}
```

### File 3: `ui/src/shared/types/cve.ts`

```typescript
export type Severity = 'Critical' | 'High' | 'Medium' | 'Low' | 'Info';

export interface CVESource {
  name: string;            // "NVD", "CIRCL", "ExploitDB"
  url: string;
  lastModified: string;
}

export interface AISeverityResult {
  severity: Severity;
  confidence: number;      // 0.0 - 1.0
  reasoning: string;
  source: 'cvss_v3' | 'cvss_v2' | 'llm';
}

export interface CVE {
  id: string;              // "CVE-2025-44228"
  severity: Severity;
  cvssV3?: number;         // 0.0 - 10.0
  cvssV2?: number;
  epssScore: number;       // 0.0 - 1.0 (percentage = *100)
  epssPercentile: number;
  isKEV: boolean;
  vendor: string;
  product: string;
  cweIds: string[];
  capecIds: string[];
  description: string;
  publishedAt: string;
  updatedAt: string;
  sources: CVESource[];
  hasExploit: boolean;
  exploitDbUrl?: string;
  aiSeverity?: AISeverityResult;
  similarityScore?: number; // Cosine similarity (0-1) — semantic search only
}

export interface KEVEntry {
  cveId: string;
  vendor: string;
  product: string;
  vulnerabilityName: string;
  dateAdded: string;
  shortDescription: string;
  requiredAction: string;
  dueDate?: string;
  knownRansomwareCampaignUse: boolean;
}

export interface EPSSData {
  cveId: string;
  epssScore: number;
  epssPercentile: number;
  date: string;
}

export interface CAPECPattern {
  id: string;              // "CAPEC-66"
  name: string;
  likelihood: string;
  description: string;
}

export interface CWEDetail {
  id: string;              // "CWE-89"
  name: string;
  description: string;
  extendedDescription?: string;
  likelihood: string;
  mitigations: string[];
  capecPatterns: CAPECPattern[];
  relatedCVECount: number;
}
```

### File 4: `ui/src/shared/types/finding.ts`

```typescript
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
```

### File 5: `ui/src/shared/types/scan.ts`

```typescript
export type ScanType = 'nmap_full' | 'nmap_discovery' | 'zap' | 'agent' | 'import';
export type ScanStatus = 'pending' | 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';
export type ScanFrequency = 'once' | 'daily' | 'weekly' | 'custom';

export interface Scan {
  id: string;
  name: string;
  type: ScanType;
  status: ScanStatus;
  targets: string[];             // IPs, CIDRs, URLs
  progress: number;              // 0-100
  findingCount: number;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
  createdBy: string;
  engagementId?: string;
  error?: string;
}

export interface ScanProgress {
  scanId: string;
  status: ScanStatus;
  progress: number;              // 0-100
  currentTarget?: string;
  message?: string;
  findingsFound: number;
}

export interface ScanSummary {
  id: string;
  name: string;
  type: ScanType;
  status: ScanStatus;
  findingCount: number;
  startedAt?: string;
  completedAt?: string;
}

export interface NmapPort {
  port: number;
  protocol: 'tcp' | 'udp';
  state: 'open' | 'filtered';
  service: string;
  version?: string;
  cveIds: string[];
}

export interface NmapHost {
  ip: string;
  hostname?: string;
  os?: string;
  state: 'up' | 'down';
  ports: NmapPort[];
  cveIds: string[];
  riskScore: number;            // Max CVSS across CVEs
}

export interface ZAPAlert {
  id: string;
  name: string;
  risk: 'High' | 'Medium' | 'Low' | 'Informational';
  confidence: 'High' | 'Medium' | 'Low';
  url: string;
  description: string;
  solution: string;
  evidence?: string;
  cweId?: string;
  references: string[];
}

export interface ScheduledScan {
  id: string;
  name: string;
  type: ScanType;
  targets: string[];
  frequency: ScanFrequency;
  cronExpr?: string;
  nextRunAt: string;
  lastRunAt?: string;
  enabled: boolean;
}

// Asset type (related to scans)
export interface AssetService {
  port: number;
  protocol: string;
  service: string;
  version?: string;
  cveIds: string[];
}

export interface Asset {
  id: string;
  ip: string;
  hostname?: string;
  os?: string;
  services: AssetService[];
  webTechnologies: string[];
  tags: string[];
  riskScore: number;
  activeFindingCount: number;
  firstSeenAt: string;
  lastSeenAt: string;
  lastScanId?: string;
}
```

---

## Verification

```bash
cd ui/

# Type check — không được có lỗi
npx tsc --noEmit

# Kiểm tra files đã tạo
ls -la src/shared/types/
# Expected: api.ts  auth.ts  cve.ts  finding.ts  scan.ts
```

**Expected:** `tsc --noEmit` hoàn thành không có lỗi type.

---

## Checklist

- [x] `src/shared/types/api.ts` — `APIError`, `PaginatedResponse<T>`
- [x] `src/shared/types/auth.ts` — `User`, `UserRole`, `Permission`, `AuthState`
- [x] `src/shared/types/cve.ts` — `CVE`, `Severity`, `KEVEntry`, `CWEDetail`, `EPSSData`
- [x] `src/shared/types/finding.ts` — `Finding`, `FindingStatus`, `SLAStatus`, `AITriageResult`
- [x] `src/shared/types/scan.ts` — `Scan`, `ScanProgress`, `NmapHost`, `ZAPAlert`, `Asset`
- [x] `npx tsc --noEmit` không có lỗi (chỉ còn 1 lỗi pre-existing trong AIEnrichment.tsx)
