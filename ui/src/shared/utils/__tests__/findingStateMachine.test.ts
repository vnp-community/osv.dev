import { describe, it, expect } from 'vitest';
import { canTransition, getAvailableTransitions, VALID_TRANSITIONS } from '../findingStateMachine';

describe('finding state machine', () => {
  describe('canTransition', () => {
    it('allows active → mitigated', () => {
      expect(canTransition('active', 'mitigated')).toBe(true);
    });
    it('allows active → false_positive', () => {
      expect(canTransition('active', 'false_positive')).toBe(true);
    });
    it('allows mitigated → active (reopen)', () => {
      expect(canTransition('mitigated', 'active')).toBe(true);
    });
    it('disallows duplicate → any', () => {
      expect(canTransition('duplicate', 'active')).toBe(false);
    });
    it('disallows mitigated → false_positive (must go through active)', () => {
      expect(canTransition('mitigated', 'false_positive')).toBe(false);
    });
    it('disallows active → duplicate (system-assigned only)', () => {
      expect(canTransition('active', 'duplicate')).toBe(false);
    });
  });

  it('duplicate has no valid transitions', () => {
    expect(VALID_TRANSITIONS.duplicate).toHaveLength(0);
  });

  it('getAvailableTransitions returns correct transitions', () => {
    expect(getAvailableTransitions('active')).toContain('mitigated');
    expect(getAvailableTransitions('active')).toContain('false_positive');
    expect(getAvailableTransitions('duplicate')).toHaveLength(0);
  });
});
