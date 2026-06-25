import { describe, it, expect } from 'vitest';
import { getSLAStatus, getSLADaysLeft, formatSLALabel } from '../sla';

describe('SLA utils', () => {
  it('returns breached when past expiration', () => {
    expect(getSLAStatus('2020-01-01T00:00:00Z')).toBe('breached');
  });

  it('returns at_risk when 1-3 days left', () => {
    const tomorrow = new Date(Date.now() + 2 * 86_400_000).toISOString();
    expect(getSLAStatus(tomorrow)).toBe('at_risk');
  });

  it('returns ok when more than 3 days left', () => {
    const nextWeek = new Date(Date.now() + 10 * 86_400_000).toISOString();
    expect(getSLAStatus(nextWeek)).toBe('ok');
  });

  describe('getSLADaysLeft', () => {
    it('returns negative for past dates', () => {
      expect(getSLADaysLeft('2020-01-01T00:00:00Z')).toBeLessThan(0);
    });

    it('returns positive for future dates', () => {
      const future = new Date(Date.now() + 5 * 86_400_000).toISOString();
      expect(getSLADaysLeft(future)).toBeGreaterThan(0);
    });
  });

  describe('formatSLALabel', () => {
    it('formats overdue correctly', () => {
      expect(formatSLALabel(-3)).toBe('Overdue 3d');
    });
    it('formats today correctly', () => {
      expect(formatSLALabel(0)).toBe('Due today');
    });
    it('formats single day', () => {
      expect(formatSLALabel(1)).toBe('1 day left');
    });
    it('formats multiple days', () => {
      expect(formatSLALabel(7)).toBe('7 days left');
    });
  });
});
