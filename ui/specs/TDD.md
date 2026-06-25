# Technical Design Document (TDD) — OSV Platform Frontend

**Version:** 1.0  
**Ngày tạo:** 2026-06-14  
**Trạng thái:** Draft  
**Tài liệu liên quan:** [architecture.md](./architecture.md), PRD v3.0, SRS v3.0, URD v3.0

---

## 1. Mục đích

Tài liệu TDD này mô tả chi tiết kỹ thuật cho từng module frontend của OSV Platform, bao gồm:
- Component specifications
- Data models (TypeScript types)
- API contracts (request/response)
- State management design
- UX behavior specifications

---

## 2. Module 1: Authentication (feature/auth)

### 2.1 Screens

| Screen | Route | Mô tả |
|--------|-------|-------|
| `LoginScreen` | `/login` | Username/password + OAuth2 |
| `MFASetup` | `/login/mfa-setup` | TOTP QR code setup |
| `MFAVerify` | `/login/mfa` | 6-digit TOTP input |
| `OAuthCallback` | `/auth/callback` | Google/GitHub OAuth redirect |

### 2.2 Data Types

```typescript
// shared/types/auth.ts

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
  // refresh_token via httpOnly cookie
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

### 2.3 API Contracts

```typescript
// features/auth/api/authApi.ts

// POST /api/v1/auth/login
interface LoginRequest {
  email: string;
  password: string;
  mfa_code?: string;  // 6-digit TOTP (nếu MFA enabled)
}
interface LoginResponse {
  access_token: string;
  expires_in: number;
  user: User;
  mfa_required?: boolean;  // true → redirect to MFA screen
}

// POST /api/v1/auth/refresh
// Body: empty (refresh token từ httpOnly cookie)
interface RefreshResponse {
  access_token: string;
  expires_in: number;
}

// GET /api/v1/auth/me
interface MeResponse {
  user: User;
}

// POST /api/v1/auth/logout
// Body: empty

// GET /api/v1/auth/oauth/google
// → Redirect to Google OAuth

// GET /api/v1/auth/mfa/setup
interface MFASetupResponse {
  secret: string;
  qr_url: string;   // otpauth:// URL for QR code rendering
  backup_codes: string[];
}

// POST /api/v1/auth/mfa/confirm
interface MFAConfirmRequest {
  code: string;  // 6-digit TOTP
}
```

### 2.4 Zustand Auth Store

```typescript
// features/auth/store/authStore.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isLoading: false,
      setUser: (user) => set({ user, isAuthenticated: true }),
      setAccessToken: (accessToken) => set({ accessToken }),
      logout: () => set({ user: null, accessToken: null, isAuthenticated: false }),
    }),
    {
      name: 'osv-auth',
      partialize: (state) => ({ user: state.user }), // Chỉ persist user, KHÔNG persist token
    }
  )
);
```

### 2.5 AuthGuard Component

```typescript
// app/components/AuthGuard.tsx
export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuthStore();
  const location = useLocation();

  if (isLoading) return <FullPageSpinner />;
  
  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }
  
  return <>{children}</>;
}
```

---

## 3. Module 2: Dashboard (feature/dashboard)

### 3.1 Screen Specifications

**Executive Dashboard** (`/dashboard`)
- KPI Row: Critical Findings, High Findings, Total Assets, Active Scans, Security Grade (A-F), SLA Compliance %
- Risk Trend Chart: Area chart — Critical/High/Medium over 6 months (Recharts AreaChart)
- Severity Distribution: Donut chart (Recharts PieChart)
- Product Security Grades: List with GradeCircle component
- KEV Alerts: Real-time KEV additions (last 30 days)
- Recent Scans: Last 5 scans with status
- SLA Breaches: Overdue findings
- Recent Critical Findings: Table (ID, CVE, Severity, Product, Asset, EPSS, KEV, Status, SLA)

### 3.2 Data Types

```typescript
// features/dashboard/types.ts

export interface DashboardKPIs {
  criticalFindings: number;
  highFindings: number;
  totalAssets: number;
  highRiskAssets: number;
  activeScans: number;
  queuedScans: number;
  securityGrade: 'A' | 'A-' | 'B+' | 'B' | 'B-' | 'C+' | 'C' | 'D' | 'F';
  securityScore: number;         // 0-100
  slaCompliance: number;         // percentage
  slaAtRisk: number;
  slaBreached: number;
}

export interface RiskTrendPoint {
  month: string;                 // "Jan", "Feb"...
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

export interface ProductGrade {
  id: string;
  name: string;
  grade: string;                 // A-F
  score: number;                 // 0-100
  criticalCount: number;
  highCount: number;
}

export interface KEVAlert {
  cveId: string;
  vendor: string;
  product: string;
  dateAdded: string;
  isRansomware: boolean;
}

export interface DashboardData {
  kpis: DashboardKPIs;
  riskTrend: RiskTrendPoint[];   // Last 6 months
  severityDistribution: SeverityDistribution;
  productGrades: ProductGrade[];
  kevAlerts: KEVAlert[];
  recentScans: ScanSummary[];
  slaBreaches: SLABreach[];
}
```

### 3.3 API Contracts

```typescript
// GET /api/v1/dashboard
// Query: ?period=30d|90d|1y
// Response: DashboardData

// GET /api/v2/kev?limit=5&sort=date_added_desc
// Response: { entries: KEVEntry[], total: number }
```

### 3.4 React Query Hooks

```typescript
// features/dashboard/hooks/useDashboardMetrics.ts
export function useDashboardMetrics(period: '30d' | '90d' | '1y' = '30d') {
  return useQuery({
    queryKey: ['dashboard', period],
    queryFn: () => dashboardApi.getMetrics(period),
    staleTime: 60_000,          // 1 min — dashboard refreshes frequently
    refetchInterval: 60_000,    // Auto-refresh every minute
  });
}
```

---

## 4. Module 3: CVE Intelligence (feature/cve-intel)

### 4.1 Screen Specifications

#### 4.1.1 CVE Search (`/cve/search`)

**Layout:** 3-panel (Filter Panel Left + Table Center + Detail Drawer Right)

**Left Filter Panel:**
- Severity filter: All | Critical | High | Medium | Low (radio buttons)
- KEV Status: All | KEV Only | Non-KEV
- Vendor: Multi-select (autocomplete từ `GET /api/v2/vendors`)
- CVSS Range: Slider (0.0 – 10.0)
- EPSS Range: Slider (0% – 100%)
- CWE: Multi-select từ taxonomy

**Center Table columns:** CVE ID, Severity, CVSS, EPSS (bar), KEV, Vendor, Product, CWE, Updated

**Right Detail Drawer:**
- CVE ID + copy button
- Severity + KEV badges
- Metrics: CVSS, EPSS, Vendor, CWE
- AI Security Analysis (95% confidence)
- Description
- Affected Products
- Exploitation status (In-the-Wild + PoC)
- References (NVD, CISA KEV, GitHub Advisory, Vendor Advisory)

**UX behaviors:**
- Row click → open detail drawer
- ⌘K → global search
- Search bar supports AI semantic search (prefix detected: "similar to", "like", "find...")
- URL-based deep links: `/cve/search?q=log4j&severity=critical`
- Pagination: server-side, 50 results per page
- Sort: click column header

#### 4.1.2 Semantic Search (`/cve/semantic`)

- Single input: natural language query
- Example queries shown: "buffer overflow in web server", "CVEs similar to Log4Shell"
- Results: cards with similarity score (cosine %), CVE ID, description excerpt
- Loading: streaming effect (AI processing indicator)

#### 4.1.3 KEV Catalog (`/cve/kev`)

**Stats bar:** Total KEV entries, Ransomware-linked, This week added, Unmitigated in platform

**Table columns:** CVE ID, Vendor, Product, Date Added, Date Due, Known Ransomware, Status in Platform

**Filter:** By vendor, ransomware flag, date range

#### 4.1.4 EPSS Analytics (`/cve/epss`)

**Charts:**
- EPSS Distribution histogram
- Top 10 CVEs by EPSS (bar chart)
- EPSS trend for selected CVE (line chart)

#### 4.1.5 Vendor Catalog (`/cve/vendors`)

- Paginated list of vendors sorted by CVE count
- Search by vendor name
- Click vendor → `/cve/vendors/{vendor}` → product list
- Click product → `/cve/search?vendor={v}&product={p}`

#### 4.1.6 CWE Library (`/cve/cwe`)

- Searchable CWE list
- CWE detail: description, CAPEC patterns, linked CVEs count
- Navigation: CWE → linked CVEs → CVE detail

### 4.2 Data Types

```typescript
// shared/types/cve.ts

export type Severity = 'Critical' | 'High' | 'Medium' | 'Low' | 'Info';

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
  embedding?: number[];    // Only for semantic search results
  similarityScore?: number; // Cosine similarity (0-1)
}

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

export interface EPSSData {
  cveId: string;
  epssScore: number;
  epssPercentile: number;
  date: string;
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

export interface CAPECPattern {
  id: string;              // "CAPEC-66"
  name: string;
  likelihood: string;
  description: string;
}
```

### 4.3 API Contracts

```typescript
// POST /api/v2/cves/search
interface CVESearchRequest {
  query?: string;            // Full-text search
  severity?: Severity[];
  vendors?: string[];
  products?: string[];
  cweIds?: string[];
  minCvss?: number;
  maxCvss?: number;
  minEpss?: number;
  maxEpss?: number;
  kevOnly?: boolean;
  hasExploit?: boolean;
  page?: number;             // 1-based
  pageSize?: number;         // default: 50
  sortBy?: 'cvss_desc' | 'epss_desc' | 'date_desc' | 'severity_desc';
}
interface CVESearchResponse {
  data: CVE[];
  total: number;
  page: number;
  pageSize: number;
  aggregations: {
    bySeverity: Record<Severity, number>;
    topVendors: Array<{ vendor: string; count: number }>;
    byYear: Array<{ year: number; count: number }>;
  };
}

// POST /api/v2/cves/search/semantic
interface SemanticSearchRequest {
  query: string;             // Natural language
  limit?: number;            // default: 20
  minSimilarity?: number;    // default: 0.7
}
interface SemanticSearchResponse {
  results: Array<CVE & { similarityScore: number }>;
  queryEmbeddingMs: number;  // For UX display
}

// GET /api/v2/cves/{id}
// Response: CVE (full detail with all fields)

// GET /api/v2/kev
interface KEVListRequest {
  page?: number;
  pageSize?: number;
  ransomwareOnly?: boolean;
  vendor?: string;
  dateFrom?: string;
  dateTo?: string;
}
interface KEVListResponse {
  entries: KEVEntry[];
  total: number;
  stats: {
    total: number;
    ransomwareLinked: number;
    addedThisWeek: number;
    unmitigatedInPlatform: number;
  };
}

// GET /api/v2/cwe/{id}
// Response: CWEDetail

// GET /api/v2/browse
// Response: { vendors: Array<{ name, cveCount }>, total: number }

// GET /api/v2/dbinfo
interface DBInfoResponse {
  sources: Array<{
    name: string;
    cveCount: number;
    lastSyncAt: string;
    lagMinutes: number;
  }>;
  totalCVEs: number;
}
```

---

## 5. Module 4: Active Scanning (feature/scanning)

### 5.1 Screen Specifications

#### 5.1.1 Scan Dashboard (`/scans`)

**Stats:** Active Scans, Completed Today, Total Findings, Scheduled Scans

**Tabs:** Running Scans | Recent Completed | Scheduled

**Running Scans:** Card per scan với SSE progress bar, target, type, cancel button

**Recent Completed:** Table — Name, Type, Target, Findings, Duration, Date

#### 5.1.2 Scan Wizard (`/scans/new`)

**Step 1 — Scan Type:**
- Radio: Nmap Network Scan | OWASP ZAP Web Scan | Import Report
- Type description & example use case

**Step 2 — Target:**
- Nmap: IP/CIDR input, scan profile (Discovery/Full/Custom), port range
- ZAP: URL input, max depth slider, timeout

**Step 3 — Schedule (Optional):**
- One-time (default) or recurring (Daily/Weekly/Custom cron)
- Link to engagement (Product → Engagement → Test)

**Step 4 — Review & Launch**

#### 5.1.3 Running Scan (`/scans/:id`)

- Scan metadata: ID, type, target, started by, elapsed time
- Real-time progress bar (SSE: 0-100%)
- Live log feed (SSE events)
- Cancel button (Admin/User only)
- Findings found so far (auto-update)

#### 5.1.4 Nmap Results (`/scans/:id/results/nmap`)

**Host table:** IP, Hostname, OS, Open Ports, CVEs found, Risk Score

**Host detail drawer:**
- Open ports list (port, protocol, service, version)
- CVEs per service (click → CVE detail)
- Severity distribution

#### 5.1.5 ZAP Results (`/scans/:id/results/zap`)

**Alert table:** Alert Name, Risk, Confidence, URL, CWE, Evidence

**Alert detail:** Description, Solution, References, Evidence snippet

### 5.2 Data Types

```typescript
// shared/types/scan.ts

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

export interface NmapHost {
  ip: string;
  hostname?: string;
  os?: string;
  state: 'up' | 'down';
  ports: NmapPort[];
  cveIds: string[];
  riskScore: number;            // Max CVSS across CVEs
}

export interface NmapPort {
  port: number;
  protocol: 'tcp' | 'udp';
  state: 'open' | 'filtered';
  service: string;
  version?: string;
  cveIds: string[];
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
  cronExpr?: string;             // Custom cron expression
  nextRunAt: string;
  lastRunAt?: string;
  enabled: boolean;
}
```

### 5.3 API Contracts

```typescript
// GET /api/v1/scans?status=running,completed&page=1&pageSize=20
// Response: { scans: Scan[], total: number }

// POST /api/v1/scans
interface CreateScanRequest {
  name: string;
  type: ScanType;
  targets: string[];
  options?: {
    scanProfile?: 'discovery' | 'full' | 'custom';
    portRange?: string;         // "1-65535"
    maxDepth?: number;          // ZAP spider depth
    timeout?: number;           // ZAP timeout seconds
  };
  engagementId?: string;
  scheduleFrequency?: ScanFrequency;
  scheduleCronExpr?: string;
}
// Response: Scan

// GET /api/v1/scans/:id/stream  → SSE
// event: message → data: ScanProgress (JSON)
// event: done → scan finished

// POST /api/v1/scans/:id/cancel
// Response: { success: true }

// GET /api/v1/scans/:id/results/nmap
// Response: { hosts: NmapHost[], scanId: string }

// GET /api/v1/scans/:id/results/zap
// Response: { alerts: ZAPAlert[], scanId: string }
```

### 5.4 SSE Implementation

```typescript
// features/scanning/hooks/useScanSSE.ts
export function useScanSSE(scanId: string, enabled: boolean) {
  const queryClient = useQueryClient();
  const [progress, setProgress] = useState<ScanProgress | null>(null);

  const { status } = useSSE(
    `/api/v1/scans/${scanId}/stream`,
    enabled,
    {
      onMessage: (data: ScanProgress) => {
        setProgress(data);
        // Update scan in cache
        queryClient.setQueryData(scanKeys.detail(scanId), (old: Scan | undefined) =>
          old ? { ...old, progress: data.progress, status: data.status } : old
        );
      },
      onDone: () => {
        // Invalidate to fetch final state
        queryClient.invalidateQueries({ queryKey: scanKeys.detail(scanId) });
        queryClient.invalidateQueries({ queryKey: scanKeys.all });
      },
    }
  );

  return { progress, sseStatus: status };
}
```

---

## 6. Module 5: Finding Management (feature/findings)

### 6.1 Screen Specifications

#### 6.1.1 Findings List (`/findings`)

**Filter tabs:** All | Active | Mitigated | False Positive | Risk Accepted | Out of Scope | Duplicates

**Advanced filters:**
- Severity: Multi-select (Critical/High/Medium/Low)
- Product: Multi-select
- Engagement: Select
- CVE ID: Free text
- SLA Status: All | Breached | At Risk | OK
- Date range: Published, Last updated
- Assignee: User select

**Table columns:** ID, CVE, Title, Severity, EPSS, KEV, Product, Asset, Status, SLA, Assignee, Actions

**Bulk operations bar** (when rows selected):
- Close | Reopen | Tag | Assign | Export

**Pagination:** Server-side, 50 per page

#### 6.1.2 Finding Detail (`/findings/:id`)

**Left panel:**
- Finding metadata (ID, CVE, Severity, Status, SLA, Duplicate flag)
- Status actions: Mark as False Positive | Accept Risk | Mark Mitigated | Reopen
- AI Triage recommendation (Confirmed/FalsePositive/NotAffected)
- EPSS + KEV + CVSS metrics
- Affected product/component/version

**Center panel:**
- Description
- Proof of Concept / Evidence
- Remediation guidance (AI-assisted)
- VEX justification (if accepted/false-positive)

**Right panel:**
- Audit trail (full history of status changes)
- Comments thread
- JIRA link (if integrated)

#### 6.1.3 SLA Dashboard (`/dashboard/sla`)

- SLA Compliance gauge (%)
- Breached findings table (overdue by severity)
- At-risk findings (< 24h to breach)
- SLA trend chart (compliance % over time)
- Per-product SLA breakdown

#### 6.1.4 Risk Acceptance Center (`/findings/risk-acceptance`)

- List of risk acceptances with expiry status
- Create risk acceptance form (findings, expiry date, justification, retest date)
- Expired acceptances: findings auto-reopened notification

### 6.2 Data Types

```typescript
// shared/types/finding.ts

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

### 6.3 API Contracts

```typescript
// GET /api/v1/findings
interface FindingsListRequest {
  status?: FindingStatus[];
  severity?: Severity[];
  productId?: string;
  engagementId?: string;
  cveId?: string;
  slaStatus?: SLAStatus;
  assignedTo?: string;
  dateFrom?: string;
  dateTo?: string;
  page?: number;
  pageSize?: number;
  sortBy?: 'severity_desc' | 'sla_asc' | 'created_desc' | 'epss_desc';
}
interface FindingsListResponse {
  findings: Finding[];
  total: number;
  bySeverity: Record<Severity, number>;
  byStatus: Record<FindingStatus, number>;
  slaStats: { breached: number; atRisk: number; ok: number };
}

// PATCH /api/v1/findings/:id
interface UpdateFindingRequest {
  status?: FindingStatus;
  comment?: string;
  assignedTo?: string;
  vexJustification?: string;
}
// Response: Finding (updated)

// POST /api/v1/findings/bulk/close
interface BulkCloseRequest {
  findingIds: string[];
  comment?: string;
}

// GET /api/v1/findings/:id/audit
// Response: { audits: FindingAudit[] }

// POST /api/v1/ai/triage/:findingId
// Response: AITriageResult

// GET /api/v1/risk-acceptances?productId=xxx
// Response: { acceptances: RiskAcceptance[] }

// POST /api/v1/risk-acceptances
// Body: Omit<RiskAcceptance, 'id' | 'isExpired' | 'createdAt'>
```

### 6.4 Finding Status State Machine (UI)

```typescript
// shared/utils/findingStateMachine.ts

export const VALID_TRANSITIONS: Record<FindingStatus, FindingStatus[]> = {
  active: ['mitigated', 'false_positive', 'risk_accepted', 'out_of_scope'],
  mitigated: ['active'],
  false_positive: ['active'],
  risk_accepted: ['active'],
  out_of_scope: ['active'],
  duplicate: [],               // No manual transitions
};

export function canTransition(from: FindingStatus, to: FindingStatus): boolean {
  return VALID_TRANSITIONS[from].includes(to);
}

// Status priority for sorting
export const STATUS_PRIORITY: Record<FindingStatus, number> = {
  duplicate: 6,
  false_positive: 5,
  out_of_scope: 4,
  risk_accepted: 3,
  mitigated: 2,
  active: 1,
};
```

---

## 7. Module 6: Asset Management (feature/assets)

### 7.1 Screen Specifications

#### 7.1.1 Asset Inventory (`/assets`)

**Filter:** By tag, OS type, risk score range, last seen

**Table columns:** IP, Hostname, OS, Open Ports, Web Tech, Risk Score, Tags, Active Findings, Last Scanned

**Risk Score:** color-coded (Red ≥8, Orange ≥5, Yellow ≥3, Green <3)

#### 7.1.2 Asset Detail (`/assets/:id`)

**Tabs:** Overview | Open Ports | Active Findings | Scan History | Tags

**Overview:** IP, Hostname, OS, web tech, risk score gauge, first/last seen

**Open Ports:** Port table with service, version, CVEs

**Active Findings:** Findings linked to this asset (reuse FindingsList component)

**Scan History:** List of scans targeting this asset

### 7.2 Data Types

```typescript
// shared/types/asset.ts (extends scan.ts)

export interface Asset {
  id: string;
  ip: string;
  hostname?: string;
  os?: string;
  services: AssetService[];
  webTechnologies: string[];
  tags: string[];
  riskScore: number;           // Max CVSS from active findings
  activeFindingCount: number;
  firstSeenAt: string;
  lastSeenAt: string;
  lastScanId?: string;
}

export interface AssetService {
  port: number;
  protocol: string;
  service: string;
  version?: string;
  cveIds: string[];
}
```

---

## 8. Module 7: Product Security (feature/product-security)

### 8.1 Data Types

```typescript
// features/product-security/types.ts

export type ProductType = 'web_app' | 'api' | 'infrastructure' | 'mobile';
export type BusinessCriticality = 'critical' | 'high' | 'medium' | 'low';
export type LifecycleStatus = 'production' | 'staging' | 'development' | 'deprecated';
export type ProductGrade = 'A' | 'B' | 'C' | 'D' | 'F';
export type EngagementType = 'interactive' | 'cicd';

export interface Product {
  id: string;
  name: string;
  description?: string;
  type: ProductType;
  criticality: BusinessCriticality;
  lifecycle: LifecycleStatus;
  grade: ProductGrade;
  score: number;               // 0-100
  findingSummary: {
    critical: number;
    high: number;
    medium: number;
    low: number;
  };
  slaConfig?: SLAConfig;
  tags: string[];
  createdAt: string;
}

export interface Engagement {
  id: string;
  productId: string;
  name: string;
  type: EngagementType;
  startDate: string;
  endDate?: string;
  status: 'not_started' | 'in_progress' | 'completed';
  leadId?: string;
  cicdUrl?: string;
}

export interface Test {
  id: string;
  engagementId: string;
  title: string;
  scanType?: ScanType;
  testDate: string;
  findingCount: number;
}
```

### 8.2 Product Grade Logic (UI)

```typescript
// shared/utils/productGrade.ts

export function calculateProductGrade(findings: {
  critical: number;
  high: number;
  total: number;
}): ProductGrade {
  const { critical, high, total } = findings;
  if (critical >= 3 || total > 20) return 'F';
  if (critical >= 1 && critical <= 2) return 'D';
  if (critical === 0 && high > 5) return 'C';
  if (critical === 0 && high <= 5) return 'B';
  if (critical === 0 && high === 0) return 'A';
  return 'F';
}
```

---

## 9. Module 8: AI Center (feature/ai-center)

### 9.1 Screen Specifications

#### 9.1.1 AI Triage Queue (`/ai/triage`)

**Filter:** By status (Pending | Confirmed | FalsePositive | NotAffected)

**Table:** Finding ID, CVE, Severity, AI Remarks, AI Confidence, Human Decision, Actions

**Inline actions:** Accept AI suggestion | Override | Reject

**Stats:** Pending triage, Confirmed %, False Positive rate, Time saved

#### 9.1.2 AI Enrichment (`/ai/enrichment`)

**Stats:** CVEs with embeddings, Semantic search accuracy, Last enrichment run

**CVE Embedding Table:** CVE ID, Embedding dims, Cached, AI Severity (vs CVSS), Provider used

**Actions:** Re-enrich selected CVEs, Trigger enrichment job

### 9.2 Data Types

```typescript
// features/ai-center/types.ts

export interface AITriageQueueItem {
  findingId: string;
  findingTitle: string;
  cveId?: string;
  severity: Severity;
  aiResult: AITriageResult;
  humanDecision?: 'accepted' | 'overridden' | 'rejected';
  humanNote?: string;
  reviewedBy?: string;
  reviewedAt?: string;
}

export interface CVEEnrichmentStatus {
  cveId: string;
  hasEmbedding: boolean;
  embeddingDims?: number;
  isCached: boolean;
  aiSeverity?: Severity;
  aiProvider?: 'ollama' | 'openai' | 'azure';
  enrichedAt?: string;
}
```

---

## 10. Module 9: Reporting (feature/reports)

### 10.1 Screen Specifications

#### ReportCenter (`/reports`)

**Tabs:** Executive | Technical | Compliance

**Report Cards:** Format (PDF/HTML/CSV/Excel/JSON), Generated Date, Status, Actions (Download, Regenerate, Delete)

**Generate Report dialog:**
- Select product(s)/engagement
- Format: PDF | HTML | CSV | Excel | JSON
- Min severity threshold
- Min CVSS score
- Date range

### 10.2 Data Types

```typescript
// features/reports/types.ts

export type ReportFormat = 'html' | 'pdf' | 'csv' | 'excel' | 'json';
export type ReportStatus = 'pending' | 'generating' | 'completed' | 'failed';

export interface ReportRun {
  id: string;
  productId?: string;
  engagementId?: string;
  format: ReportFormat;
  status: ReportStatus;
  exitCode?: 0 | 1;          // CI/CD: 0=clean, 1=findings above threshold
  minSeverity?: Severity;
  minScore?: number;
  findingCount?: number;
  generatedAt?: string;
  artifactUrl?: string;      // S3/MinIO download URL
  expiresAt?: string;
  createdAt: string;
  createdBy: string;
}
```

### 10.3 API Contracts

```typescript
// POST /api/v1/reports
interface CreateReportRequest {
  productId?: string;
  engagementId?: string;
  format: ReportFormat;
  minSeverity?: Severity;
  minScore?: number;
  dateFrom?: string;
  dateTo?: string;
}
// Response: ReportRun (status: pending)

// GET /api/v1/reports/:id/download/:format
// Response: Binary file with Content-Disposition: attachment
```

---

## 11. Module 10: Administration (feature/admin)

### 11.1 Screen Specifications

#### 11.1.1 User Management (`/admin/users`)

**Table:** Name, Email, Role, Status, Last Login, MFA, Created, Actions

**Actions:** Edit role, Deactivate, Reset password, View audit trail

**Invite user dialog:** Email + role assignment

#### 11.1.2 RBAC Management (`/admin/roles`)

**Permission matrix table:**
- Rows: Permissions (scan:create, finding:write, ...)
- Columns: Roles (admin, user, readonly, agent)
- Read-only display of current permission matrix

#### 11.1.3 Audit Logs (`/admin/audit`)

**Filter:** By user, action type, entity type, date range

**Table:** Timestamp, User, Action, Entity Type, Entity ID, IP Address, Result

**Export:** CSV download

#### 11.1.4 System Health (`/admin/health`)

**Service grid:** Each microservice — status (healthy/degraded/down), response time, last check

**Metrics:** NATS JetStream lag, PostgreSQL connections, Redis memory, OpenSearch status

#### 11.1.5 System Settings (`/admin/settings`)

**Tabs:** General | Security | AI | Notifications | Integrations

### 11.2 Data Types

```typescript
// features/admin/types.ts

export interface AdminUser {
  id: string;
  email: string;
  name: string;
  role: UserRole;
  isActive: boolean;
  mfaEnabled: boolean;
  lastLoginAt?: string;
  createdAt: string;
  loginAttempts: number;
  isLocked: boolean;
}

export interface AuditEvent {
  id: string;
  userId: string;
  userName: string;
  action: string;
  entityType: string;
  entityId: string;
  ipAddress: string;
  userAgent?: string;
  result: 'success' | 'failure';
  metadata?: Record<string, unknown>;
  timestamp: string;
}

export interface ServiceHealth {
  name: string;
  status: 'healthy' | 'degraded' | 'down';
  responseTimeMs?: number;
  lastCheckedAt: string;
  version?: string;
  details?: string;
}

export interface SystemHealthResponse {
  services: ServiceHealth[];
  nats: {
    status: string;
    pendingMessages: number;
    consumerLag: number;
  };
  postgres: {
    status: string;
    activeConnections: number;
    maxConnections: number;
  };
  redis: {
    status: string;
    usedMemoryMb: number;
    maxMemoryMb: number;
  };
  opensearch: {
    status: string;
    indexedDocs: number;
  };
}
```

---

## 12. Shared Components Technical Spec

### 12.1 DataTable Component

```typescript
// shared/components/data-display/DataTable.tsx

interface Column<T> {
  key: keyof T;
  header: string;
  width?: string;
  sortable?: boolean;
  render?: (value: T[keyof T], row: T) => React.ReactNode;
}

interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onSort?: (key: string, direction: 'asc' | 'desc') => void;
  onRowClick?: (row: T) => void;
  selectedRows?: Set<string>;
  onSelectRow?: (id: string) => void;
  onSelectAll?: () => void;
  getRowId: (row: T) => string;
  isLoading?: boolean;
  emptyState?: React.ReactNode;
}
```

### 12.2 SeverityBadge Component

```typescript
// shared/components/data-display/SeverityBadge.tsx

const SEVERITY_CONFIG = {
  Critical: { color: '#EF4444', bg: 'rgba(239,68,68,0.15)' },
  High:     { color: '#F97316', bg: 'rgba(249,115,22,0.15)' },
  Medium:   { color: '#EAB308', bg: 'rgba(234,179,8,0.15)' },
  Low:      { color: '#3B82F6', bg: 'rgba(59,130,246,0.15)' },
  Info:     { color: '#6B7280', bg: 'rgba(107,114,128,0.15)' },
};

export function SeverityBadge({ severity }: { severity: Severity }) {
  const config = SEVERITY_CONFIG[severity];
  return (
    <span
      style={{
        background: config.bg,
        color: config.color,
        padding: '2px 8px',
        borderRadius: 6,
        fontSize: 11,
        fontWeight: 600,
      }}
    >
      {severity}
    </span>
  );
}
```

### 12.3 Command Palette (⌘K)

```typescript
// shared/components/global/CommandPalette.tsx

// Search categories:
// - CVEs: search /api/v2/cves/search?q=...
// - Findings: search /api/v1/findings?q=...
// - Assets: search /api/v1/assets?q=...
// - Actions: Navigate to screens

// UX:
// - Open: ⌘K (Mac) / Ctrl+K (Windows)
// - Grouped results by category
// - Keyboard navigation (↑↓ Enter)
// - Recent searches (localStorage)
// - Debounce: 300ms
```

---

## 13. Error Handling Strategy

### 13.1 API Error Response Types

```typescript
// shared/types/api.ts

export interface APIError {
  error: string;         // Machine-readable: "NOT_FOUND", "UNAUTHORIZED"
  message: string;       // Human-readable
  details?: unknown;
  traceId?: string;      // For support
}

// HTTP Status mapping
// 400 → Validation error → Show form errors
// 401 → Unauthorized → Trigger refresh or redirect to login
// 403 → Forbidden → Show "permission denied" message
// 404 → Not found → Show empty state
// 409 → Conflict → Show specific conflict message
// 429 → Rate limit → Show "too many requests" with retry-after
// 500 → Server error → Show generic error + trace ID
```

### 13.2 Error Boundary Strategy

```typescript
// App-level: catch critical rendering errors
// Feature-level: catch and show feature-specific fallback
// Query-level: React Query error states with retry

// Toast notifications for mutation errors
// Inline error states for query errors
// Full-page error for catastrophic failures
```

---

## 14. Performance Targets (NFR Mapping)

| NFR ID | Requirement | Implementation |
|--------|-------------|----------------|
| NFR-U-04 | LCP < 2s | Code splitting, lazy routes, preload critical CSS |
| NFR-U-05 | SSE latency < 2s | EventSource với keep-alive, gateway pass-through |
| NFR-01 | API P95 < 100ms | React Query cache, optimistic updates |
| NFR-02 | Search < 500ms | Debounce 300ms, loading skeleton |
| NFR-U-08 | API Key < 1ms | Cached in Zustand, no re-fetch on mount |

---

## 15. Internationalization (i18n)

Giai đoạn đầu: **English only**  
Chuẩn bị: sử dụng `react-i18next` structure, tất cả strings trong `en.json`

```json
{
  "common": {
    "loading": "Loading...",
    "error": "An error occurred",
    "retry": "Retry",
    "search": "Search",
    "filter": "Filter"
  },
  "severity": {
    "critical": "Critical",
    "high": "High",
    "medium": "Medium",
    "low": "Low"
  }
}
```

---

## 16. Accessibility (a11y)

- Sử dụng Radix UI primitives (shadcn/ui) → ARIA attributes built-in
- Keyboard navigation: Tab order, focus visible
- Screen reader: `aria-label` trên icon-only buttons
- Color contrast: Minimum WCAG AA (4.5:1) cho text
- Severity colors: Kết hợp màu + icon + text (không chỉ dựa vào màu)
- Motion: Tôn trọng `prefers-reduced-motion`

---

## 17. Changelog

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-06-14 | Initial TDD — based on PRD v3.0, SRS v3.0, existing frontend code analysis |
