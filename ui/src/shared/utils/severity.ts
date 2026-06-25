import type { Severity } from '@/shared/types/cve';

export const SEVERITY_COLORS: Record<Severity, string> = {
  Critical: '#EF4444',
  High: '#F97316',
  Medium: '#EAB308',
  Low: '#3B82F6',
  Info: '#6B7280',
};

export const SEVERITY_BG_COLORS: Record<Severity, string> = {
  Critical: 'rgba(239,68,68,0.15)',
  High: 'rgba(249,115,22,0.15)',
  Medium: 'rgba(234,179,8,0.15)',
  Low: 'rgba(59,130,246,0.15)',
  Info: 'rgba(107,114,128,0.15)',
};

export const SEVERITY_BORDER_COLORS: Record<Severity, string> = {
  Critical: 'rgba(239,68,68,0.4)',
  High: 'rgba(249,115,22,0.4)',
  Medium: 'rgba(234,179,8,0.4)',
  Low: 'rgba(59,130,246,0.4)',
  Info: 'rgba(107,114,128,0.4)',
};

export const SEVERITY_ORDER: Record<Severity, number> = {
  Critical: 0, High: 1, Medium: 2, Low: 3, Info: 4,
};

export function getSeverityColor(severity: Severity): string {
  return SEVERITY_COLORS[severity] ?? '#6B7280';
}

export function getSeverityBgColor(severity: Severity): string {
  return SEVERITY_BG_COLORS[severity] ?? 'rgba(107,114,128,0.15)';
}

export function getCVSSColor(cvss?: number): string {
  if (!cvss) return '#6B7280';
  if (cvss >= 9.0) return '#EF4444';
  if (cvss >= 7.0) return '#F97316';
  if (cvss >= 4.0) return '#EAB308';
  return '#3B82F6';
}

export function sortBySeverity<T extends { severity: Severity }>(items: T[]): T[] {
  return [...items].sort(
    (a, b) => SEVERITY_ORDER[a.severity] - SEVERITY_ORDER[b.severity]
  );
}
