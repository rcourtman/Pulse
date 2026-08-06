import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';

import { unifiedPlatformOverrideIdCandidates } from '../alertOverridesModel';
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

  it('keeps non-Proxmox systems out of the virtualization host threshold scope', () => {
    const [hasUnsavedChanges] = createSignal(false);
    const [, setOverviewOverrides] = createSignal([]);
    const resources = [
      // A cloud box running only the Pulse agent. It belongs to the Machines tab,
      // whose overrides resolve by agent identity, so a second row here would save
      // an override that agent alerting never reads.
      makeResource({
        id: 'docker-host-gcp-01',
        name: 'docker-host-gcp-01',
        type: 'agent',
        platformType: 'agent',
        sourceType: 'agent',
        platformData: {
          agent: { agentId: 'agent-gcp-01', platform: 'Linux' },
        },
      }),
      // A real Proxmox node must still appear under Virtualization Hosts.
      makeResource({
        id: 'cluster-a:pve01',
        name: 'pve01',
        type: 'agent',
        platformType: 'proxmox-pve',
        sourceType: 'api',
        platformData: {
          proxmox: { node: 'pve01', instance: 'cluster-a' },
        },
      }),
      // TrueNAS systems are canonical `agent` resources too, but their row must
      // use TrueNAS defaults. If this ID also enters Virtualization Hosts, a 95%
      // memory edit collides with the PVE node default and is stripped (#1593).
      makeResource({
        id: 'agent-truenas-01',
        name: 'strawberrynas',
        type: 'agent',
        platformType: 'truenas',
        sourceType: 'api',
        sources: ['truenas'],
        truenas: { hostname: 'strawberrynas' },
        proxmox: { node: 'strawberrynas' },
      }),
      // vSphere hosts also own a dedicated platform threshold section.
      makeResource({
        id: 'agent-vsphere-01',
        name: 'esxi-01',
        type: 'agent',
        platformType: 'vmware-vsphere',
        sourceType: 'api',
        sources: ['vmware'],
        vmware: { runtimeHostName: 'esxi-01' },
        proxmox: { node: 'esxi-01' },
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

    expect(result.virtualizationHostResources().map((resource) => resource.id)).toEqual([
      'cluster-a:pve01',
    ]);
    // The standalone machine is still reachable, via the Machines tab.
    expect(result.agentResources().map((resource) => resource.id)).toContain('docker-host-gcp-01');
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

  it('canonicalizes shared-storage overrides for the live thresholds surface', async () => {
    const [hasUnsavedChanges] = createSignal(false);
    const [overviewOverrides, setOverviewOverrides] = createSignal([]);
    const resources = [
      makeResource({
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
      'Main-pve1-ceph-pool': {
        usage: { trigger: 92, clear: 82 },
      } as any,
    });

    await waitFor(() => expect(result.overrides()).toHaveLength(1));

    expect(Object.keys(result.rawOverridesConfig())).toEqual(['Main-cluster-ceph-pool']);
    expect(result.overrides()[0]).toMatchObject({
      id: 'Main-cluster-ceph-pool',
      type: 'storage',
      thresholds: {
        usage: 92,
      },
    });
    expect(overviewOverrides()).toEqual(result.overrides());
  });

  it('exposes canonical container runtimes for TrueNAS-backed app workloads', async () => {
    const [hasUnsavedChanges] = createSignal(false);
    const [, setOverviewOverrides] = createSignal([]);
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

  it('exposes the unified platform override candidate chain used by display threshold lookups', () => {
    const resource = makeResource({
      id: 'k8s:prod:node:worker-01',
      type: 'k8s-node',
      platformId: 'prod',
      canonicalIdentity: {
        supersededIds: ['k8s:prod-old:node:worker-01'],
      },
      metricsTarget: { resourceId: 'metrics:worker-01' },
      discoveryTarget: { resourceId: 'discovery:worker-01' },
    } as Partial<Resource>);

    // Candidate precedence matches the buildProjectedOverrides indexing order,
    // so a stored override binds to the same row the tables color from.
    expect(unifiedPlatformOverrideIdCandidates(resource)).toEqual([
      'k8s:prod:node:worker-01',
      'k8s:prod-old:node:worker-01',
      'metrics:worker-01',
      'discovery:worker-01',
      'prod',
    ]);

    // Duplicates and blank entries collapse instead of producing repeat keys.
    expect(
      unifiedPlatformOverrideIdCandidates(
        makeResource({
          id: 'truenas-main',
          type: 'agent',
          platformId: 'truenas-main',
        }),
      ),
    ).toEqual(['truenas-main']);
  });
});
