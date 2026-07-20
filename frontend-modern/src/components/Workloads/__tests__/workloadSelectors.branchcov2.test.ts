import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import {
  computeWorkloadStats,
  createWorkloadSortComparator,
  filterWorkloads,
  getWorkloadGroupLabel,
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

describe('workloadSelectors (branch coverage 2)', () => {
  describe('createWorkloadSortComparator', () => {
    it('routes both-null disk values through the name/id tiebreaker and orders empties last', () => {
      const nullDisk = { total: 0, used: 50, free: 0, usage: NaN };
      const guests = [
        makeGuest(1, { id: 'zz', name: 'zebra', disk: nullDisk }),
        makeGuest(2, { id: 'aa', name: 'alpha', disk: nullDisk }),
        makeGuest(3, {
          id: 'mm',
          name: 'mid',
          disk: { total: 100, used: 20, free: 80, usage: 0.2 },
        }),
        makeGuest(4, {
          id: 'bb',
          name: 'bravo',
          disk: { total: 100, used: 90, free: 10, usage: 0.9 },
        }),
      ];

      const diskAsc = createWorkloadSortComparator('disk', 'asc');
      const diskDesc = createWorkloadSortComparator('disk', 'desc');

      // Non-empty values sort first (asc: mid=20, bravo=90); both-null pair falls
      // through aIsEmpty && bIsEmpty -> tiebreak (alpha < zebra).
      expect([...guests].sort(diskAsc!).map((g) => g.id)).toEqual(['mm', 'bb', 'aa', 'zz']);
      // desc only flips the non-empty comparison; empties still sort last by name asc.
      expect([...guests].sort(diskDesc!).map((g) => g.id)).toEqual(['bb', 'mm', 'aa', 'zz']);
    });

    it('uses the name/id tiebreaker when a generic key is undefined on every guest', () => {
      const guests = [
        makeGuest(1, { id: 'b', name: 'beta' }),
        makeGuest(2, { id: 'a', name: 'alpha' }),
      ];

      const cmp = createWorkloadSortComparator('nonexistentField', 'asc');
      // Every value is undefined -> both empty -> tiebreak by name (alpha < beta).
      expect([...guests].sort(cmp!).map((g) => g.id)).toEqual(['a', 'b']);
    });

    it('returns 0 when numeric values are equal and both names and ids match', () => {
      const a = makeGuest(1, { id: 'dup', name: 'same', cpu: 0.5 });
      const b = makeGuest(2, { id: 'dup', name: 'same', cpu: 0.5 });

      const cmp = createWorkloadSortComparator('cpu', 'asc');
      // Equal numeric values -> tiebreak; equal names -> id compare; equal ids -> 0.
      expect(cmp!(a, b)).toBe(0);
      expect(cmp!(b, a)).toBe(0);
    });
  });

  describe('filterWorkloads', () => {
    it('skips the node-scope filter in pod view mode so pods on other nodes survive', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-a',
          name: 'api',
          type: 'pod',
          workloadType: 'pod',
          node: 'worker-a',
          instance: 'ctx',
          contextLabel: 'ctx',
          namespace: 'default',
          status: 'running',
        }),
        makeGuest(2, {
          id: 'pod-b',
          name: 'web',
          type: 'pod',
          workloadType: 'pod',
          node: 'worker-b',
          instance: 'ctx',
          contextLabel: 'ctx',
          namespace: 'default',
          status: 'running',
        }),
      ];

      const result = filterWorkloads({
        ...baseFilterParams,
        guests,
        viewMode: 'pod',
        selectedNode: 'worker-a',
      });

      // workloadHostScopeId(pod) === '' would remove both if the node filter ran;
      // because viewMode === 'pod' the guard is skipped and both pods remain.
      expect(result.map((g) => g.id)).toEqual(['pod-a', 'pod-b']);
    });

    it('excludes non-pod guests during kubernetes namespace filtering in pod view', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-a',
          name: 'api',
          type: 'pod',
          workloadType: 'pod',
          node: 'worker-a',
          instance: 'ctx',
          contextLabel: 'ctx',
          namespace: 'payments',
          status: 'running',
        }),
        makeGuest(2, {
          id: 'vm-a',
          name: 'vm',
          type: 'vm',
          workloadType: 'vm',
          node: 'node-a',
          instance: 'inst-a',
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

      // The vm hits `resolveWorkloadType(g) !== 'pod' -> return false` in the
      // namespace filter and is dropped (it would also be dropped by the later
      // view-mode filter, but this exercises the guard's true arm).
      expect(result.map((g) => g.id)).toEqual(['pod-a']);
    });

    it('excludes system-containers from the runtime filter under the combined container view mode', () => {
      const guests = [
        makeGuest(1, {
          id: 'app-1',
          name: 'redis',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'docker',
          contextLabel: 'host-a',
          node: '',
          instance: '',
          status: 'running',
        }),
        makeGuest(2, {
          id: 'lxc-1',
          name: 'db',
          type: 'lxc',
          workloadType: 'system-container',
          instance: 'inst-a',
          node: 'node-a',
          containerRuntime: 'docker',
          status: 'running',
        }),
      ];

      const result = filterWorkloads({
        ...baseFilterParams,
        guests,
        viewMode: 'container',
        containerRuntime: 'docker',
      });

      // 'container' view mode keeps both app- and system-containers via
      // workloadMatchesViewMode, so the system-container reaches the runtime
      // filter where `resolveWorkloadType(g) !== 'app-container' -> return false`.
      expect(result.map((g) => g.id)).toEqual(['app-1']);
    });

    it('does not apply the platform filter when the normalized platform is "all"', () => {
      const guests = [
        makeGuest(1, {
          id: 'truenas-1',
          name: 'nextcloud',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: ['truenas'],
          contextLabel: 'host-a',
          node: '',
          instance: '',
          status: 'running',
        }),
        makeGuest(2, {
          id: 'docker-1',
          name: 'grafana',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: ['docker'],
          contextLabel: 'host-b',
          node: '',
          instance: '',
          status: 'running',
        }),
      ];

      const result = filterWorkloads({
        ...baseFilterParams,
        guests,
        selectedPlatform: 'all',
      });

      // normalizeSourcePlatformQueryValue('all') === 'all' -> filter skipped.
      expect(result.map((g) => g.id)).toEqual(['truenas-1', 'docker-1']);
    });

    it('counts an empty-status guest as degraded via the (status || "") fallback in degraded mode', () => {
      const guests = [
        makeGuest(1, { id: 'empty', name: 'empty', status: '' }),
        makeGuest(2, { id: 'running', name: 'running', status: 'running' }),
        makeGuest(3, { id: 'offline', name: 'offline', status: 'offline' }),
      ];

      const result = filterWorkloads({
        ...baseFilterParams,
        guests,
        statusMode: 'degraded',
      });

      // '' -> toLowerCase '' -> not DEGRADED, !== 'running', not OFFLINE -> degraded.
      expect(result.map((g) => g.id)).toEqual(['empty']);
    });

    it('matches status case-sensitively in running mode (capital "Running" is excluded)', () => {
      const guests = [
        makeGuest(1, { id: 'capital', name: 'capital', status: 'Running' }),
        makeGuest(2, { id: 'lower', name: 'lower', status: 'running' }),
      ];

      const result = filterWorkloads({
        ...baseFilterParams,
        guests,
        statusMode: 'running',
      });

      // g.status === 'running' is an exact, case-sensitive comparison.
      expect(result.map((g) => g.id)).toEqual(['lower']);
    });

    it('matches the numeric vmid and string status/instance text-search candidates', () => {
      const guests = [
        makeGuest(1, { id: 'g1', name: 'numhost', vmid: 7777, status: 'running' }),
        makeGuest(2, { id: 'g2', name: 'stathost', vmid: 1, status: 'quarantined' }),
        makeGuest(3, {
          id: 'g3',
          name: 'insthost',
          vmid: 2,
          status: 'running',
          instance: 'insttoken',
        }),
      ];

      // vmid is a number candidate -> filtered into the String(value) match.
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: '7777' }).map((g) => g.id),
      ).toEqual(['g1']);
      // status candidate.
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'quarantined' }).map(
          (g) => g.id,
        ),
      ).toEqual(['g2']);
      // instance candidate.
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'insttoken' }).map((g) => g.id),
      ).toEqual(['g3']);
    });
  });

  describe('matchesWorkloadTextSearch (exercised via filterWorkloads)', () => {
    it('excludes a guest whose only matching candidate is an empty platformScopes array', () => {
      // platformScopes is joined with ' ' -> '' for an empty array; the type
      // guard inside matchesWorkloadTextSearch filters non-string/number values,
      // and an empty joined string never satisfies .includes(needle).
      const guests = [
        makeGuest(1, {
          id: 'scope-1',
          name: 'alpha',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: [],
          status: 'running',
        }),
        makeGuest(2, {
          id: 'scope-2',
          name: 'beta',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: ['uniquematch'],
          status: 'running',
        }),
      ];

      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'uniquematch' }).map(
          (g) => g.id,
        ),
      ).toEqual(['scope-2']);
    });
  });

  describe('getWorkloadGroupLabel', () => {
    it('returns empty type and the raw context for a vm guest with no node and no cluster', () => {
      const guests = [
        makeGuest(1, {
          id: 'solo',
          name: 'solo',
          type: 'vm',
          workloadType: 'vm',
          instance: 'solo',
          node: '',
          clusterName: '',
        }),
      ];

      // groupKey = 'solo-' (no colon) -> prefix not recognized -> first guest has
      // no node and no cluster -> final fallback { type: '', name: context }.
      expect(getWorkloadGroupLabel('', guests)).toStrictEqual({ type: '', name: 'solo-' });
    });

    it('joins a multi-segment context after a recognized prefix for an empty guests array', () => {
      // normalizedGroupKey = groupKey; split(':') -> ['pod', 'ctx', 'extra'];
      // rest joins back to 'ctx:extra'.
      expect(getWorkloadGroupLabel('pod:ctx:extra', [])).toStrictEqual({
        type: 'Pods',
        name: 'ctx:extra',
      });
    });
  });

  describe('computeWorkloadStats', () => {
    it('classifies empty and non-standard statuses as degraded via the second condition', () => {
      const guests = [
        makeGuest(1, { id: 'empty', name: 'empty', status: '', type: 'vm', workloadType: 'vm' }),
        makeGuest(2, {
          id: 'custom',
          name: 'custom',
          status: 'quarantined',
          type: 'vm',
          workloadType: 'vm',
        }),
        makeGuest(3, {
          id: 'running',
          name: 'running',
          status: 'running',
          type: 'vm',
          workloadType: 'vm',
        }),
        makeGuest(4, {
          id: 'offline',
          name: 'offline',
          status: 'offline',
          type: 'vm',
          workloadType: 'vm',
        }),
      ];

      // '' and 'quarantined' are neither DEGRADED nor OFFLINE nor running, so the
      // (status !== 'running' && !OFFLINE) second condition counts them degraded.
      expect(computeWorkloadStats(guests)).toStrictEqual({
        total: 4,
        running: 1,
        degraded: 2,
        stopped: 1,
        vms: 4,
        containers: 0,
        appContainers: 0,
        pods: 0,
      });
    });

    it('classifies a capitalized "Running" status as stopped due to case-sensitive running check', () => {
      const guests = [
        makeGuest(1, {
          id: 'capital',
          name: 'capital',
          status: 'Running',
          type: 'vm',
          workloadType: 'vm',
        }),
      ];

      // g.status === 'running' is exact; the degraded check lowercases, so
      // 'Running' is neither running nor degraded and lands in stopped.
      expect(computeWorkloadStats(guests)).toStrictEqual({
        total: 1,
        running: 0,
        degraded: 0,
        stopped: 1,
        vms: 1,
        containers: 0,
        appContainers: 0,
        pods: 0,
      });
    });
  });
});
