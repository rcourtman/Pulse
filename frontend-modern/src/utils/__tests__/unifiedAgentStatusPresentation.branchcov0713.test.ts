import { describe, expect, it } from 'vitest';
import {
  getUnifiedAgentLookupStatusPresentation,
  getUnifiedAgentStatusPresentation,
  MONITORING_STOPPED_STATUS_LABEL,
  type UnifiedAgentMonitoringState,
} from '@/utils/unifiedAgentStatusPresentation';

// Branch-coverage companion to unifiedAgentStatusPresentation.test.ts.
// The sibling test already locks the two canonical happy paths
// (('active','online') and ('removed','offline')) plus both lookup arms.
// This file targets the remaining branches of the two exported functions:
//   - the `state === 'removed'` guard's precedence over the connected check
//     and its independence from healthStatus
//   - every accepted connected token (online / running / healthy) plus the
//     normalize step (trim + lowercase) inside isConnectedHealthStatus
//   - the default (not-removed, not-connected) arm for both truthy and
//     falsy healthStatus, including the reachable `|| 'unknown'` fallback
//   - non-canonical state values falling through to the default arm
//   - lookup edge inputs (truthy/falsy non-boolean values cast to boolean)

const REMOVED_BADGE = 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200';
const CONNECTED_BADGE = 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300';
const DEFAULT_BADGE = 'bg-surface-alt text-base-content';
const LOOKUP_CONNECTED_BADGE = 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300';
const LOOKUP_DISCONNECTED_BADGE =
  'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200';

describe('getUnifiedAgentStatusPresentation — branch coverage', () => {
  describe('state === "removed" guard', () => {
    it('takes precedence over a connected healthStatus (removed arm wins over connected arm)', () => {
      // 'online' would normally select the green connected badge, but the
      // removed guard returns first -> amber + "Monitoring stopped".
      const result = getUnifiedAgentStatusPresentation('removed', 'online');
      expect(result).toEqual({ badgeClass: REMOVED_BADGE, label: MONITORING_STOPPED_STATUS_LABEL });
      expect(result.badgeClass).not.toBe(CONNECTED_BADGE);
      expect(result.label).toBe('Monitoring stopped');
    });

    it('ignores an undefined healthStatus', () => {
      expect(getUnifiedAgentStatusPresentation('removed', undefined)).toEqual({
        badgeClass: REMOVED_BADGE,
        label: MONITORING_STOPPED_STATUS_LABEL,
      });
    });

    it('ignores a null healthStatus', () => {
      expect(getUnifiedAgentStatusPresentation('removed', null)).toEqual({
        badgeClass: REMOVED_BADGE,
        label: MONITORING_STOPPED_STATUS_LABEL,
      });
    });

    it('ignores an empty-string healthStatus', () => {
      expect(getUnifiedAgentStatusPresentation('removed', '')).toEqual({
        badgeClass: REMOVED_BADGE,
        label: MONITORING_STOPPED_STATUS_LABEL,
      });
    });
  });

  describe('connected arm (isConnectedHealthStatus === true)', () => {
    it.each(['online', 'running', 'healthy'] as const)(
      'accepts the connected token "%s" and echoes it verbatim as the label',
      (token) => {
        expect(getUnifiedAgentStatusPresentation('active', token)).toEqual({
          badgeClass: CONNECTED_BADGE,
          label: token,
        });
      },
    );

    it('normalizes a cased/whitespace-padded "ONLINE" to the connected arm (trim + lowercase)', () => {
      // isConnectedHealthStatus normalizes via trim().toLowerCase(), so
      // '  ONLINE  ' is recognized as connected. The label, however, is the
      // raw healthStatus argument (not the normalized form).
      const result = getUnifiedAgentStatusPresentation('active', '  ONLINE  ');
      expect(result).toEqual({ badgeClass: CONNECTED_BADGE, label: '  ONLINE  ' });
    });

    it('matches "Healthy" case-insensitively (lowercased to "healthy")', () => {
      expect(getUnifiedAgentStatusPresentation('active', 'Healthy')).toEqual({
        badgeClass: CONNECTED_BADGE,
        label: 'Healthy',
      });
    });

    it('does NOT treat a non-connected token ("offline") as connected', () => {
      // Proves the connected arm is skipped for an unrecognized status.
      const result = getUnifiedAgentStatusPresentation('active', 'offline');
      expect(result.badgeClass).not.toBe(CONNECTED_BADGE);
      expect(result.badgeClass).toBe(DEFAULT_BADGE);
    });
  });

  describe('default arm (not removed, not connected)', () => {
    it.each(['offline', 'degraded', 'unknown', 'error', 'warning'])(
      'uses the surface-alt badge and echoes a truthy non-connected healthStatus "%s" as the label',
      (status) => {
        expect(getUnifiedAgentStatusPresentation('active', status)).toEqual({
          badgeClass: DEFAULT_BADGE,
          label: status,
        });
      },
    );

    it('falls back to the "unknown" label when healthStatus is undefined (reachable || right arm)', () => {
      expect(getUnifiedAgentStatusPresentation('active', undefined)).toEqual({
        badgeClass: DEFAULT_BADGE,
        label: 'unknown',
      });
    });

    it('falls back to the "unknown" label when healthStatus is null', () => {
      expect(getUnifiedAgentStatusPresentation('active', null)).toEqual({
        badgeClass: DEFAULT_BADGE,
        label: 'unknown',
      });
    });

    it('falls back to the "unknown" label when healthStatus is the empty string', () => {
      // '' is falsy, so `healthStatus || 'unknown'` yields 'unknown'.
      expect(getUnifiedAgentStatusPresentation('active', '')).toEqual({
        badgeClass: DEFAULT_BADGE,
        label: 'unknown',
      });
    });

    it('echoes a whitespace-only healthStatus verbatim (truthy, so || fallback is skipped)', () => {
      // '   ' is truthy even though it normalizes to '' (not connected),
      // so the raw whitespace becomes the label.
      const result = getUnifiedAgentStatusPresentation('active', '   ');
      expect(result).toEqual({ badgeClass: DEFAULT_BADGE, label: '   ' });
      expect(result.label).not.toBe('unknown');
    });

    it('routes a non-canonical state value to the default arm (only "removed" is special)', () => {
      // A state outside {'active','removed'} is not 'removed', so it falls
      // through to the connected/default logic.
      const result = getUnifiedAgentStatusPresentation(
        'paused' as UnifiedAgentMonitoringState,
        'offline',
      );
      expect(result).toEqual({ badgeClass: DEFAULT_BADGE, label: 'offline' });
    });

    it('routes an empty-string state with no healthStatus to the default arm + "unknown" label', () => {
      expect(
        getUnifiedAgentStatusPresentation('' as UnifiedAgentMonitoringState, undefined),
      ).toEqual({ badgeClass: DEFAULT_BADGE, label: 'unknown' });
    });

    it('routes a non-canonical state with a connected healthStatus to the connected arm', () => {
      // Confirms the connected check is independent of the state value
      // (as long as state !== 'removed').
      expect(
        getUnifiedAgentStatusPresentation('paused' as UnifiedAgentMonitoringState, 'running'),
      ).toEqual({ badgeClass: CONNECTED_BADGE, label: 'running' });
    });
  });

  describe('return shape', () => {
    it('always returns exactly the {badgeClass,label} keys (no extra fields)', () => {
      const result = getUnifiedAgentStatusPresentation('active', 'online');
      expect(Object.keys(result).sort()).toEqual(['badgeClass', 'label']);
    });
  });
});

describe('getUnifiedAgentLookupStatusPresentation — branch coverage', () => {
  it('returns the green Connected presentation for a truthy boolean', () => {
    // Note: the lookup connected badge intentionally uses text-green-700
    // (distinct from the status connected badge's text-green-800).
    expect(getUnifiedAgentLookupStatusPresentation(true)).toEqual({
      badgeClass: LOOKUP_CONNECTED_BADGE,
      label: 'Connected',
    });
  });

  it('returns the amber "Not reporting yet" presentation for a falsy boolean', () => {
    expect(getUnifiedAgentLookupStatusPresentation(false)).toEqual({
      badgeClass: LOOKUP_DISCONNECTED_BADGE,
      label: 'Not reporting yet',
    });
  });

  it.each([
    ['0 (number)', 0],
    ['empty string', ''],
    ['null', null],
    ['undefined', undefined],
  ] as const)('routes a falsy non-boolean value (%s) to the not-connected arm', (_name, value) => {
    expect(getUnifiedAgentLookupStatusPresentation(value as unknown as boolean)).toEqual({
      badgeClass: LOOKUP_DISCONNECTED_BADGE,
      label: 'Not reporting yet',
    });
  });

  it('routes a truthy non-boolean value (1) to the connected arm', () => {
    expect(getUnifiedAgentLookupStatusPresentation(1 as unknown as boolean)).toEqual({
      badgeClass: LOOKUP_CONNECTED_BADGE,
      label: 'Connected',
    });
  });

  it('always returns exactly the {badgeClass,label} keys (no extra fields)', () => {
    const result = getUnifiedAgentLookupStatusPresentation(true);
    expect(Object.keys(result).sort()).toEqual(['badgeClass', 'label']);
  });
});
