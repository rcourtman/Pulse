import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import {
  computeWorkloadIOEmphasis,
  createWorkloadSortComparator,
  filterWorkloads,
} from '@/components/Workloads/workloadSelectors';

const makeGuest = (i: number, overrides?: Partial<WorkloadGuest>): WorkloadGuest => ({
  id: `guest-${i}`,
  vmid: 100 + i,
  name: `workload-${i}`,
  node: `node-${i % 5}`,
  instance: `cluster-${i % 3}`,
  status: 'running',
  type: 'vm',
  cpu: (i % 100) / 100,
  cpus: 2,
  memory: { total: 4096, used: ((i % 80) / 100) * 4096, free: 0, usage: (i % 80) / 100 },
  disk: { total: 102400, used: ((i % 60) / 100) * 102400, free: 0, usage: (i % 60) / 100 },
  networkIn: i * 100,
  networkOut: i * 50,
  diskRead: i * 10,
  diskWrite: i * 5,
  uptime: i * 3600,
  template: false,
  lastBackup: 0,
  tags: [],
  lock: '',
  lastSeen: new Date().toISOString(),
  workloadType: 'vm',
  ...overrides,
});

const baseFilterParams = {
  viewMode: 'all' as const,
  statusMode: 'all',
  searchTerm: '',
  selectedNode: null,
  selectedHostHint: null,
  selectedKubernetesContext: null,
};

describe('workloadSelectors (branch coverage 0724pm)', () => {
  describe('createWorkloadSortComparator - ?? / || fallback arms', () => {
    it('coerces a non-numeric cpu to 0 via `getWorkloadCPUPercent(cpu) ?? 0` on both a and b', () => {
      const nanCpu = makeGuest(1, {
        id: 'nan',
        name: 'nan-cpu',
        cpu: undefined as unknown as number,
      });
      const realCpu = makeGuest(2, { id: 'real', name: 'real-cpu', cpu: 0.5 });
      const cmp = createWorkloadSortComparator('cpu', 'asc')!;

      // nan-cpu -> getWorkloadCPUPercent(undefined) -> undefined -> ?? 0 -> 0
      // real-cpu -> 50. Ascending: nan (0) sorts before real (50).
      expect(cmp(nanCpu, realCpu)).toBe(-1);
      // Reversed orientation exercises bVal ?? 0 for the non-numeric guest.
      expect(cmp(realCpu, nanCpu)).toBe(1);
    });

    it('falls back to 0 for missing diskRead/diskWrite on both a and b of the diskIo arm', () => {
      const noIo = makeGuest(1, {
        id: 'no-io',
        name: 'no-io',
        diskRead: undefined as unknown as number,
        diskWrite: undefined as unknown as number,
      });
      const withIo = makeGuest(2, {
        id: 'with-io',
        name: 'with-io',
        diskRead: 50,
        diskWrite: 30,
      });
      const cmp = createWorkloadSortComparator('diskIo', 'asc')!;

      // noIo -> max(0, undefined||0) + max(0, undefined||0) = 0 + 0 = 0
      // withIo -> 50 + 30 = 80. Ascending: no-io (0) before with-io (80).
      expect(cmp(noIo, withIo)).toBe(-1);
      // Reversed orientation exercises the b-side diskRead||0 / diskWrite||0 fallbacks.
      expect(cmp(withIo, noIo)).toBe(1);
    });

    it('falls back to 0 for missing networkIn/networkOut on both a and b of the netIo arm', () => {
      const noNet = makeGuest(1, {
        id: 'no-net',
        name: 'no-net',
        networkIn: undefined as unknown as number,
        networkOut: undefined as unknown as number,
      });
      const withNet = makeGuest(2, {
        id: 'with-net',
        name: 'with-net',
        networkIn: 20,
        networkOut: 10,
      });
      const cmp = createWorkloadSortComparator('netIo', 'asc')!;

      // noNet -> max(0, undefined||0) + max(0, undefined||0) = 0
      // withNet -> 20 + 10 = 30. Ascending: no-net (0) before with-net (30).
      expect(cmp(noNet, withNet)).toBe(-1);
      // Reversed orientation exercises the b-side networkIn||0 / networkOut||0 fallbacks.
      expect(cmp(withNet, noNet)).toBe(1);
    });
  });

  describe('filterWorkloads - `(g.x || "")` missing-field fallback arms', () => {
    it('treats a pod with no namespace as a non-match via `(g.namespace || "")` in the k8s namespace filter', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-no-ns',
          name: 'api',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx',
          node: 'worker',
          contextLabel: 'ctx',
          namespace: undefined as unknown as string,
          status: 'running',
        }),
        makeGuest(2, {
          id: 'pod-with-ns',
          name: 'web',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx',
          node: 'worker',
          contextLabel: 'ctx',
          namespace: 'payments',
          status: 'running',
        }),
      ];

      const result = filterWorkloads({
        ...baseFilterParams,
        guests,
        viewMode: 'pod',
        selectedKubernetesNamespace: 'payments',
      });

      // pod-no-ns: (undefined || '').trim() === 'payments' -> '' === 'payments' -> false (dropped)
      // pod-with-ns: kept.
      expect(result.map((g) => g.id)).toEqual(['pod-with-ns']);
    });

    it('treats a vm with no clusterName as a non-match via `(g.clusterName || "")` in the cluster filter', () => {
      const guests = [
        makeGuest(1, {
          id: 'vm-no-cluster',
          name: 'vm-a',
          type: 'vm',
          workloadType: 'vm',
          instance: 'inst-a',
          node: 'node-a',
          clusterName: undefined as unknown as string,
        }),
        makeGuest(2, {
          id: 'vm-with-cluster',
          name: 'vm-b',
          type: 'vm',
          workloadType: 'vm',
          instance: 'inst-b',
          node: 'node-b',
          clusterName: 'prod-cluster',
        }),
      ];

      const result = filterWorkloads({
        ...baseFilterParams,
        guests,
        viewMode: 'vm',
        selectedCluster: 'prod-cluster',
      });

      // vm-no-cluster: (undefined || '').trim() === 'prod-cluster' -> false (dropped)
      // vm-with-cluster: kept.
      expect(result.map((g) => g.id)).toEqual(['vm-with-cluster']);
    });

    it('treats an app-container with no containerRuntime as a non-match via `(g.containerRuntime || "")` in the runtime filter', () => {
      const guests = [
        makeGuest(1, {
          id: 'no-runtime',
          name: 'redis',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: undefined as unknown as string,
          contextLabel: 'host-a',
          node: '',
          instance: '',
        }),
        makeGuest(2, {
          id: 'with-runtime',
          name: 'nginx',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'docker',
          contextLabel: 'host-b',
          node: '',
          instance: '',
        }),
      ];

      const result = filterWorkloads({
        ...baseFilterParams,
        guests,
        viewMode: 'app-container',
        containerRuntime: 'docker',
      });

      // no-runtime: (undefined || '').trim().toLowerCase() === 'docker' -> '' === 'docker' -> false (dropped)
      // with-runtime: kept.
      expect(result.map((g) => g.id)).toEqual(['with-runtime']);
    });
  });

  describe('computeWorkloadIOEmphasis - `?? 0` nullish fallback arms', () => {
    it('coerces null network/disk telemetry to 0 on every `guest.x ?? 0` arm and blends with real values', () => {
      const nullTelemetry = makeGuest(1, {
        id: 'null-io',
        name: 'null-io',
        networkIn: null as unknown as number,
        networkOut: null as unknown as number,
        diskRead: null as unknown as number,
        diskWrite: null as unknown as number,
      });
      const realTelemetry = makeGuest(2, {
        id: 'real-io',
        name: 'real-io',
        networkIn: 10,
        networkOut: 6,
        diskRead: 8,
        diskWrite: 4,
      });

      const result = computeWorkloadIOEmphasis([nullTelemetry, realTelemetry]);

      // nullTelemetry contributes 0 to both facets (null ?? 0 -> 0); realTelemetry
      // contributes network 16 and disk 12. With the two-sample distribution
      // [0, 16] / [0, 12]: median = midpoint, mad = midpoint, max/p97/p99 = upper.
      expect(result).toEqual({
        network: { median: 8, mad: 8, max: 16, p97: 16, p99: 16, count: 2 },
        diskIO: { median: 6, mad: 6, max: 12, p97: 12, p99: 12, count: 2 },
      });
    });
  });
});
