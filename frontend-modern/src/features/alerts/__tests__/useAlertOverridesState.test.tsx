import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';

import { useAlertOverridesState } from '../useAlertOverridesState';

const makeResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: 'resource-1',
    name: 'resource-1',
    type: 'vm',
    ...overrides,
  }) as Resource;

describe('useAlertOverridesState', () => {
  it('owns raw override normalization and projected alert override read models outside config transport', async () => {
    const [hasUnsavedChanges] = createSignal(false);
    const [overviewOverrides, setOverviewOverrides] = createSignal([]);
    const resources = [
      makeResource({
        id: 'agent-resource-1',
        name: 'Agent 01',
        type: 'agent',
        platformData: {
          agent: {
            agentId: 'agent-1',
            platform: 'Linux',
          },
        },
      }),
    ];

    const { result } = renderHook(() =>
      useAlertOverridesState({
        allResources: () => resources,
        byType: (resourceType) => resources.filter((resource) => resource.type === resourceType),
        children: () => [],
        hasUnsavedChanges,
        setOverviewOverrides,
      }),
    );

    result.replaceRawOverridesConfig({
      'agent:agent-1/disk:NVMe 0n1': {
        disk: { trigger: 90, clear: 85 },
      } as any,
    });

    await waitFor(() => expect(result.overrides()).toHaveLength(1));

    expect(Object.keys(result.rawOverridesConfig())).toEqual(['agent:agent-1/disk:nvme-0n1']);
    expect(result.overrides()[0]).toMatchObject({
      id: 'agent:agent-1/disk:nvme-0n1',
      type: 'agentDisk',
      node: 'Agent 01',
      thresholds: {
        disk: 90,
      },
    });
    expect(overviewOverrides()).toEqual(result.overrides());
  });

  it('projects guest overrides without agent resources and clears stale overrides when config is emptied', async () => {
    const [hasUnsavedChanges] = createSignal(false);
    const [overviewOverrides, setOverviewOverrides] = createSignal([]);
    const resources = [
      makeResource({
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
      }),
    ];

    const { result } = renderHook(() =>
      useAlertOverridesState({
        allResources: () => resources,
        byType: (resourceType) => resources.filter((resource) => resource.type === resourceType),
        children: () => [],
        hasUnsavedChanges,
        setOverviewOverrides,
      }),
    );

    result.replaceRawOverridesConfig({
      'cluster-a:node-1:100': {
        cpu: { trigger: 95, clear: 90 },
        disabled: true,
      } as any,
    });

    await waitFor(() => expect(result.overrides()).toHaveLength(1));
    expect(Object.keys(result.rawOverridesConfig())).toEqual(['guest:cluster-a:100']);
    expect(result.overrides()[0]).toMatchObject({
      id: 'guest:cluster-a:100',
      type: 'guest',
      resourceType: 'VM',
      instance: 'cluster-a',
      node: 'node-2',
      disabled: true,
      thresholds: {
        cpu: 95,
      },
    });

    result.replaceRawOverridesConfig({});

    await waitFor(() => expect(result.overrides()).toEqual([]));
    expect(overviewOverrides()).toEqual([]);
  });

  it('exposes canonical container runtimes for TrueNAS-backed app workloads', async () => {
    const [hasUnsavedChanges] = createSignal(false);
    const [overviewOverrides, setOverviewOverrides] = createSignal([]);
    const resources = [
      makeResource({
        id: 'truenas-main',
        name: 'truenas-main',
        displayName: 'TrueNAS Main',
        type: 'agent',
        platformType: 'truenas',
        platformData: {
          agent: {
            agentId: 'truenas-main',
          },
        },
      }),
      makeResource({
        id: 'ix-nextcloud',
        name: 'nextcloud',
        displayName: 'Nextcloud',
        type: 'app-container',
        parentId: 'truenas-main',
      }),
    ];

    const { result } = renderHook(() =>
      useAlertOverridesState({
        allResources: () => resources,
        byType: (resourceType) => resources.filter((resource) => resource.type === resourceType),
        children: (resourceId) => resources.filter((resource) => resource.parentId === resourceId),
        hasUnsavedChanges,
        setOverviewOverrides,
      }),
    );

    await waitFor(() => expect(result.containerRuntimeResources()).toHaveLength(1));
    expect(result.containerRuntimeResources()[0]).toMatchObject({
      id: 'truenas-main',
      type: 'agent',
      platformType: 'truenas',
    });
  });
});
