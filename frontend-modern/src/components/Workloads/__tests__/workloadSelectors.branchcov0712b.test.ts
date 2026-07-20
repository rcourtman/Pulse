import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import {
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

describe('workloadSelectors (branch coverage 2)', () => {
  describe('createWorkloadSortComparator', () => {
    it('returns null for an empty sortKey (guard arm)', () => {
      expect(createWorkloadSortComparator('', 'asc')).toBeNull();
      expect(createWorkloadSortComparator('', 'desc')).toBeNull();
    });

    it('scales cpu by 100 and toggles asc/desc on the numeric branch', () => {
      const guests = [
        makeGuest(1, { id: 'high', name: 'high', cpu: 0.8 }),
        makeGuest(2, { id: 'low', name: 'low', cpu: 0.2 }),
      ];
      const asc = createWorkloadSortComparator('cpu', 'asc')!;
      const desc = createWorkloadSortComparator('cpu', 'desc')!;
      // cpu*100: low=20 < high=80
      expect([...guests].sort(asc).map((g) => g.id)).toEqual(['low', 'high']);
      expect([...guests].sort(desc).map((g) => g.id)).toEqual(['high', 'low']);
    });

    it('treats null memory as 0 via the `a.memory ? ... : 0` ternary false arm', () => {
      const guests = [
        makeGuest(1, {
          id: 'null-mem',
          name: 'null-mem',
          memory: null as unknown as WorkloadGuest['memory'],
        }),
        makeGuest(2, {
          id: 'big-mem',
          name: 'big-mem',
          memory: { total: 100, used: 80, free: 20, usage: 0.8 },
        }),
      ];
      const asc = createWorkloadSortComparator('memory', 'asc')!;
      // null-mem -> 0 < 0.8
      expect([...guests].sort(asc).map((g) => g.id)).toEqual(['null-mem', 'big-mem']);
    });

    it('coerces memory.usage of 0 to 0 via the `|| 0` fallback and tiebreaks equals', () => {
      const guests = [
        makeGuest(1, {
          id: 'b',
          name: 'beta',
          memory: { total: 100, used: 0, free: 100, usage: 0 },
        }),
        makeGuest(2, {
          id: 'a',
          name: 'alpha',
          memory: { total: 100, used: 0, free: 100, usage: 0 },
        }),
        makeGuest(3, {
          id: 'c',
          name: 'gamma',
          memory: { total: 100, used: 50, free: 50, usage: 0.5 },
        }),
      ];
      const asc = createWorkloadSortComparator('memory', 'asc')!;
      // a,b both 0 -> equal numeric -> tiebreak (alpha<beta); c=0.5 last
      expect([...guests].sort(asc).map((g) => g.id)).toEqual(['a', 'b', 'c']);
    });

    it('clamps negative diskRead/diskWrite to 0 on the diskIo branch', () => {
      const guests = [
        makeGuest(1, { id: 'neg', name: 'neg', diskRead: -40, diskWrite: -20 }),
        makeGuest(2, { id: 'pos', name: 'pos', diskRead: 100, diskWrite: 50 }),
      ];
      const asc = createWorkloadSortComparator('diskIo', 'asc')!;
      // neg -> max(0,-40)+max(0,-20)=0; pos -> 150
      expect([...guests].sort(asc).map((g) => g.id)).toEqual(['neg', 'pos']);
    });

    it('sums netIo with the max(0,...) clamp in both directions', () => {
      const guests = [
        makeGuest(1, { id: 'busy', name: 'busy', networkIn: 200, networkOut: 100 }),
        makeGuest(2, { id: 'quiet', name: 'quiet', networkIn: 5, networkOut: 3 }),
      ];
      expect(
        [...guests].sort(createWorkloadSortComparator('netIo', 'asc')!).map((g) => g.id),
      ).toEqual(['quiet', 'busy']);
      expect(
        [...guests].sort(createWorkloadSortComparator('netIo', 'desc')!).map((g) => g.id),
      ).toEqual(['busy', 'quiet']);
    });

    it('routes a both-empty disk pair through tiebreak and orders empties last in both directions', () => {
      const emptyDisk = { total: 0, used: 50, free: 0, usage: NaN } as WorkloadGuest['disk'];
      const guests = [
        makeGuest(1, { id: 'zz', name: 'zebra', disk: emptyDisk }),
        makeGuest(2, { id: 'aa', name: 'alpha', disk: emptyDisk }),
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
      // getDiskUsagePercent(emptyDisk) -> total 0 -> null -> isEmpty. Non-empty sort first;
      // empty pair -> aIsEmpty && bIsEmpty -> tiebreak (alpha<zebra).
      expect(
        [...guests].sort(createWorkloadSortComparator('disk', 'asc')!).map((g) => g.id),
      ).toEqual(['mm', 'bb', 'aa', 'zz']);
      // desc only flips the non-empty comparison; empties still sort last by name asc.
      expect(
        [...guests].sort(createWorkloadSortComparator('disk', 'desc')!).map((g) => g.id),
      ).toEqual(['bb', 'mm', 'aa', 'zz']);
    });

    it('pushes a single empty disk value to the end via the aIsEmpty/bIsEmpty arms', () => {
      const guests = [
        makeGuest(1, {
          id: 'empty',
          name: 'empty',
          disk: undefined as unknown as WorkloadGuest['disk'],
        }),
        makeGuest(2, {
          id: 'full',
          name: 'full',
          disk: { total: 100, used: 50, free: 50, usage: 0.5 },
        }),
      ];
      // empty -> getDiskUsagePercent null -> aIsEmpty -> return 1 (sorts last) in both dirs.
      expect(
        [...guests].sort(createWorkloadSortComparator('disk', 'asc')!).map((g) => g.id),
      ).toEqual(['full', 'empty']);
      expect(
        [...guests].sort(createWorkloadSortComparator('disk', 'desc')!).map((g) => g.id),
      ).toEqual(['full', 'empty']);
    });

    it('uses the generic-key else branch for a string field with asc/desc', () => {
      const guests = [
        makeGuest(1, { id: 'a', name: 'alpha', status: 'stopped' }),
        makeGuest(2, { id: 'b', name: 'beta', status: 'running' }),
      ];
      // 'running' < 'stopped'
      expect(
        [...guests].sort(createWorkloadSortComparator('status', 'asc')!).map((g) => g.id),
      ).toEqual(['b', 'a']);
      expect(
        [...guests].sort(createWorkloadSortComparator('status', 'desc')!).map((g) => g.id),
      ).toEqual(['a', 'b']);
    });

    it('hits the string-equal tiebreak when a generic string key matches on both sides', () => {
      const guests = [
        makeGuest(1, { id: 'b-id', name: 'beta', status: 'running' }),
        makeGuest(2, { id: 'a-id', name: 'alpha', status: 'running' }),
      ];
      // status 'running' === 'running' -> aStr===bStr -> tiebreak (alpha<beta).
      expect(
        [...guests].sort(createWorkloadSortComparator('status', 'asc')!).map((g) => g.id),
      ).toEqual(['a-id', 'b-id']);
    });

    it('treats an empty-string generic value as empty and sorts it last', () => {
      const guests = [
        makeGuest(1, { id: 'empty', name: 'empty', status: '' }),
        makeGuest(2, { id: 'full', name: 'full', status: 'running' }),
      ];
      // aVal '' -> aIsEmpty -> sorts last.
      expect(
        [...guests].sort(createWorkloadSortComparator('status', 'asc')!).map((g) => g.id),
      ).toEqual(['full', 'empty']);
    });

    it('returns 0 only when numeric, name, and id are all equal', () => {
      const a = makeGuest(1, { id: 'dup', name: 'same', cpu: 0.5 });
      const b = makeGuest(2, { id: 'dup', name: 'same', cpu: 0.5 });
      const cmp = createWorkloadSortComparator('cpu', 'asc')!;
      expect(cmp(a, b)).toBe(0);
      expect(cmp(b, a)).toBe(0);
    });
  });

  describe('tiebreak (exercised via createWorkloadSortComparator)', () => {
    it('compares lowercased names and returns -1/1 for nameA</>nameB', () => {
      const alpha = makeGuest(1, { id: 'x', name: 'alpha', cpu: 0.5 });
      const beta = makeGuest(2, { id: 'x', name: 'Beta', cpu: 0.5 });
      const cmp = createWorkloadSortComparator('cpu', 'asc')!;
      // equal cpu -> tiebreak; lowercased 'alpha' < 'beta' regardless of original case.
      expect(cmp(alpha, beta)).toBe(-1);
      expect(cmp(beta, alpha)).toBe(1);
    });

    it('falls back to id comparison (all three arms) when names are equal', () => {
      const a = makeGuest(1, { id: 'a-id', name: 'same', cpu: 0.5 });
      const b = makeGuest(2, { id: 'b-id', name: 'same', cpu: 0.5 });
      const c = makeGuest(3, { id: 'a-id', name: 'same', cpu: 0.5 });
      const cmp = createWorkloadSortComparator('cpu', 'asc')!;
      expect(cmp(a, b)).toBe(-1); // a.id < b.id
      expect(cmp(b, a)).toBe(1); // b.id > a.id
      expect(cmp(a, c)).toBe(0); // ids equal -> 0
    });

    it('coerces an undefined name to "" via the `(a.name || "")` guard', () => {
      const a = makeGuest(1, {
        id: 'b-id',
        name: undefined as unknown as WorkloadGuest['name'],
        cpu: 0.5,
      });
      const b = makeGuest(2, {
        id: 'a-id',
        name: undefined as unknown as WorkloadGuest['name'],
        cpu: 0.5,
      });
      const cmp = createWorkloadSortComparator('cpu', 'asc')!;
      // both names -> "" (equal) -> id compare: a-id < b-id.
      expect(cmp(a, b)).toBe(1);
      expect(cmp(b, a)).toBe(-1);
    });
  });

  describe('filterWorkloads', () => {
    it('returns the input array identity when no filters are active', () => {
      const guests = [makeGuest(1), makeGuest(2)];
      expect(filterWorkloads({ ...baseFilterParams, guests })).toBe(guests);
    });

    it('skips the node-scope filter in pod view so pods on other nodes survive', () => {
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
        }),
      ];
      // workloadHostScopeId(pod) === '' would drop both; viewMode==='pod' skips the guard.
      expect(
        filterWorkloads({
          ...baseFilterParams,
          guests,
          viewMode: 'pod',
          selectedNode: 'worker-a',
        }).map((g) => g.id),
      ).toEqual(['pod-a', 'pod-b']);
    });

    it('filters by workloadHostScopeId when nodeScope is set in a non-pod view', () => {
      const guests = [
        makeGuest(1, {
          id: 'vm-a',
          type: 'vm',
          workloadType: 'vm',
          instance: 'cluster-a',
          node: 'node-a',
        }),
        makeGuest(2, {
          id: 'vm-b',
          type: 'vm',
          workloadType: 'vm',
          instance: 'cluster-b',
          node: 'node-b',
        }),
      ];
      expect(
        filterWorkloads({
          ...baseFilterParams,
          guests,
          selectedNode: 'cluster-a-node-a',
        }).map((g) => g.id),
      ).toEqual(['vm-a']);
    });

    it('gives nodeScope precedence over hostHint (hostHint filter skipped when nodeScope set)', () => {
      const guests = [
        makeGuest(1, {
          id: 'vm-a',
          type: 'vm',
          workloadType: 'vm',
          instance: 'cluster-a',
          node: 'node-a',
        }),
        makeGuest(2, {
          id: 'app-a',
          name: 'redis',
          type: 'app-container',
          workloadType: 'app-container',
          contextLabel: 'edge-host',
          node: '',
          instance: '',
        }),
      ];
      // nodeScope set -> hostHint branch (`!nodeScope`) is skipped; only vm-a matches the scope.
      expect(
        filterWorkloads({
          ...baseFilterParams,
          guests,
          selectedNode: 'cluster-a-node-a',
          selectedHostHint: 'edge',
        }).map((g) => g.id),
      ).toEqual(['vm-a']);
    });

    it('excludes pods and keeps matching app-containers in the hostHint branch', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-x',
          name: 'api',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx',
          node: 'worker',
          contextLabel: 'edge-host',
          namespace: 'default',
        }),
        makeGuest(2, {
          id: 'app-x',
          name: 'redis',
          type: 'app-container',
          workloadType: 'app-container',
          contextLabel: 'edge-host',
          node: '',
          instance: '',
        }),
      ];
      // pod -> resolveWorkloadType==='pod' -> false; app-container candidate matches 'edge'.
      expect(
        filterWorkloads({ ...baseFilterParams, guests, selectedHostHint: 'edge' }).map((g) => g.id),
      ).toEqual(['app-x']);
    });

    it('skips the hostHint filter when viewMode is pod', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-a',
          name: 'api',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx',
          node: 'worker',
          contextLabel: 'ctx',
          namespace: 'default',
        }),
      ];
      // hostHint set but viewMode==='pod' -> `viewMode !== 'pod'` false -> skipped; pod kept.
      expect(
        filterWorkloads({
          ...baseFilterParams,
          guests,
          viewMode: 'pod',
          selectedHostHint: 'nope',
        }).map((g) => g.id),
      ).toEqual(['pod-a']);
    });

    it('filters pods by kubernetes context key in pod view', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-a',
          name: 'api',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx',
          node: 'worker',
          contextLabel: 'prod-context',
          namespace: 'default',
        }),
        makeGuest(2, {
          id: 'pod-b',
          name: 'web',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx',
          node: 'worker',
          contextLabel: 'stage-context',
          namespace: 'default',
        }),
      ];
      expect(
        filterWorkloads({
          ...baseFilterParams,
          guests,
          viewMode: 'pod',
          selectedKubernetesContext: 'prod-context',
        }).map((g) => g.id),
      ).toEqual(['pod-a']);
    });

    it('skips the kubernetes context filter when viewMode is not pod', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-a',
          name: 'api',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx',
          node: 'worker',
          contextLabel: 'prod-context',
          namespace: 'default',
        }),
        makeGuest(2, { id: 'vm-a', type: 'vm', workloadType: 'vm', instance: 'i', node: 'n' }),
      ];
      // viewMode 'all' -> k8s context guard skipped -> both remain.
      expect(
        filterWorkloads({
          ...baseFilterParams,
          guests,
          selectedKubernetesContext: 'prod-context',
        }).map((g) => g.id),
      ).toEqual(['pod-a', 'vm-a']);
    });

    it('drops non-pod guests during kubernetes namespace filtering in pod view', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-a',
          name: 'api',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx',
          node: 'worker',
          contextLabel: 'ctx',
          namespace: 'payments',
        }),
        makeGuest(2, { id: 'vm-a', type: 'vm', workloadType: 'vm', namespace: 'payments' }),
      ];
      // vm -> resolveWorkloadType !== 'pod' -> return false in the namespace filter.
      expect(
        filterWorkloads({
          ...baseFilterParams,
          guests,
          viewMode: 'pod',
          selectedKubernetesNamespace: 'payments',
        }).map((g) => g.id),
      ).toEqual(['pod-a']);
    });

    it('filters vms by cluster name in vm view', () => {
      const guests = [
        makeGuest(1, {
          id: 'vm-a',
          type: 'vm',
          workloadType: 'vm',
          instance: 'i',
          node: 'n',
          clusterName: 'prod-cluster',
        }),
        makeGuest(2, {
          id: 'vm-b',
          type: 'vm',
          workloadType: 'vm',
          instance: 'i',
          node: 'n',
          clusterName: 'dev-cluster',
        }),
      ];
      expect(
        filterWorkloads({
          ...baseFilterParams,
          guests,
          viewMode: 'vm',
          selectedCluster: 'prod-cluster',
        }).map((g) => g.id),
      ).toEqual(['vm-a']);
    });

    it('skips the cluster filter when viewMode is not vm', () => {
      const guests = [
        makeGuest(1, {
          id: 'vm-a',
          type: 'vm',
          workloadType: 'vm',
          instance: 'i',
          node: 'n',
          clusterName: 'prod-cluster',
        }),
        makeGuest(2, {
          id: 'vm-b',
          type: 'vm',
          workloadType: 'vm',
          instance: 'i',
          node: 'n',
          clusterName: 'dev-cluster',
        }),
      ];
      // viewMode 'all' -> cluster guard skipped -> both remain.
      expect(
        filterWorkloads({ ...baseFilterParams, guests, selectedCluster: 'prod-cluster' }).map(
          (g) => g.id,
        ),
      ).toEqual(['vm-a', 'vm-b']);
    });

    it('keeps only container-typed workloads under the combined container view mode', () => {
      const guests = [
        makeGuest(1, {
          id: 'app',
          type: 'app-container',
          workloadType: 'app-container',
          contextLabel: 'h',
        }),
        makeGuest(2, {
          id: 'lxc',
          type: 'lxc',
          workloadType: 'system-container',
          instance: 'i',
          node: 'n',
        }),
        makeGuest(3, { id: 'vm', type: 'vm', workloadType: 'vm', instance: 'i', node: 'n' }),
        makeGuest(4, {
          id: 'pod',
          type: 'pod',
          workloadType: 'pod',
          instance: 'c',
          node: 'w',
          contextLabel: 'c',
          namespace: 'd',
        }),
      ];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, viewMode: 'container' }).map((g) => g.id),
      ).toEqual(['app', 'lxc']);
    });

    it('skips the platform filter when the normalized platform is "all"', () => {
      const guests = [
        makeGuest(1, {
          id: 'tn',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: ['truenas'],
          contextLabel: 'h',
        }),
        makeGuest(2, {
          id: 'dk',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: ['docker'],
          contextLabel: 'h',
        }),
      ];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, selectedPlatform: 'all' }).map((g) => g.id),
      ).toEqual(['tn', 'dk']);
    });

    it('applies workloadMatchesPlatformScope for a concrete platform', () => {
      const guests = [
        makeGuest(1, {
          id: 'tn',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: ['truenas'],
          contextLabel: 'h',
        }),
        makeGuest(2, {
          id: 'dk',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: ['docker'],
          contextLabel: 'h',
        }),
      ];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, selectedPlatform: 'truenas' }).map(
          (g) => g.id,
        ),
      ).toEqual(['tn']);
    });

    it('drops system-containers in the runtime filter under the combined container view', () => {
      const guests = [
        makeGuest(1, {
          id: 'app-1',
          name: 'redis',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'docker',
          contextLabel: 'h',
          node: '',
          instance: '',
        }),
        makeGuest(2, {
          id: 'lxc-1',
          name: 'db',
          type: 'lxc',
          workloadType: 'system-container',
          instance: 'i',
          node: 'n',
          containerRuntime: 'docker',
        }),
      ];
      // 'container' view keeps both; runtime filter drops the system-container
      // (resolveWorkloadType !== 'app-container' -> return false).
      expect(
        filterWorkloads({
          ...baseFilterParams,
          guests,
          viewMode: 'container',
          containerRuntime: 'docker',
        }).map((g) => g.id),
      ).toEqual(['app-1']);
    });

    it('skips the containerRuntime filter when viewMode is not a container mode', () => {
      const guests = [
        makeGuest(1, {
          id: 'c1',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'docker',
          contextLabel: 'h',
        }),
        makeGuest(2, {
          id: 'c2',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'podman',
          contextLabel: 'h',
        }),
      ];
      // isContainerWorkloadViewMode('all') === false -> filter skipped.
      expect(
        filterWorkloads({ ...baseFilterParams, guests, containerRuntime: 'docker' }).map(
          (g) => g.id,
        ),
      ).toEqual(['c1', 'c2']);
    });

    it('matches status exactly (case-sensitive) in running mode', () => {
      const guests = [
        makeGuest(1, { id: 'lower', status: 'running' }),
        makeGuest(2, { id: 'capital', status: 'Running' }),
      ];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, statusMode: 'running' }).map((g) => g.id),
      ).toEqual(['lower']);
    });

    it('counts DEGRADED-set and unknown statuses as degraded, excluding running and OFFLINE-set', () => {
      const guests = [
        makeGuest(1, { id: 'warn', status: 'warning' }), // DEGRADED set
        makeGuest(2, { id: 'migrating', status: 'migrating' }), // unknown -> second condition
        makeGuest(3, { id: 'stopped', status: 'stopped' }), // OFFLINE set -> excluded
        makeGuest(4, { id: 'running', status: 'running' }), // running -> excluded
        makeGuest(5, { id: 'empty', status: '' }), // '' -> unknown -> second condition
      ];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, statusMode: 'degraded' }).map((g) => g.id),
      ).toEqual(['warn', 'migrating', 'empty']);
    });

    it('treats capitalized "Running" as stopped due to the case-sensitive !== running check', () => {
      const guests = [
        makeGuest(1, { id: 'capital', status: 'Running' }),
        makeGuest(2, { id: 'lower', status: 'running' }),
        makeGuest(3, { id: 'stopped', status: 'stopped' }),
      ];
      // stopped mode keeps g.status !== 'running' -> 'Running' survives (case quirk).
      expect(
        filterWorkloads({ ...baseFilterParams, guests, statusMode: 'stopped' }).map((g) => g.id),
      ).toEqual(['capital', 'stopped']);
    });

    it('applies no status filter for an unrecognized statusMode (else arm)', () => {
      const guests = [
        makeGuest(1, { id: 'a', status: 'running' }),
        makeGuest(2, { id: 'b', status: 'stopped' }),
      ];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, statusMode: 'all' }).map((g) => g.id),
      ).toEqual(['a', 'b']);
    });

    it('parses a ">" metric filter part and evaluates it via evaluateFilterStack', () => {
      const guests = [
        makeGuest(1, { id: 'high', name: 'high', cpu: 0.8 }),
        makeGuest(2, { id: 'low', name: 'low', cpu: 0.5 }),
      ];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'cpu>70' }).map((g) => g.id),
      ).toEqual(['high']);
    });

    it('parses a "<" metric filter part (the includes("<") arm of the filter-char detection)', () => {
      const guests = [
        makeGuest(1, { id: 'high', name: 'high', cpu: 0.8 }),
        makeGuest(2, { id: 'low', name: 'low', cpu: 0.2 }),
      ];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'cpu<50' }).map((g) => g.id),
      ).toEqual(['low']);
    });

    it('parses a ":" text filter part (the includes(":") arm of the filter-char detection)', () => {
      const guests = [
        makeGuest(1, { id: 'prod', name: 'prod-api' }),
        makeGuest(2, { id: 'dev', name: 'dev-api' }),
      ];
      // 'name:prod' -> text condition { field: 'name', value: 'prod' }.
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'name:prod' }).map((g) => g.id),
      ).toEqual(['prod']);
    });

    it('hides rows matching a "-term" exclusion while keeping comma-separated OR text', () => {
      const guests = [
        makeGuest(1, { id: 'a', name: 'alpha' }),
        makeGuest(2, { id: 'b', name: 'beta-worker' }),
        makeGuest(3, { id: 'c', name: 'gamma' }),
      ];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: '-worker' }).map((g) => g.id),
      ).toEqual(['a', 'c']);
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'alpha, gamma' }).map(
          (g) => g.id,
        ),
      ).toEqual(['a', 'c']);
    });

    it('combines metric filter, exclusion, and OR text search in one pass', () => {
      const guests = [
        makeGuest(1, { id: 'g1', name: 'alpha-api', cpu: 0.8 }),
        makeGuest(2, { id: 'g2', name: 'beta-worker', cpu: 0.9 }),
        makeGuest(3, { id: 'g3', name: 'alpha-big', cpu: 0.95 }),
        makeGuest(4, { id: 'g4', name: 'alpha-low', cpu: 0.5 }),
      ];
      // cpu>70 keeps g1,g2,g3; -beta drops g2; text "alpha" keeps g1,g3.
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'cpu>70, alpha, -beta' }).map(
          (g) => g.id,
        ),
      ).toEqual(['g1', 'g3']);
    });

    it('treats a whitespace-only searchTerm as no search and returns the input identity', () => {
      const guests = [makeGuest(1), makeGuest(2)];
      expect(filterWorkloads({ ...baseFilterParams, guests, searchTerm: '   ' })).toBe(guests);
    });
  });

  describe('matchesWorkloadTextSearch (exercised via filterWorkloads)', () => {
    it('matches a numeric vmid candidate through the string|number type guard', () => {
      const guests = [
        makeGuest(1, { id: 'g1', name: 'host-a', vmid: 7777 }),
        makeGuest(2, { id: 'g2', name: 'host-b', vmid: 1 }),
      ];
      // vmid is a number -> passes guard -> String(7777).includes('7777').
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: '7777' }).map((g) => g.id),
      ).toEqual(['g1']);
    });

    it('matches a string status candidate case-insensitively', () => {
      const guests = [
        makeGuest(1, { id: 'g1', name: 'a', status: 'quarantined' }),
        makeGuest(2, { id: 'g2', name: 'b', status: 'running' }),
      ];
      // needle lowercased; candidate lowercased -> 'QUARANTINED' still matches.
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'QUARANTINED' }).map(
          (g) => g.id,
        ),
      ).toEqual(['g1']);
    });

    it('does not match a guest whose only candidate is an empty joined platformScopes string', () => {
      const guests = [
        makeGuest(1, {
          id: 'empty',
          name: 'alpha',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: [],
        }),
        makeGuest(2, {
          id: 'scope',
          name: 'beta',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: ['uniquematch'],
        }),
      ];
      // [].join(' ') -> '' -> String('').includes('uniquematch') is false.
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'uniquematch' }).map(
          (g) => g.id,
        ),
      ).toEqual(['scope']);
    });

    it('matches a multi-element platformScopes candidate joined with a space', () => {
      const guests = [
        makeGuest(1, {
          id: 'multi',
          name: 'gamma',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: ['foo', 'bar-baz'],
        }),
      ];
      // join -> 'foo bar-baz' -> includes('bar-baz').
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'bar-baz' }).map((g) => g.id),
      ).toEqual(['multi']);
    });

    it('returns no guests when no candidate contains the needle', () => {
      const guests = [makeGuest(1, { id: 'g1', name: 'alpha', vmid: 1 })];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'zzzz-nope' }).map((g) => g.id),
      ).toEqual([]);
    });

    it('matches against the image, namespace, and contextLabel candidates', () => {
      const guests = [
        makeGuest(1, {
          id: 'img',
          name: 'a',
          type: 'pod',
          workloadType: 'pod',
          image: 'registry.local/api:v2',
          namespace: 'default',
          contextLabel: 'ctx',
          instance: 'ctx',
          node: 'w',
        }),
        makeGuest(2, {
          id: 'ns',
          name: 'b',
          type: 'pod',
          workloadType: 'pod',
          namespace: 'payments-uniq',
          contextLabel: 'ctx',
          instance: 'ctx',
          node: 'w',
        }),
        makeGuest(3, {
          id: 'ctx',
          name: 'c',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'contextlabel-uniq',
          namespace: 'd',
          instance: 'ctx',
          node: 'w',
        }),
      ];
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'registry.local' }).map(
          (g) => g.id,
        ),
      ).toEqual(['img']);
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'payments-uniq' }).map(
          (g) => g.id,
        ),
      ).toEqual(['ns']);
      expect(
        filterWorkloads({ ...baseFilterParams, guests, searchTerm: 'contextlabel-uniq' }).map(
          (g) => g.id,
        ),
      ).toEqual(['ctx']);
    });
  });
});
