import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';

import {
  buildContainerRuntimeResources,
  buildProjectedOverrides,
  normalizeRawOverridesConfig,
} from '../alertOverridesModel';

const makeResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: 'resource-1',
    name: 'resource-1',
    type: 'vm',
    ...overrides,
  }) as Resource;

describe('alertOverridesModel', () => {
  it('normalizes disk override ids into canonical agent disk keys', () => {
    expect(
      normalizeRawOverridesConfig({
        'agent:agent-1/disk:NVMe 0n1': {
          disk: { trigger: 90, clear: 85 },
        } as any,
      }),
    ).toEqual({
      'agent:agent-1/disk:nvme-0n1': {
        disk: { trigger: 90, clear: 85 },
      },
    });
  });

  it('normalizes legacy shared-storage override ids into canonical storage resource ids', () => {
    const storage = makeResource({
      id: 'storage-4a40f1c6',
      name: 'ceph-pool',
      type: 'storage',
      platformId: 'Main',
      metricsTarget: {
        resourceType: 'storage',
        resourceId: 'Main-cluster-ceph-pool',
      },
      proxmox: {
        instance: 'Main',
        node: 'cluster',
      },
      storage: {
        shared: true,
        isCeph: true,
        nodes: ['pve1', 'pve2'],
        type: 'rbd',
      },
    });

    expect(
      normalizeRawOverridesConfig(
        {
          'Main-pve1-ceph-pool': {
            usage: { trigger: 92, clear: 82 },
          } as any,
        },
        [storage],
      ),
    ).toEqual({
      'Main-cluster-ceph-pool': {
        usage: { trigger: 92, clear: 82 },
      },
    });
  });

  it('normalizes hashed storage resource ids into storage metrics target ids', () => {
    const storage = makeResource({
      id: 'storage-4a40f1c6',
      name: 'data_replication',
      type: 'storage',
      metricsTarget: {
        resourceType: 'storage',
        resourceId: 'Main-ceph-pool-data_replication',
      },
      storage: {
        shared: true,
        isCeph: true,
        type: 'ceph',
      },
    });

    expect(
      normalizeRawOverridesConfig(
        {
          'storage-4a40f1c6': {
            usage: { trigger: 70, clear: 65 },
          } as any,
        },
        [storage],
      ),
    ).toEqual({
      'Main-ceph-pool-data_replication': {
        usage: { trigger: 70, clear: 65 },
      },
    });
  });

  it('projects guest overrides without requiring agent-backed resources', () => {
    const guest = makeResource({
      id: 'cluster-a:node-2:100',
      name: 'db-01',
      type: 'vm',
      platformId: 'qemu/100',
      proxmox: {
        vmid: 100,
        node: 'node-2',
        instance: 'cluster-a',
      },
      platformData: {
        proxmox: {
          vmid: 100,
          node: 'node-2',
          instance: 'cluster-a',
        },
      },
    });

    expect(
      buildProjectedOverrides({
        rawConfig: normalizeRawOverridesConfig({
          'cluster-a:node-1:100': {
            cpu: { trigger: 95, clear: 90 },
            disabled: true,
          } as any,
        }),
        nodeResources: [],
        vmResources: [guest],
        containerResources: [],
        storageResources: [],
        agentResourceList: [],
        containerRuntimeResources: [],
        getChildren: () => [],
        pbsInstanceById: new Map(),
      }),
    ).toEqual([
      expect.objectContaining({
        id: 'guest:cluster-a:100',
        type: 'guest',
        resourceType: 'VM',
        instance: 'cluster-a',
        node: 'node-2',
        disabled: true,
        thresholds: {
          cpu: 95,
        },
      }),
    ]);
  });

  it('prefers explicit stable guest override ids during normalization', () => {
    expect(
      normalizeRawOverridesConfig({
        'guest:cluster-a:100': {
          cpu: { trigger: 70, clear: 65 },
        } as any,
        'cluster-a:node-1:100': {
          cpu: { trigger: 95, clear: 90 },
        } as any,
      }),
    ).toEqual({
      'guest:cluster-a:100': {
        cpu: { trigger: 70, clear: 65 },
      },
    });
  });

  it('derives canonical container runtimes from explicit docker hosts and TrueNAS app parents', () => {
    const truenas = makeResource({
      id: 'truenas-main',
      type: 'agent',
      name: 'truenas-main',
      platformType: 'truenas',
    });
    const dockerHost = makeResource({
      id: 'docker-main',
      type: 'docker-host',
      name: 'docker-main',
    });
    const app = makeResource({
      id: 'ix-nextcloud',
      type: 'app-container',
      name: 'nextcloud',
      parentId: 'truenas-main',
    });

    expect(
      buildContainerRuntimeResources({
        allResources: [truenas, dockerHost, app],
        dockerHostResources: [dockerHost],
      }).map((resource) => resource.id),
    ).toEqual(['docker-main', 'truenas-main']);
  });

  it('projects TrueNAS app overrides through the canonical container runtime surface', () => {
    const truenas = makeResource({
      id: 'truenas-main',
      type: 'agent',
      name: 'truenas-main',
      displayName: 'TrueNAS Main',
      platformType: 'truenas',
    });
    const app = makeResource({
      id: 'ix-nextcloud',
      type: 'app-container',
      name: 'nextcloud',
      displayName: 'Nextcloud',
      parentId: 'truenas-main',
      status: 'running',
    });

    expect(
      buildProjectedOverrides({
        rawConfig: {
          'docker:truenas-main/ix-nextcloud': {
            cpu: { trigger: 95, clear: 90 },
          } as any,
        },
        nodeResources: [],
        vmResources: [],
        containerResources: [],
        storageResources: [],
        agentResourceList: [truenas],
        containerRuntimeResources: [truenas],
        getChildren: (resourceId) => (resourceId === 'truenas-main' ? [app] : []),
        pbsInstanceById: new Map(),
      }),
    ).toEqual([
      expect.objectContaining({
        id: 'docker:truenas-main/ix-nextcloud',
        type: 'dockerContainer',
        name: 'Nextcloud',
        node: 'TrueNAS Main',
        thresholds: {
          cpu: 95,
        },
      }),
    ]);
  });

  it('projects shared-storage overrides through the canonical storage resource id', () => {
    const storage = makeResource({
      id: 'storage-4a40f1c6',
      name: 'ceph-pool',
      displayName: 'ceph-pool',
      type: 'storage',
      platformId: 'Main',
      metricsTarget: {
        resourceType: 'storage',
        resourceId: 'Main-cluster-ceph-pool',
      },
      proxmox: {
        instance: 'Main',
        node: 'cluster',
      },
      storage: {
        shared: true,
        isCeph: true,
        nodes: ['pve1', 'pve2'],
        type: 'rbd',
      },
      platformData: {
        node: 'cluster',
        instance: 'Main',
      },
    });

    expect(
      buildProjectedOverrides({
        rawConfig: {
          'Main-pve1-ceph-pool': {
            usage: { trigger: 92, clear: 82 },
          } as any,
        },
        nodeResources: [],
        vmResources: [],
        containerResources: [],
        storageResources: [storage],
        agentResourceList: [],
        containerRuntimeResources: [],
        getChildren: () => [],
        pbsInstanceById: new Map(),
      }),
    ).toEqual([
      expect.objectContaining({
        id: 'Main-cluster-ceph-pool',
        type: 'storage',
        name: 'ceph-pool',
        thresholds: {
          usage: 92,
        },
      }),
    ]);
  });
});
