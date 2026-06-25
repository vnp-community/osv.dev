import type { FindingStatus } from '@/shared/types/finding';

export const VALID_TRANSITIONS: Record<FindingStatus, FindingStatus[]> = {
  active: ['mitigated', 'false_positive', 'risk_accepted', 'out_of_scope'],
  mitigated: ['active'],
  false_positive: ['active'],
  risk_accepted: ['active'],
  out_of_scope: ['active'],
  duplicate: [],
};

export const STATUS_LABELS: Record<FindingStatus, string> = {
  active: 'Active',
  mitigated: 'Mitigated',
  false_positive: 'False Positive',
  risk_accepted: 'Risk Accepted',
  out_of_scope: 'Out of Scope',
  duplicate: 'Duplicate',
};

export const STATUS_COLORS: Record<FindingStatus, string> = {
  active: '#EF4444',
  mitigated: '#10B981',
  false_positive: '#6B7280',
  risk_accepted: '#8B5CF6',
  out_of_scope: '#6B7280',
  duplicate: '#374151',
};

export function canTransition(from: FindingStatus, to: FindingStatus): boolean {
  return VALID_TRANSITIONS[from].includes(to);
}

export function getAvailableTransitions(current: FindingStatus): FindingStatus[] {
  return VALID_TRANSITIONS[current];
}
