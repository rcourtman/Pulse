import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  mergeCanonicalResourceDeltaSnapshot,
  mergeCanonicalResourceSnapshot,
} from '@/utils/resourceStateAdapters';

const guest = (type: 'vm' | 'system-container', instance = 'site-a'): Resource => ({
  id: `${instance}:pve1:101`,
  type,
  name: 'same-name',
  displayName: 'same-name',
  platformId: instance,
  platformType: 'proxmox-pve',
  sourceType: 'api',
  sources: ['proxmox'],
  status: 'running',
  lastSeen: 1000,
  proxmox: {
    vmid: 101,
    instance,
    nodeName: 'pve1',
    runtimeStatus: 'running',
    ...(type === 'vm' ? { guestAgentStatus: 'expected-unreachable' } : {}),
    backupInProgress: true,
    lastBackup: '2026-10-24T12:00:00Z',
    diskStatusReason: 'agent-not-running',
  },
});

const paths = ['full', 'delta', 'fast-delta'] as const;
const merge = (path: (typeof paths)[number], incoming: Resource, previous: Resource[]) =>
  path === 'full'
    ? mergeCanonicalResourceSnapshot([incoming, previous[1]!], previous)
    : mergeCanonicalResourceDeltaSnapshot(
        [incoming, previous[1]!],
        previous,
        new Set([incoming.id]),
        path === 'fast-delta' ? new Map([[incoming.id, ['proxmox']]]) : undefined,
      );

describe.each(paths)('backup activity through %s merging', (path) => {
  it.each(['vm', 'system-container'] as const)(
    'withdraws omitted native false for a %s without inventing guest-agent recovery',
    (type) => {
      const active = guest(type);
      const other = guest(type, 'site-b');
      const previous = mergeCanonicalResourceSnapshot([active, other], []);
      const incoming = structuredClone(active);
      delete incoming.proxmox!.backupInProgress;
      incoming.proxmox!.lock = 'backup';
      const [cleared, independent] = merge(path, incoming, previous);
      expect(cleared!.proxmox?.backupInProgress ?? false).toBe(false);
      expect(cleared!.proxmox?.lastBackup).toBe(active.proxmox!.lastBackup);
      expect(cleared!.proxmox?.guestAgentStatus).toBe(active.proxmox!.guestAgentStatus);
      expect(cleared!.proxmox?.diskStatusReason).toBe('agent-not-running');
      expect(cleared!.proxmox?.lock).toBe('backup');
      expect(independent!.proxmox?.backupInProgress).toBe(true);
      expect(previous[0]!.proxmox?.backupInProgress).toBe(true);
      expect(active.proxmox?.backupInProgress).toBe(true);
    },
  );

  it.each([true, false])('honours explicit native activity %s', (backupInProgress) => {
    const active = guest('vm');
    const previous = mergeCanonicalResourceSnapshot([active, guest('vm', 'site-b')], []);
    const incoming = {
      ...active,
      proxmox: { ...active.proxmox, backupInProgress },
      platformData: { vmid: 101, backupInProgress: !backupInProgress },
    };
    expect(merge(path, incoming, previous)[0]!.proxmox?.backupInProgress).toBe(backupInProgress);
  });

  it.each([
    { vmid: 101, cpus: 4 },
    { runtimeStatus: 'running' },
    { vmid: 0, runtimeStatus: 'running' },
    { vmid: -1, runtimeStatus: 'running' },
    { vmid: 1.5, runtimeStatus: 'running' },
    { vmid: 101, runtimeStatus: ' ' },
  ])('keeps activity when a thin facet has no complete native runtime: %j', (proxmox) => {
    const active = guest('vm');
    const previous = mergeCanonicalResourceSnapshot([active, guest('vm', 'site-b')], []);
    const [kept] = merge(path, { ...active, proxmox }, previous);
    expect(kept!.proxmox?.backupInProgress).toBe(true);
  });

  it('clears the native compatibility mirror too', () => {
    const active = guest('vm');
    active.platformData = { sources: ['proxmox'], proxmox: active.proxmox };
    const previous = mergeCanonicalResourceSnapshot([active, guest('vm', 'site-b')], []);
    const incoming = structuredClone(active);
    delete incoming.proxmox!.backupInProgress;
    incoming.platformData = { sources: ['proxmox'], proxmox: incoming.proxmox };
    // A mirror change requires the general delta path, not the metric-only fast path.
    const [cleared] =
      path === 'full'
        ? mergeCanonicalResourceSnapshot([incoming, previous[1]!], previous)
        : mergeCanonicalResourceDeltaSnapshot(
            [incoming, previous[1]!],
            previous,
            new Set([incoming.id]),
            path === 'fast-delta'
              ? new Map([[incoming.id, ['proxmox', 'platformData.proxmox']]])
              : undefined,
          );
    expect(cleared!.proxmox?.backupInProgress).toBeUndefined();
    expect(cleared!.platformData).toMatchObject({ proxmox: incoming.proxmox });
    expect(
      (cleared!.platformData?.proxmox as Resource['proxmox'])?.backupInProgress,
    ).toBeUndefined();
  });
});

describe('legacy backup activity', () => {
  it.each([true, false])('preserves an initial explicit legacy boolean: %s', (flag) => {
    const raw = {
      ...guest('system-container'),
      proxmox: undefined,
      platformData: { vmid: 101, instance: 'site-a', backupInProgress: flag },
    };
    expect(mergeCanonicalResourceSnapshot([raw], [])[0]!.proxmox?.backupInProgress).toBe(flag);
  });

  it.each([true, false, undefined])('uses only an explicit legacy boolean: %s', (flag) => {
    const active = guest('system-container');
    const previous = mergeCanonicalResourceSnapshot([active], []);
    const incoming = {
      ...active,
      proxmox: undefined,
      platformData: {
        vmid: 101,
        instance: 'site-a',
        ...(flag === undefined ? {} : { backupInProgress: flag }),
      },
    };
    for (const [row] of [
      mergeCanonicalResourceSnapshot([incoming], previous),
      mergeCanonicalResourceDeltaSnapshot([incoming], previous, new Set([active.id])),
    ]) {
      expect(row!.proxmox?.backupInProgress).toBe(flag ?? true);
    }
  });

  it('does not infer a missing legacy flag from a previously merged runtime', () => {
    const active = guest('vm');
    const [row] = mergeCanonicalResourceSnapshot(
      [{ ...active, proxmox: undefined, platformData: { vmid: 101, cpus: 2 } }],
      mergeCanonicalResourceSnapshot([active], []),
    );
    expect(row!.proxmox?.backupInProgress).toBe(true);
  });
});
