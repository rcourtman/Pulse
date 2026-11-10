import { describe, expect, it } from 'vitest';
import type { Investigation } from '@/api/patrol';
import type { Resource } from '@/types/resource';
import {
  buildPatrolCommandAccessHandover,
  buildPatrolCommandAccessSummary,
  buildPatrolGuestActionAccessSummary,
  buildPatrolGuestActionsHandover,
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

const RUNNER_REASON =
  'Pulse cannot start, stop or restart guests on pve1: no Pulse action runner is connected on that node. "Allow Pulse to run commands on this host" does not cover guest actions. Use Proxmox for this.';

const guest = (
  id: string,
  node: string,
  overrides: Partial<Resource> = {},
  proxmox: Partial<NonNullable<Resource['proxmox']>> = {},
): Resource =>
  ({
    id,
    type: 'system-container',
    name: id,
    displayName: id,
    status: 'online',
    sources: ['proxmox'],
    proxmox: { nodeName: node, instance: 'home', ...proxmox },
    ...overrides,
  }) as Resource;

const noRunner = (name: string) => [
  { name, available: false, reasonCode: 'action_runner_unavailable', reason: RUNNER_REASON },
];

describe('patrol guest action access', () => {
  it('names the Proxmox nodes where the server refuses a guest action for want of a runner', () => {
    const summary = buildPatrolGuestActionAccessSummary([
      guest('gitea', 'pve1', { actionReadiness: noRunner('start') }),
      guest('pihole', 'pve1', { actionReadiness: noRunner('stop') }),
      // A runner is connected on pve2: its guests carry no refusal.
      guest('nextcloud', 'pve2'),
      // Another refusal is not this one.
      guest('locked', 'pve3', {
        actionReadiness: [{ name: 'start', available: false, reasonCode: 'locked' }],
      }),
      guest(
        'web',
        'zeta',
        { type: 'vm', actionReadiness: noRunner('reboot') },
        { nodeDisplayName: 'Alpha' },
      ),
      // The same node name on another connection is another node.
      guest('other', 'pve1', { actionReadiness: noRunner('start') }, { instance: 'office' }),
      // Not judged: a template, a guest with no node, and things that are not Proxmox guests.
      guest('tmpl', 'pve9', { actionReadiness: noRunner('start') }, { template: true }),
      guest('lost', '', { actionReadiness: noRunner('start') }),
      host('pve1', { agent: { agentId: 'pve1', commandsEnabled: true } }),
      { id: 'jail', type: 'system-container', name: 'jail', status: 'online' } as Resource,
    ]);
    expect(summary.total).toBe(5);
    expect(summary.noRunner.map((entry) => entry.name)).toEqual(['Alpha', 'pve1', 'pve1']);
    expect(new Set(summary.noRunner.map((entry) => entry.id)).size).toBe(3);
  });

  it('lists nothing when no guest action is refused for a runner', () => {
    expect(buildPatrolGuestActionAccessSummary([guest('nextcloud', 'pve2')])).toEqual({
      total: 1,
      noRunner: [],
    });
    expect(buildPatrolGuestActionAccessSummary([])).toEqual({ total: 0, noRunner: [] });
  });

  it('builds the hand-back for a stopped guest from what the server wrote with the run', () => {
    const run = {
      guest_actions_unavailable: {
        guest_name: 'gitea',
        node_name: 'pve1',
        actions: ['start'],
        reason: RUNNER_REASON,
      },
    } as Investigation;
    expect(buildPatrolGuestActionsHandover(run, false)).toEqual({
      title: 'Patrol could not start gitea itself.',
      detail: RUNNER_REASON,
    });
    expect(buildPatrolGuestActionsHandover(run, true)).toBeUndefined();
    expect(buildPatrolGuestActionsHandover({} as Investigation, false)).toBeUndefined();
    expect(buildPatrolGuestActionsHandover(null, false)).toBeUndefined();
  });

  it('stays quiet for a running guest, whose refused stop is rarely why a run handed back', () => {
    const run = {
      guest_actions_unavailable: {
        guest_name: 'nextcloud',
        node_name: 'pve1',
        actions: ['reboot', 'shutdown', 'stop'],
        reason: RUNNER_REASON,
      },
    } as Investigation;
    expect(buildPatrolGuestActionsHandover(run, false)).toBeUndefined();
  });

  it('still says what to do when the server sent no sentence', () => {
    const run = {
      guest_actions_unavailable: { node_name: 'pve1', actions: ['start'], reason: '' },
    } as Investigation;
    expect(buildPatrolGuestActionsHandover(run, false)).toEqual({
      title: 'Patrol could not start this guest itself.',
      detail:
        'Pulse cannot start, stop or restart guests on pve1: no Pulse action runner is connected on that node. Use Proxmox for this.',
    });
  });
});
