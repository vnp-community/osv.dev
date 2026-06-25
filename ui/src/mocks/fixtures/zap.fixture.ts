// ── Types ────────────────────────────────────────────────────────────────────

export interface ZapAlert {
  id: string;
  name: string;
  risk: 'Critical' | 'High' | 'Medium' | 'Low';
  confidence: 'High' | 'Medium' | 'Low';
  count: number;
  url: string;
  method: string;
  param: string | null;
  evidence: string;
  desc: string;
  solution: string;
}

export interface ZapResultsResponse {
  alerts: ZapAlert[];
  total: number;
  scanId: string;
  riskBreakdown: Record<string, number>;
}

// ── Fixture ──────────────────────────────────────────────────────────────────

export const zapAlertsFixture: ZapAlert[] = [
  { id: 'A-01', name: 'Cross-Site Scripting (XSS)',                 risk: 'High',     confidence: 'Medium', count: 14, url: '/api/v1/search',       method: 'GET',  param: 'q',          evidence: '<script>alert(1)</script>',    desc: 'Cross-site Scripting (XSS) - Reflected. Attacker-supplied code is executed in the victim browser.', solution: 'Phase: Architecture and Design. Use a vetted library or framework that does not allow this weakness to occur.' },
  { id: 'A-02', name: 'SQL Injection',                              risk: 'Critical', confidence: 'High',   count: 3,  url: '/api/v1/users',        method: 'POST', param: 'email',       evidence: "' OR 1=1--",                  desc: 'SQL injection may be possible. The page results were manipulated using a boolean-based injection.', solution: 'Use parameterized queries, prepared statements. Apply input validation and least-privilege database accounts.' },
  { id: 'A-03', name: 'Broken Authentication — Missing CSRF Token', risk: 'Medium',   confidence: 'Low',    count: 8,  url: '/api/v1/account/update', method: 'POST', param: 'csrf_token', evidence: 'No CSRF token found in form', desc: 'No Anti-CSRF tokens were found in a HTML submission form. A CSRF attack forces a logged-on victim\'s browser to send a forged HTTP request.', solution: 'Phase: Architecture and Design. Use a vetted library or framework that does not allow this weakness. Use anti-CSRF tokens.' },
  { id: 'A-04', name: 'Sensitive Data Exposure — API Keys in Response', risk: 'High', confidence: 'High',  count: 2,  url: '/api/v1/config',       method: 'GET',  param: null,          evidence: '"api_key": "sk-prod-xxxx..."', desc: 'The response contains a potentially sensitive API key or credential that should not be returned to the client.', solution: 'Remove sensitive data from API responses. Use environment variables and secrets management.' },
  { id: 'A-05', name: 'Missing Security Headers',                   risk: 'Low',      confidence: 'High',   count: 1,  url: '/',                    method: 'GET',  param: null,          evidence: 'X-Frame-Options header not set', desc: 'The response does not include a X-Frame-Options header, meaning it can be embedded in frames.', solution: 'Ensure that your web server, application server, load balancer, etc. is configured to set these headers.' },
  { id: 'A-06', name: 'Directory Traversal',                        risk: 'High',     confidence: 'Medium', count: 5,  url: '/api/v1/files',        method: 'GET',  param: 'path',        evidence: '../../../etc/passwd',          desc: 'Path traversal is possible via the \'path\' parameter.', solution: 'Assume all input is malicious. Validate and canonicalize paths. Use a chroot jail or equivalent.' },
];
