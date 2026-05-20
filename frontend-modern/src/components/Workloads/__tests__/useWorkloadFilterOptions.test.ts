import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import { useWorkloadFilterOptions } from '../useWorkloadFilterOptions';

const makeGuest = (overrides: Partial<WorkloadGuest>): WorkloadGuest =>
  ({
    id: 'guest-1',
    vmid: 101,
    name: 'guest-1',
    node: 'node-a',
    instance: 'cluster-a',
    status: 'running',
    type: 'vm',
    cpu: 0,
    cpus: 2,
    memory: { total: 1024, used: 256, free: 768, usage: 25 },
    disk: { total: 1024, used: 256, free: 768, usage: 25 },
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 0,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: new Date().toISOString(),
    workloadType: 'vm',
    ...overrides,
  }) as WorkloadGuest;

describe('useWorkloadFilterOptions', () => {
  it('scopes host and runtime facets to the selected platform', () => {
    const [guests] = createSignal<WorkloadGuest[]>([
      makeGuest({
        id: 'docker-orders',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        platformScopes: ['proxmox-pve', 'docker'],
        contextLabel: 'docker-prod-a',
        dockerHostId: 'docker-prod-a',
        containerRuntime: 'docker',
      }),
      makeGuest({
        id: 'truenas-plex',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'truenas',
        contextLabel: 'truenas-main',
        containerRuntime: 'truenas-apps',
      }),
      makeGuest({
        id: 'pve-vm-101',
        type: 'vm',
        workloadType: 'vm',
        platformType: 'proxmox-pve',
        node: 'pve-prod-a',
        instance: 'core-fabric',
      }),
    ]);
    const [viewMode] = createSignal<ViewMode>('app-container');
    const [containerRuntime, setContainerRuntime] = createSignal('');
    const [selectedPlatform, setSelectedPlatform] = createSignal<string | null>(null);
    const [selectedKubernetesContext, setSelectedKubernetesContext] = createSignal<string | null>(
      null,
    );
    const [selectedKubernetesNamespace, setSelectedKubernetesNamespace] = createSignal<
      string | null
    >(null);
    const [selectedNode] = createSignal<string | null>(null);
    const [platformScope, setPlatformScope] = createSignal('docker');

    const { result } = renderHook(() =>
      useWorkloadFilterOptions({
        allGuests: guests,
        isWorkloadsRoute: () => false,
        allowEmbeddedScopeFilters: () => true,
        viewMode,
        platformScope,
        containerRuntime,
        selectedPlatform,
        selectedNode,
        selectedKubernetesContext,
        selectedKubernetesNamespace,
        setContainerRuntime,
        setSelectedPlatform,
        setSelectedKubernetesContext,
        handleNodeSelect: vi.fn(),
        setSelectedKubernetesNamespace,
      }),
    );

    expect(result.workloadNodeOptions()).toEqual([
      { value: 'docker-prod-a', label: 'docker-prod-a' },
    ]);
    expect(result.containerRuntimeOptions()).toEqual(['docker']);

    setPlatformScope('proxmox-pve');

    expect(result.workloadNodeOptions()).toEqual([
      { value: 'docker-prod-a', label: 'docker-prod-a' },
      { value: 'core-fabric-pve-prod-a', label: 'pve-prod-a' },
    ]);
    expect(result.containerRuntimeOptions()).toEqual(['docker']);
  });
});
