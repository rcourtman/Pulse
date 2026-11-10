import { describe, expect, it } from 'vitest';
import type { Investigation } from '@/api/patrol';
import type { Resource } from '@/types/resource';
import {
  buildPatrolCommandAccessHandover,
  buildPatrolCommandAccessSummary,
  isCommandsOffAgentHost,
  patrolModeNeedsCommandAccess,
} from '../patrolCommandAccessModel';

const host = (id: string, overrides: Partial<Resource> = {}): Resource =>
  ({
    id,
    type: 'agent',
    name: id,
    displayName: id,
    status: 'online',
    sources: ['agent'],
    agent: { agentId: id, stale: false },
    ...overrides,
  }) as Resource;

describe('patrol command access', () => {
  it('judges only a host its own Pulse agent is reporting for', () => {
    expect(isCommandsOffAgentHost(host('docker01'))).toBe(true);
    expect(
      isCommandsOffAgentHost(host('ready', { agent: { agentId: 'ready', commandsEnabled: true } })),
    ).toBe(false);
    expect(
      isCommandsOffAgentHost(host('stopped', { agent: { agentId: 'stopped', stale: true } })),
    ).toBe(false);
    // TrueNAS and vSphere hosts carry an agent facet with no Pulse agent behind it.
    expect(isCommandsOffAgentHost(host('nas', { sources: ['truenas'] }))).toBe(false);
    expect(isCommandsOffAgentHost(host('vm', { agent: undefined }))).toBe(false);
  });

  it('counts reachable hosts and names the rest in order', () => {
    const summary = buildPatrolCommandAccessSummary([
      host('tower', { displayName: 'Tower' }),
      host('ready', { agent: { agentId: 'ready', commandsEnabled: true } }),
      host('docker01'),
      host('stopped', { agent: { agentId: 'stopped', stale: true } }),
      host('nas', { sources: ['truenas'] }),
      { id: 'vm-1', type: 'vm', name: 'vm-1', displayName: 'vm-1', status: 'online' } as Resource,
    ]);
    expect(summary.total).toBe(3);
    expect(summary.ready).toBe(1);
    expect(summary.commandsOff.map((entry) => entry.name)).toEqual(['docker01', 'Tower']);
  });

  it('needs command access in every mode that investigates', () => {
    expect(patrolModeNeedsCommandAccess('monitor')).toBe(false);
    expect(patrolModeNeedsCommandAccess('approval')).toBe(true);
    expect(patrolModeNeedsCommandAccess('assisted')).toBe(true);
    expect(patrolModeNeedsCommandAccess('full')).toBe(true);
  });

  it('builds the hand-back from what the server wrote with the run', () => {
    const run = {
      commands_disabled: {
        host_name: 'docker01',
        reason: 'Commands are not enabled on the Pulse agent on docker01.',
        step: 'Server-written step.',
      },
    } as Investigation;
    expect(buildPatrolCommandAccessHandover(run, false)).toEqual({
      title:
        'Patrol could not read logs or run a fix on docker01: commands are not enabled on its Pulse agent.',
      step: 'Server-written step.',
      actionLabel: 'Get install command',
      actionHref: '/settings/infrastructure?add=agent&commands=1',
    });
    expect(buildPatrolCommandAccessHandover(run, true)).toBeUndefined();
    expect(buildPatrolCommandAccessHandover({} as Investigation, false)).toBeUndefined();
    expect(buildPatrolCommandAccessHandover(null, false)).toBeUndefined();
    const withoutStep = {
      commands_disabled: { host_name: 'docker01', reason: '', step: '' },
    } as Investigation;
    expect(buildPatrolCommandAccessHandover(withoutStep, false)?.step).toBe(
      'To allow them, run the Pulse install command on docker01 again with "Allow Pulse to run commands on this host" ticked.',
    );
  });
});
