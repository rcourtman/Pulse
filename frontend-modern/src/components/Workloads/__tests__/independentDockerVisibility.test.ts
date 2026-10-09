import { createRoot, createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { useWorkloads } from '@/hooks/useWorkloads';
import { buildDockerPageModel } from '@/features/docker/dockerPageModel';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import { buildNestedWorkloadContextByGuestId } from '../nestedWorkloadContext';

vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn(), getOrgID: () => 'default' }));

const guest = (installation: string): Resource => ({
  id: `${installation}:node-${installation}:141`,
  type: 'vm',
  name: 'same-short-name',
  displayName: 'same-short-name',
  platformType: 'proxmox-pve',
  platformId: installation,
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_791_533_000_000,
  platformScopes: ['proxmox-pve'],
  sources: ['proxmox'],
  proxmox: {
    instance: installation,
    nodeName: `node-${installation}`,
    vmid: 141,
    runtimeStatus: 'running',
  },
});

const container: Resource = {
  id: 'app-container:home-agent:fixture-container',
  type: 'app-container',
  name: 'independent-container',
  displayName: 'independent-container',
  platformType: 'docker',
  platformId: 'home-agent',
  sourceType: 'agent',
  status: 'online',
  lastSeen: 1_791_533_000_000,
  sources: ['docker'],
  // Exercise ambiguity refusal even when a retained row carries PVE scope.
  platformScopes: ['proxmox-pve', 'docker'],
  parentId: 'home-agent',
  parentName: 'same-short-name',
  docker: {
    hostSourceId: 'home-agent',
    hostname: 'same-short-name',
    containerId: 'fixture-container',
    runtime: 'docker',
    containerState: 'running',
  },
};

describe('independent Docker visibility across repeated guest identities', () => {
  it('keeps an existing Docker row without a new agent report while refusing ambiguous PVE nesting', async () => {
    const home = guest('home');
    const remote = guest('remote');
    const [snapshot, setSnapshot] = createSignal<Resource[]>([home, container]);
    let dispose = () => {};
    let workloads!: ReturnType<typeof useWorkloads>;
    createRoot((d) => {
      dispose = d;
      workloads = useWorkloads(() => true, { resourceSnapshot: snapshot });
    });
    const nested = () =>
      buildNestedWorkloadContextByGuestId({
        guests: workloads.workloads(),
        visibleGuests: workloads.workloads().filter((row) => row.workloadType === 'vm'),
        excludedWorkloadTypes: ['app-container'],
        platformScope: 'proxmox-pve',
      });
    try {
      await Promise.resolve();
      const original = workloads.workloads().find((row) => row.workloadType === 'app-container')!;
      expect(original).toBeDefined();
      expect(Object.values(nested()).map((context) => context.count)).toEqual([1]);
      // Only the remote PVE installation is added. The original Docker object,
      // canonical id and lastSeen are unchanged: a fresh report cannot mask loss.
      setSnapshot([home, remote, container]);
      await Promise.resolve();
      const current = workloads.workloads().find((row) => row.workloadType === 'app-container')!;
      expect(getCanonicalWorkloadId(current)).toBe(getCanonicalWorkloadId(original));
      expect(current.dockerHostId).toBe('home-agent');
      expect(current.lastSeen).toBe(original.lastSeen);
      expect(nested()).toEqual({});
      expect(buildDockerPageModel(snapshot()).containers).toEqual([container]);
      expect(snapshot()[2]).toBe(container);

      // A complete empty Docker inventory is different from parent ambiguity.
      setSnapshot([home, remote]);
      await Promise.resolve();
      expect(workloads.workloads().some((row) => row.workloadType === 'app-container')).toBe(false);
      expect(buildDockerPageModel(snapshot()).containers).toEqual([]);
    } finally {
      dispose();
    }
  });
});
