import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  PROXMOX_TAB_SPECS,
  buildProxmoxPageModel,
  buildVisibleProxmoxTabSpecs,
  filterProxmoxNodesForSearch,
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

  describe('filterProxmoxNodesForSearch', () => {
    const minipc = makeResource({
      id: 'minipc',
      type: 'agent',
      proxmox: { nodeName: 'minipc', clusterName: 'homelab' },
    });
    const delly = makeResource({
      id: 'delly',
      type: 'agent',
      proxmox: { nodeName: 'delly', clusterName: 'homelab' },
    });
    const debianGo = makeResource({
      id: 'system-container-112',
      type: 'system-container',
      name: 'debian-go',
      proxmox: { vmid: 112, nodeName: 'minipc' },
    });

    it('returns every node when the search term is empty', () => {
      expect(filterProxmoxNodesForSearch([minipc, delly], [debianGo], '')).toEqual([minipc, delly]);
    });

    it('keeps the host node of a matching guest so a guest search does not empty the nodes table', () => {
      // Regression: searching a guest name used to filter the nodes table to
      // nothing, surfacing the "No Proxmox VE nodes" empty state even though the
      // guest's host node exists and the guest is listed below.
      const result = filterProxmoxNodesForSearch([minipc, delly], [debianGo], 'debian-go');
      expect(result).toEqual([minipc]);
    });

    it('still matches a node by its own name', () => {
      expect(filterProxmoxNodesForSearch([minipc, delly], [debianGo], 'delly')).toEqual([delly]);
    });
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
    expect(model.resources.map((resource) => resource.id)).not.toContain('docker-host');
    // Replication is gated on the fetched job count, not on anything in the
    // resource model: the jobs bypass the unified-resource pipeline.
    expect(buildVisibleProxmoxTabSpecs(model, 1).map((tab) => tab.id)).toEqual([
      'overview',
      'storage',
      'replication',
      'backups',
      'ceph',
      'mail',
    ]);
  });

  it('hides Replication for a PVE estate without replication jobs', () => {
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

    expect(buildVisibleProxmoxTabSpecs(model, 0).map((tab) => tab.id)).toEqual(['overview']);
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
