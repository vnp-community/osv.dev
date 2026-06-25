/**
 * Fixture dữ liệu cho AI Triage Queue.
 * Dùng trong ai.handlers.ts và các unit tests.
 *
 * ⚠️ Chỉ dùng trong src/mocks/
 */

export interface AITriageFixture {
  finding_id: string;
  finding_title: string;
  cve_id?: string;
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  ai_result: {
    remarks: 'Confirmed' | 'FalsePositive' | 'NotAffected' | 'Unexplored';
    confidence: number;
    justification: string;
    actions: string[];
    generated_at: string;
    ai_provider: string;
  };
  human_decision: 'accepted' | 'overridden' | 'rejected' | null;
  human_note: string | null;
  reviewed_by: string | null;
  reviewed_at: string | null;
}

export const aiTriageQueueFixture: AITriageFixture[] = [
  {
    finding_id: 'F-2847',
    finding_title: 'Apache Log4j2 JNDI Remote Code Execution',
    cve_id: 'CVE-2025-44228',
    severity: 'Critical',
    ai_result: {
      remarks: 'Confirmed',
      confidence: 0.98,
      justification: 'CVE-2025-44228 has CVSS 10.0, is in CISA KEV, and actively exploited. Component log4j-core 2.14.1 is within affected version range (<= 2.14.1).',
      actions: [
        'Update log4j-core to 2.15.0 or later immediately',
        'Apply WAF rule to block JNDI lookup patterns',
        'Audit all log4j usages across codebase',
      ],
      generated_at: new Date(Date.now() - 30 * 60_000).toISOString(),
      ai_provider: 'ollama',
    },
    human_decision: null,
    human_note: null,
    reviewed_by: null,
    reviewed_at: null,
  },
  {
    finding_id: 'F-2843',
    finding_title: 'Spring Framework Expression DoS',
    cve_id: 'CVE-2024-38819',
    severity: 'Medium',
    ai_result: {
      remarks: 'FalsePositive',
      confidence: 0.87,
      justification: 'Target system uses Spring Boot 3.2.6 which includes the security patch. The affected class SpelExpressionParser is not exposed in this configuration.',
      actions: [
        'Verify Spring Boot version >= 3.2.6',
        'Mark as false positive if confirmed',
      ],
      generated_at: new Date(Date.now() - 2 * 3_600_000).toISOString(),
      ai_provider: 'ollama',
    },
    human_decision: 'accepted',
    human_note: 'Confirmed false positive — Spring Boot version validated',
    reviewed_by: 'bob.chen@company.com',
    reviewed_at: new Date(Date.now() - 3_600_000).toISOString(),
  },
  {
    finding_id: 'F-2840',
    finding_title: 'Cisco IOS XE Web UI Privilege Escalation',
    cve_id: 'CVE-2023-20198',
    severity: 'Critical',
    ai_result: {
      remarks: 'Confirmed',
      confidence: 0.92,
      justification: 'Device running IOS XE 17.09.01a is within vulnerable range. Web UI is exposed on management interface.',
      actions: [
        'Apply Cisco security advisory patch immediately',
        'Disable HTTP Server if not required: no ip http server',
        'Restrict access to management interface via ACL',
      ],
      generated_at: new Date(Date.now() - 5 * 3_600_000).toISOString(),
      ai_provider: 'ollama',
    },
    human_decision: null,
    human_note: null,
    reviewed_by: null,
    reviewed_at: null,
  },
];
