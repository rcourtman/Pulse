import { describe, expect, it } from 'vitest';
import { getGuestNetworkLastKnownReason } from '../guestNetworkObservation';

const vm = { type: 'vm', platformScopes: ['proxmox-pve'], status: 'running' };

describe('selected guest network details are not connection evidence', () => {
  it.each(['paused', 'suspended', 'stopped', 'offline', ' PAUSED '])(
    'qualifies both retained VM producers at native power %s despite online/leased carriers',
    (status) => {
      for (const networkInterfacesFromAgent of [true, false, undefined]) {
        const input = { ...vm, status, agentStale: false, networkInterfacesFromAgent };
        const before = structuredClone(input);
        expect(getGuestNetworkLastKnownReason(input)).toContain(
          'Retained addresses and counters do not establish a current connection',
        );
        expect(input).toEqual(before);
      }
    },
  );

  it('recognises the legacy VM shape without borrowing a different platform', () => {
    expect(getGuestNetworkLastKnownReason({ type: 'qemu', status: 'stopped' })).toContain(
      'this VM stopped',
    );
    expect(
      getGuestNetworkLastKnownReason({
        type: 'qemu',
        platformScopes: ['vmware-vsphere'],
        status: 'stopped',
      }),
    ).toBeNull();
  });

  it.each(['running', 'online', 'healthy', 'unknown', 'starting', 'private-native-value'])(
    'does not invent a shutdown or reveal raw power %s',
    (status) => {
      expect(getGuestNetworkLastKnownReason({ ...vm, status })).toBeNull();
    },
  );

  it('qualifies only the silent agent’s selected list; missing producer preserves legacy behaviour', () => {
    for (const networkInterfacesFromAgent of [true, undefined]) {
      expect(
        getGuestNetworkLastKnownReason({
          ...vm,
          networkInterfacesFromAgent,
          agentStale: true,
        }),
      ).toBe(
        'Using last known interface details. The Pulse Agent in this guest stopped reporting.',
      );
    }
    expect(
      getGuestNetworkLastKnownReason({
        ...vm,
        networkInterfacesFromAgent: false,
        agentStale: true,
      }),
    ).toBeNull();
  });

  it.each(['lxc', 'system-container', 'app-container', 'pod'])(
    'does not treat %s host/configuration interfaces as in-VM observations',
    (type) => {
      expect(getGuestNetworkLastKnownReason({ ...vm, type, status: 'stopped' })).toBeNull();
    },
  );
});
