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

  it('projects guest overrides without requiring agent-backed resources', () => {
    const guest = makeResource({
      id: 'vm-100',
      name: 'db-01',
      type: 'vm',
      platformId: 'qemu/100',
      platformData: {
        vmid: 100,
        node: 'pve-1',
        instance: 'qemu/100',
      },
    });

    expect(
      buildProjectedOverrides({
        rawConfig: {
          'vm-100': {
            cpu: { trigger: 95, clear: 90 },
            disabled: true,
          } as any,
        },
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
        id: 'vm-100',
        type: 'guest',
        resourceType: 'VM',
        disabled: true,
        thresholds: {
          cpu: 95,
        },
      }),
    ]);
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
});
