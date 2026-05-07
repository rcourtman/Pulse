import { describe, expect, it } from 'vitest';

import { resolvePatrolAutonomyLevelForSave } from '../usePatrolIntelligenceState';

describe('usePatrolIntelligenceState', () => {
  describe('resolvePatrolAutonomyLevelForSave', () => {
    it('clamps stale paid autonomy to monitor when safe remediation is locked', () => {
      expect(resolvePatrolAutonomyLevelForSave('full', true, true)).toBe('monitor');
      expect(resolvePatrolAutonomyLevelForSave('assisted', false, true)).toBe('monitor');
      expect(resolvePatrolAutonomyLevelForSave('approval', false, true)).toBe('monitor');
    });

    it('preserves paid autonomy choices when safe remediation is available', () => {
      expect(resolvePatrolAutonomyLevelForSave('assisted', false, false)).toBe('assisted');
      expect(resolvePatrolAutonomyLevelForSave('assisted', true, false)).toBe('full');
      expect(resolvePatrolAutonomyLevelForSave('approval', false, false)).toBe('approval');
    });
  });
});
