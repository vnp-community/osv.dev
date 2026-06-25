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
