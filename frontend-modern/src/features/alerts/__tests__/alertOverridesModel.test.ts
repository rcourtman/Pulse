import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';

import {
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
        dockerHostResources: [],
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
});
