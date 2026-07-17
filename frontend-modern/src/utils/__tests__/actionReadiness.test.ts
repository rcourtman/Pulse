import { describe, expect, it } from 'vitest';
import { getActionReadinessRefusal } from '@/utils/actionReadiness';
import type { ResourceActionReadiness } from '@/types/resource';

describe('getActionReadinessRefusal', () => {
  it('returns undefined when no readiness list is present', () => {
    expect(getActionReadinessRefusal(undefined, 'update')).toBeUndefined();
    expect(getActionReadinessRefusal([], 'update')).toBeUndefined();
  });

  it('ignores entries for other capabilities and entries that are available', () => {
    const readiness: ResourceActionReadiness[] = [
      { name: 'restart', available: false, reason: 'restart refused' },
      { name: 'update', available: true, reason: 'should not surface' },
    ];
    expect(getActionReadinessRefusal(readiness, 'update')).toBeUndefined();
  });

  it('prefers the server-provided reason over the reason-code fallback', () => {
    const readiness: ResourceActionReadiness[] = [
      {
        name: 'update',
        available: false,
        reasonCode: 'command_agent_disconnected',
        reason: 'The Pulse agent on this host is still on an older version.',
      },
    ];
    expect(getActionReadinessRefusal(readiness, 'update')).toBe(
      'The Pulse agent on this host is still on an older version.',
    );
  });

  it('matches capability names case-insensitively with trimming', () => {
    const readiness: ResourceActionReadiness[] = [
      { name: ' Update ', available: false, reason: 'refused' },
    ];
    expect(getActionReadinessRefusal(readiness, 'update')).toBe('refused');
  });

  it('falls back to a canned message per reason code when reason is blank', () => {
    const cases: Array<[string, string]> = [
      ['command_agent_disconnected', 'Docker / Podman command agent is not connected.'],
      ['command_agent_unavailable', 'Docker / Podman command execution is not available.'],
      [
        'stale_inventory',
        'Docker / Podman inventory is not fresh enough to run lifecycle actions.',
      ],
      ['host_policy_blocked', 'Docker / Podman host policy blocks mutating lifecycle actions.'],
      [
        'unsupported_handler',
        'This container action is not routed through the supported lifecycle executor.',
      ],
    ];
    for (const [reasonCode, message] of cases) {
      expect(
        getActionReadinessRefusal(
          [{ name: 'update', available: false, reasonCode, reason: '  ' }],
          'update',
        ),
      ).toBe(message);
    }
  });

  it('returns undefined for an unknown reason code with no reason text', () => {
    expect(
      getActionReadinessRefusal(
        [{ name: 'update', available: false, reasonCode: 'mystery_code' }],
        'update',
      ),
    ).toBeUndefined();
  });
});
