import type { Scan } from '@/shared/types/scan';

// ── Types ────────────────────────────────────────────────────────────────────

export interface NmapPort {
  port: number;
  proto: string;
  service: string;
  version: string;
  state: 'open' | 'closed' | 'filtered';
}

export interface NmapHost {
  ip: string;
  hostname: string;
  os: string;
  riskScore: number;
  ports: NmapPort[];
  cves: string[];
}

export interface NmapResultsResponse {
  hosts: NmapHost[];
  total: number;
  scanId: string;
}

// ── Fixture ──────────────────────────────────────────────────────────────────

export const nmapHostsFixture: NmapHost[] = [
  {
    ip: '10.0.1.45', hostname: 'webserver01.prod', os: 'Ubuntu 22.04', riskScore: 9.8,
    ports: [
      { port: 22,   proto: 'tcp', service: 'SSH',      version: 'OpenSSH 8.9p1',      state: 'open' },
      { port: 80,   proto: 'tcp', service: 'HTTP',     version: 'nginx 1.22.1',        state: 'open' },
      { port: 443,  proto: 'tcp', service: 'HTTPS',    version: 'nginx 1.22.1',        state: 'open' },
      { port: 8080, proto: 'tcp', service: 'HTTP-ALT', version: 'Apache Tomcat 9.0.58', state: 'open' },
    ],
    cves: ['CVE-2025-44228', 'CVE-2025-18935', 'CVE-2025-33127'],
  },
  {
    ip: '10.0.2.11', hostname: 'db01.prod', os: 'CentOS 8', riskScore: 8.9,
    ports: [
      { port: 22,   proto: 'tcp', service: 'SSH',   version: 'OpenSSH 7.4p1', state: 'open' },
      { port: 3306, proto: 'tcp', service: 'MySQL', version: 'MySQL 8.0.32',  state: 'open' },
    ],
    cves: ['CVE-2025-44228', 'CVE-2025-09876'],
  },
  {
    ip: '10.0.3.22', hostname: 'api-gw.prod', os: 'Debian 11', riskScore: 8.1,
    ports: [
      { port: 443,  proto: 'tcp', service: 'HTTPS',     version: 'nginx 1.22.1', state: 'open' },
      { port: 8443, proto: 'tcp', service: 'HTTPS-ALT', version: 'Kong 3.4.0',   state: 'open' },
    ],
    cves: ['CVE-2025-18935'],
  },
  {
    ip: '10.0.4.10', hostname: 'cache01.prod', os: 'Alpine Linux', riskScore: 3.2,
    ports: [
      { port: 6379, proto: 'tcp', service: 'Redis', version: 'Redis 7.0.12', state: 'open' },
    ],
    cves: [],
  },
  {
    ip: '10.0.5.55', hostname: 'k8s-master-01', os: 'Ubuntu 20.04', riskScore: 6.8,
    ports: [
      { port: 6443,  proto: 'tcp', service: 'HTTPS',   version: 'k8s API 1.28',  state: 'open' },
      { port: 2379,  proto: 'tcp', service: 'etcd',    version: 'etcd 3.5.9',    state: 'open' },
      { port: 10250, proto: 'tcp', service: 'Kubelet', version: 'Kubelet 1.28',  state: 'open' },
    ],
    cves: [],
  },
];
