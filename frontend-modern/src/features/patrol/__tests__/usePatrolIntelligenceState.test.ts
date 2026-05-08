import { describe, expect, it } from 'vitest';

import {
  resolvePatrolAutonomyLevelForSave,
  resolvePatrolAutonomySettingsForSave,
} from '../usePatrolIntelligenceState';

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

  describe('resolvePatrolAutonomySettingsForSave', () => {
    it('clears stale full-mode state when safe remediation is locked', () => {
      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'full',
          fullModeUnlocked: true,
          autoFixLocked: true,
        }),
      ).toEqual({ autonomyLevel: 'monitor', fullModeUnlocked: false });
    });

    it('does not carry full-mode state into non-remediation modes', () => {
      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'monitor',
          fullModeUnlocked: true,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'monitor', fullModeUnlocked: false });

      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'approval',
          fullModeUnlocked: true,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'approval', fullModeUnlocked: false });
    });

    it('promotes remediation mode to full only when full mode is explicitly unlocked', () => {
      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'assisted',
          fullModeUnlocked: true,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'full', fullModeUnlocked: true });

      expect(
        resolvePatrolAutonomySettingsForSave({
          level: 'full',
          fullModeUnlocked: false,
          autoFixLocked: false,
        }),
      ).toEqual({ autonomyLevel: 'assisted', fullModeUnlocked: false });
    });
  });
});
