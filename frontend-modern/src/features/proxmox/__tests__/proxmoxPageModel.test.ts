import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  PROXMOX_TAB_SPECS,
  buildProxmoxPageModel,
  buildVisibleProxmoxTabSpecs,
  getResourceVersion,
  resolveProxmoxPlatformScope,
} from '../proxmoxPageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('proxmoxPageModel', () => {
  it('keeps the platform-native section set aligned with the legacy Proxmox workspace', () => {
    expect(PROXMOX_TAB_SPECS.map((tab) => tab.id)).toEqual([
      'overview',
      'storage',
      'replication',
      'backups',
      'ceph',
      'mail',
    ]);
  });

  it('builds a Proxmox-first estate model from canonical v6 resources', () => {
    const model = buildProxmoxPageModel([
      makeResource({
        id: 'pve-node-1',
        type: 'agent',
        displayName: 'pve-node-1',
        proxmox: { nodeName: 'pve-node-1', clusterName: 'alpha' },
      }),
      makeResource({
        id: 'vm-101',
        type: 'vm',
        displayName: 'database',
        parentName: 'pve-node-1',
        status: 'running',
        proxmox: { vmid: 101, nodeName: 'pve-node-1' },
        platformData: {
          proxmox: { lastBackup: 1_700_000_000_000 },
        },
        recentChanges: [
          {
            id: 'replication-1',
            observedAt: '2026-05-15T08:00:00.000Z',
            resourceId: 'vm-101',
            kind: 'activity',
            sourceType: 'platform_event',
            sourceAdapter: 'proxmox_adapter',
            confidence: 'high',
            reason: 'Replication job completed',
          },
        ],
      }),
      makeResource({
        id: 'local-zfs',
        type: 'storage',
        displayName: 'local-zfs',
        parentName: 'pve-node-1',
        storage: { type: 'zfs', isCeph: false },
      }),
      makeResource({
        id: 'ceph-alpha',
        type: 'ceph',
        displayName: 'ceph-alpha',
        platformData: { ceph: { healthStatus: 'healthy' } },
      }),
      makeResource({
        id: 'pbs-main',
        type: 'pbs',
        displayName: 'pbs-main',
        platformType: 'proxmox-pbs',
        pbs: { version: '3.2' },
      }),
      makeResource({
        id: 'pmg-main',
        type: 'pmg',
        displayName: 'pmg-main',
        platformType: 'proxmox-pmg',
        platformData: { pmg: { version: '8.1' } },
      }),
      makeResource({
        id: 'docker-host',
        type: 'docker-host',
        displayName: 'docker-host',
        platformType: 'docker',
      }),
    ]);

    expect(model.summary).toMatchObject({
      clusterCount: 1,
      nodeCount: 1,
      guestCount: 1,
      runningGuestCount: 1,
      storageCount: 1,
      pbsCount: 1,
      pmgCount: 1,
      cephCount: 1,
    });
    expect(model.clusterGroups).toHaveLength(1);
    expect(model.clusterGroups[0]).toMatchObject({
      label: 'alpha',
      nodes: [expect.objectContaining({ id: 'pve-node-1' })],
      guests: [expect.objectContaining({ id: 'vm-101' })],
      storage: [expect.objectContaining({ id: 'local-zfs' })],
    });
    expect(model.replicationChanges).toHaveLength(1);
    expect(model.replicationChanges[0]).toMatchObject({
      resource: expect.objectContaining({ id: 'vm-101' }),
      change: expect.objectContaining({ id: 'replication-1' }),
    });
    expect(model.resources.map((resource) => resource.id)).not.toContain('docker-host');
    expect(buildVisibleProxmoxTabSpecs(model).map((tab) => tab.id)).toEqual([
      'overview',
      'storage',
      'replication',
      'backups',
      'ceph',
      'mail',
    ]);
  });

  it('hides Replication for a PVE estate without replication signals', () => {
    const model = buildProxmoxPageModel([
      makeResource({
        id: 'pve-node-1',
        type: 'agent',
        displayName: 'pve-node-1',
        proxmox: { nodeName: 'pve-node-1', clusterName: 'alpha' },
      }),
      makeResource({
        id: 'vm-101',
        type: 'vm',
        displayName: 'database',
        parentName: 'pve-node-1',
        status: 'running',
        proxmox: { vmid: 101, nodeName: 'pve-node-1' },
      }),
    ]);

    expect(buildVisibleProxmoxTabSpecs(model).map((tab) => tab.id)).toEqual(['overview']);
  });

  it('resolves Proxmox suite scope from canonical platform hints', () => {
    expect(resolveProxmoxPlatformScope(makeResource({ id: 'pbs', type: 'pbs' }))).toBe(
      'proxmox-pbs',
    );
    expect(resolveProxmoxPlatformScope(makeResource({ id: 'pmg', type: 'pmg' }))).toBe(
      'proxmox-pmg',
    );
    expect(
      resolveProxmoxPlatformScope(
        makeResource({ id: 'pve-source', type: 'agent', sources: ['proxmox'] }),
      ),
    ).toBe('proxmox-pve');
  });

  it('surfaces compact PVE versions from canonical Proxmox resource metadata', () => {
    expect(
      getResourceVersion(
        makeResource({
          id: 'pve-node',
          type: 'agent',
          proxmox: { pveVersion: 'pve-manager/9.1.9/ee7bad0a3d1546c9' },
        }),
      ),
    ).toBe('9.1.9');

    expect(
      getResourceVersion(
        makeResource({
          id: 'pve-node-platform-data',
          type: 'agent',
          platformData: {
            proxmox: { pveVersion: 'pve-manager/8.3.3/bbba1c53a1a65b24' },
          },
        }),
      ),
    ).toBe('8.3.3');
  });
});
