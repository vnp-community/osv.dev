// ── Types ────────────────────────────────────────────────────────────────────

export interface CapecPattern {
  id: string;
  name: string;
  mechanism: string;
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  likelihood: 'High' | 'Medium' | 'Low';
  cve_count: number;
  mitre_url: string;
}

export interface CapecListResponse {
  items: CapecPattern[];
  total: number;
}

// ── Fixture ──────────────────────────────────────────────────────────────────

export const capecPatternsFixture: CapecPattern[] = [
  { id: 'CAPEC-1',   name: 'Accessing/Intercepting/Modifying HTTP Cookies',   mechanism: 'Exploitation of Authentication',      severity: 'High',     likelihood: 'High',   cve_count: 312,  mitre_url: 'https://capec.mitre.org/data/definitions/1.html' },
  { id: 'CAPEC-2',   name: 'Inducing Account Lockout',                        mechanism: 'Exploitation of Authentication',      severity: 'Medium',   likelihood: 'Medium', cve_count: 48,   mitre_url: 'https://capec.mitre.org/data/definitions/2.html' },
  { id: 'CAPEC-7',   name: 'Blind SQL Injection',                             mechanism: 'Injection',                           severity: 'High',     likelihood: 'High',   cve_count: 891,  mitre_url: 'https://capec.mitre.org/data/definitions/7.html' },
  { id: 'CAPEC-17',  name: 'Using Malicious Files',                           mechanism: 'Malicious Code Injection',            severity: 'High',     likelihood: 'Medium', cve_count: 523,  mitre_url: 'https://capec.mitre.org/data/definitions/17.html' },
  { id: 'CAPEC-19',  name: 'Embedding Scripts within Scripts',                mechanism: 'Injection',                           severity: 'High',     likelihood: 'Medium', cve_count: 267,  mitre_url: 'https://capec.mitre.org/data/definitions/19.html' },
  { id: 'CAPEC-22',  name: 'Exploiting Trust in Client',                      mechanism: 'Exploitation of Trusted Credentials', severity: 'High',     likelihood: 'High',   cve_count: 445,  mitre_url: 'https://capec.mitre.org/data/definitions/22.html' },
  { id: 'CAPEC-33',  name: 'Remote Code Inclusion',                           mechanism: 'Injection',                           severity: 'Critical', likelihood: 'Medium', cve_count: 678,  mitre_url: 'https://capec.mitre.org/data/definitions/33.html' },
  { id: 'CAPEC-45',  name: 'Buffer Overflow via Environment Variables',       mechanism: 'Memory/Buffer Abuse',                 severity: 'High',     likelihood: 'Low',    cve_count: 134,  mitre_url: 'https://capec.mitre.org/data/definitions/45.html' },
  { id: 'CAPEC-55',  name: 'Rainbow Table Password Cracking',                 mechanism: 'Exploitation of Authentication',      severity: 'Medium',   likelihood: 'High',   cve_count: 89,   mitre_url: 'https://capec.mitre.org/data/definitions/55.html' },
  { id: 'CAPEC-62',  name: 'Cross-Site Request Forgery (CSRF)',               mechanism: 'Exploitation of Trusted Credentials', severity: 'High',     likelihood: 'High',   cve_count: 1203, mitre_url: 'https://capec.mitre.org/data/definitions/62.html' },
  { id: 'CAPEC-66',  name: 'SQL Injection',                                   mechanism: 'Injection',                           severity: 'Critical', likelihood: 'High',   cve_count: 4521, mitre_url: 'https://capec.mitre.org/data/definitions/66.html' },
  { id: 'CAPEC-86',  name: 'XSS Through HTTP Query Strings',                  mechanism: 'Injection',                           severity: 'High',     likelihood: 'High',   cve_count: 2134, mitre_url: 'https://capec.mitre.org/data/definitions/86.html' },
  { id: 'CAPEC-97',  name: 'Cryptanalysis of Cellular Phone Communication',   mechanism: 'Communication Channel Manipulation',  severity: 'High',     likelihood: 'Low',    cve_count: 23,   mitre_url: 'https://capec.mitre.org/data/definitions/97.html' },
  { id: 'CAPEC-116', name: 'Excavation',                                      mechanism: 'Collect and Analyze Information',     severity: 'Low',      likelihood: 'High',   cve_count: 56,   mitre_url: 'https://capec.mitre.org/data/definitions/116.html' },
  { id: 'CAPEC-122', name: 'Privilege Abuse',                                 mechanism: 'Exploitation of Privilege',           severity: 'Medium',   likelihood: 'High',   cve_count: 387,  mitre_url: 'https://capec.mitre.org/data/definitions/122.html' },
  { id: 'CAPEC-186', name: 'Malicious Software Update',                       mechanism: 'Supply Chain',                        severity: 'Critical', likelihood: 'Low',    cve_count: 67,   mitre_url: 'https://capec.mitre.org/data/definitions/186.html' },
  { id: 'CAPEC-196', name: 'Session Fixation',                                mechanism: 'Exploitation of Authentication',      severity: 'High',     likelihood: 'Medium', cve_count: 213,  mitre_url: 'https://capec.mitre.org/data/definitions/196.html' },
  { id: 'CAPEC-209', name: 'XSS Using MIME Type Mismatch',                    mechanism: 'Injection',                           severity: 'Medium',   likelihood: 'Medium', cve_count: 98,   mitre_url: 'https://capec.mitre.org/data/definitions/209.html' },
  { id: 'CAPEC-234', name: 'Hijacking Privileged Thread of Execution',        mechanism: 'Exploitation of Privilege',           severity: 'Critical', likelihood: 'Low',    cve_count: 44,   mitre_url: 'https://capec.mitre.org/data/definitions/234.html' },
  { id: 'CAPEC-248', name: 'Command Injection',                               mechanism: 'Injection',                           severity: 'Critical', likelihood: 'High',   cve_count: 2876, mitre_url: 'https://capec.mitre.org/data/definitions/248.html' },
];
