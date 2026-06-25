import { describe, it, expect } from 'vitest';
import {
  SEVERITY_COLORS,
  getSeverityColor,
  getCVSSColor,
  sortBySeverity,
} from '../severity';
import type { Severity } from '@/shared/types/cve';

describe('severity utils', () => {
  it('returns correct color for Critical', () => {
    expect(getSeverityColor('Critical')).toBe('#EF4444');
  });

  it('returns correct color for High', () => {
    expect(getSeverityColor('High')).toBe('#F97316');
  });

  it('returns fallback for unknown severity', () => {
    expect(getSeverityColor('Unknown' as Severity)).toBe('#6B7280');
  });

  it('has all 5 severity levels defined', () => {
    const severities: Severity[] = ['Critical', 'High', 'Medium', 'Low', 'Info'];
    severities.forEach((s) => {
      expect(SEVERITY_COLORS).toHaveProperty(s);
    });
  });

  describe('getCVSSColor', () => {
    it('returns red for >= 9.0', () => {
      expect(getCVSSColor(10.0)).toBe('#EF4444');
      expect(getCVSSColor(9.0)).toBe('#EF4444');
    });
    it('returns orange for 7.0-8.9', () => {
      expect(getCVSSColor(8.0)).toBe('#F97316');
    });
    it('returns yellow for 4.0-6.9', () => {
      expect(getCVSSColor(5.0)).toBe('#EAB308');
    });
    it('returns blue for < 4.0', () => {
      expect(getCVSSColor(3.0)).toBe('#3B82F6');
    });
    it('returns grey for undefined', () => {
      expect(getCVSSColor(undefined)).toBe('#6B7280');
    });
  });

  describe('sortBySeverity', () => {
    it('sorts Critical before High before Medium before Low', () => {
      const items = [
        { severity: 'Low' as Severity, id: 1 },
        { severity: 'Critical' as Severity, id: 2 },
        { severity: 'High' as Severity, id: 3 },
        { severity: 'Medium' as Severity, id: 4 },
      ];
      const sorted = sortBySeverity(items);
      expect(sorted.map((i) => i.severity)).toEqual([
        'Critical', 'High', 'Medium', 'Low',
      ]);
    });
  });
});
