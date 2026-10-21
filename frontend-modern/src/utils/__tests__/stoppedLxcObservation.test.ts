import { describe, expect, it } from 'vitest';
import { getGuestHardwareLastKnownReason } from '../guestHardwareObservation';
import { getGuestNetworkLastKnownReason } from '../guestNetworkObservation';
import type { WorkloadGuest } from '@/types/workloads';

// The resource remains online and its lease unexpired in the consuming fixture.
// Native power contradicts execution, not the independent host configuration.
const lxc = {
  type: 'system-container',
  platformScopes: ['proxmox-pve'],
  agentStale: false,
  networkInterfacesFromAgent: true,
};

describe('stopped LXC observations', () => {
  it.each(['stopped', 'offline', ' STOPPED '])(
    'qualifies retained hardware and selected in-container interfaces at %s',
    (status) => {
      const input = { ...lxc, status };
      const before = structuredClone(input);
      expect(getGuestHardwareLastKnownReason(input)).toBe(
        'Proxmox reports this container stopped.',
      );
      expect(getGuestNetworkLastKnownReason(input)).toBe(
        'Using last known interface details. Proxmox reports this container stopped. Retained addresses and counters do not establish a current connection.',
      );
      expect(input).toEqual(before);
    },
  );

  it('recognises legacy lxc without requiring canonical platform metadata', () => {
    const input = { type: 'lxc', status: 'offline', networkInterfacesFromAgent: true };
    expect(getGuestHardwareLastKnownReason(input)).toContain('this container stopped');
    expect(getGuestNetworkLastKnownReason(input)).toContain('this container stopped');
  });

  it.each([false, undefined])(
    'preserves independent or unspecified interface provenance %s',
    (fromAgent) => {
      const input = { ...lxc, status: 'stopped', networkInterfacesFromAgent: fromAgent };
      expect(getGuestHardwareLastKnownReason(input)).toContain('this container stopped');
      expect(getGuestNetworkLastKnownReason(input)).toBeNull();
    },
  );

  it.each(['running', 'online', 'paused', 'suspended', 'unknown', 'private-state'])(
    'does not invent LXC power semantics at %s',
    (status) => {
      expect(getGuestHardwareLastKnownReason({ ...lxc, status })).toBeUndefined();
      expect(getGuestNetworkLastKnownReason({ ...lxc, status })).toBeNull();
    },
  );

  it.each([
    { platformScopes: ['vmware-vsphere'] },
    { platformScopes: ['docker'] },
    { platformScopes: [], platformType: 'kubernetes' },
    { type: 'app-container' },
    { type: 'pod' },
    { type: 'unknown-type' },
    { workloadType: 'app-container' },
  ] as Partial<WorkloadGuest>[])(
    'does not borrow LXC scope or ignore explicit workload type %j',
    (overrides) => {
      const input = { ...lxc, status: 'stopped', ...overrides };
      // An unknown raw type uses the established system-container fallback;
      // without positive container identity it must not gain a power claim.
      expect(getGuestHardwareLastKnownReason(input)).toBeUndefined();
      expect(getGuestNetworkLastKnownReason(input)).toBeNull();
    },
  );
});
