import type { SLAStatus } from '@/shared/types/finding';
import type { Severity } from '@/shared/types/cve';

export const SLA_DAYS_BY_SEVERITY: Record<Exclude<Severity, 'Info'>, number> = {
  Critical: 7,
  High: 30,
  Medium: 90,
  Low: 180,
};

export const SLA_STATUS_COLORS: Record<SLAStatus, string> = {
  ok: '#10B981',
  at_risk: '#F97316',
  breached: '#EF4444',
};

export function getSLAStatus(expirationDate: string): SLAStatus {
  const daysLeft = getSLADaysLeft(expirationDate);
  if (daysLeft < 0) return 'breached';
  if (daysLeft <= 3) return 'at_risk';
  return 'ok';
}

export function getSLADaysLeft(expirationDate: string): number {
  const now = new Date();
  const exp = new Date(expirationDate);
  return Math.floor((exp.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
}

export function formatSLALabel(daysLeft: number): string {
  if (daysLeft < 0) return `Overdue ${Math.abs(daysLeft)}d`;
  if (daysLeft === 0) return 'Due today';
  if (daysLeft === 1) return '1 day left';
  return `${daysLeft} days left`;
}

export function computeSLAExpiration(
  severity: Exclude<Severity, 'Info'>,
  createdAt: string,
  customDays?: number
): string {
  const days = customDays ?? SLA_DAYS_BY_SEVERITY[severity];
  const exp = new Date(createdAt);
  exp.setDate(exp.getDate() + days);
  return exp.toISOString();
}
