import { describe, expect, it } from 'vitest';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import {
  buildWorkloadSummaryGroupScope,
  buildWorkloadSummaryGroupScopeMap,
  createWorkloadSortComparator,
  filterWorkloads,
  getDiskUsagePercent,
  getWorkloadGroupKey,
  getWorkloadGroupLabel,
  groupWorkloads,
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

const makeNode = (id: string, instance: string, name: string): Node => ({
  id,
  name,
  displayName: name,
  instance,
  host: `${name}.local`,
  status: 'online',
  type: 'pve',
  cpu: 0,
  memory: { total: 1, used: 0, free: 1, usage: 0 },
  disk: { total: 1, used: 0, free: 1, usage: 0 },
  uptime: 1,
  loadAverage: [0, 0, 0],
  kernelVersion: 'test',
  pveVersion: 'test',
  cpuInfo: { model: 'test', cores: 1, sockets: 1, mhz: '1' },
  lastSeen: new Date().toISOString(),
  connectionHealth: 'online',
});

describe('workloadSelectors (branch coverage)', () => {
  describe('createWorkloadSortComparator', () => {
    it('sorts by memory usage in both directions and falls back to 0 for null memory', () => {
      const guests = [
        makeGuest(1, {
          id: 'high',
          name: 'high-mem',
          memory: { total: 100, used: 80, free: 20, usage: 0.8 },
        }),
        makeGuest(2, {
          id: 'low',
          name: 'low-mem',
          memory: { total: 100, used: 10, free: 90, usage: 0.1 },
        }),
        makeGuest(3, {
          id: 'null',
          name: 'null-mem',
          memory: null as unknown as WorkloadGuest['memory'],
        }),
      ];

      const memAsc = createWorkloadSortComparator('memory', 'asc');
      const memDesc = createWorkloadSortComparator('memory', 'desc');

      expect(memAsc).not.toBeNull();
 expect(memDesc).not.toBeNull();

      // null-mem (0) < low-mem (0.1) < high-mem (0.8)
      expect([...guests].sort(memAsc!).map((g) => g.id)).toEqual(['null', 'low', 'high']);
      expect([...guests].sort(memDesc!).map((g) => g.id)).toEqual(['high', 'low', 'null']);
    });

    it('treats memory.usage of 0 as 0 via the || fallback', () => {
      const guests = [
        makeGuest(1, { id: 'a', name: 'alpha', cpu: 0.5, memory: { total: 100, used: 0, free: 100, usage: 0 } }),
        makeGuest(2, { id: 'b', name: 'beta', cpu: 0.5, memory: { total: 100, used: 50, free: 50, usage: 0.5 } }),
      ];

      const memAsc = createWorkloadSortComparator('memory', 'asc');
      expect([...guests].sort(memAsc!).map((g) => g.id)).toEqual(['a', 'b']);
    });

    it('sorts by disk usage percent via getDiskUsagePercent', () => {
      const guests = [
        makeGuest(1, { id: 'big', name: 'big-disk', disk: { total: 100, used: 90, free: 10, usage: 0.9 } }),
        makeGuest(2, { id: 'small', name: 'small-disk', disk: { total: 100, used: 10, free: 90, usage: 0.1 } }),
      ];

      const diskAsc = createWorkloadSortComparator('disk', 'asc');
      const diskDesc = createWorkloadSortComparator('disk', 'desc');

      expect([...guests].sort(diskAsc!).map((g) => g.id)).toEqual(['small', 'big']);
      expect([...guests].sort(diskDesc!).map((g) => g.id)).toEqual(['big', 'small']);
    });

    it('sorts by diskIo and clamps negative read/write values to 0', () => {
      const guests = [
        makeGuest(1, { id: 'positive', name: 'p', diskRead: 100, diskWrite: 50 }),
        makeGuest(2, { id: 'negative', name: 'n', diskRead: -40, diskWrite: -20 }),
      ];

      const ioAsc = createWorkloadSortComparator('diskIo', 'asc');
      // negative -> max(0,-40)+max(0,-20) = 0; positive -> 150
      expect([...guests].sort(ioAsc!).map((g) => g.id)).toEqual(['negative', 'positive']);
    });

    it('sorts by netIo in both directions', () => {
      const guests = [
        makeGuest(1, { id: 'busy', name: 'busy-net', networkIn: 200, networkOut: 100 }),
        makeGuest(2, { id: 'quiet', name: 'quiet-net', networkIn: 5, networkOut: 3 }),
      ];

      const netAsc = createWorkloadSortComparator('netIo', 'asc');
      const netDesc = createWorkloadSortComparator('netIo', 'desc');

      expect([...guests].sort(netAsc!).map((g) => g.id)).toEqual(['quiet', 'busy']);
      expect([...guests].sort(netDesc!).map((g) => g.id)).toEqual(['busy', 'quiet']);
    });

    it('sorts by a generic string key (status) via the else branch', () => {
      const guests = [
        makeGuest(1, { id: 'a', name: 'alpha', status: 'stopped' }),
        makeGuest(2, { id: 'b', name: 'beta', status: 'running' }),
      ];

      const statusAsc = createWorkloadSortComparator('status', 'asc');
      // 'running' < 'stopped' alphabetically
      expect([...guests].sort(statusAsc!).map((g) => g.id)).toEqual(['b', 'a']);
    });
  });

  describe('filterWorkloads', () => {
    it('statusMode running keeps only guests whose status is exactly running', () => {
      const guests = [
        makeGuest(1, { id: 'r1', status: 'running', type: 'vm', workloadType: 'vm' }),
        makeGuest(2, { id: 'r2', status: 'warning', type: 'vm', workloadType: 'vm' }),
        makeGuest(3, { id: 'r3', status: 'stopped', type: 'vm', workloadType: 'vm' }),
      ];

      const result = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'running',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });

      expect(result.map((g) => g.id)).toEqual(['r1']);
    });

    it('filters pods by kubernetes namespace in pod view mode', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-a',
          name: 'api',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx-a',
          node: 'worker-1',
          contextLabel: 'ctx-a',
          namespace: 'payments',
          status: 'running',
        }),
        makeGuest(2, {
          id: 'pod-b',
          name: 'web',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx-a',
          node: 'worker-2',
          contextLabel: 'ctx-a',
          namespace: 'default',
          status: 'running',
        }),
      ];

      const result = filterWorkloads({
        guests,
        viewMode: 'pod',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
        selectedKubernetesNamespace: 'payments',
      });

      expect(result.map((g) => g.id)).toEqual(['pod-a']);
    });

    it('filters vms by cluster name in vm view mode', () => {
      const guests = [
        makeGuest(1, {
          id: 'vm-a',
          name: 'vm-a',
          type: 'vm',
          workloadType: 'vm',
          instance: 'inst-a',
          node: 'node-a',
          clusterName: 'prod-cluster',
        }),
        makeGuest(2, {
          id: 'vm-b',
          name: 'vm-b',
          type: 'vm',
          workloadType: 'vm',
          instance: 'inst-b',
          node: 'node-b',
          clusterName: 'dev-cluster',
        }),
      ];

      const result = filterWorkloads({
        guests,
        viewMode: 'vm',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
        selectedCluster: 'prod-cluster',
      });

      expect(result.map((g) => g.id)).toEqual(['vm-a']);
    });

    it('filters app-containers by runtime in container view mode', () => {
      const guests = [
        makeGuest(1, {
          id: 'c-docker',
          name: 'redis',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'docker',
          contextLabel: 'host-a',
          node: '',
          instance: '',
        }),
        makeGuest(2, {
          id: 'c-podman',
          name: 'nginx',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'podman',
          contextLabel: 'host-b',
          node: '',
          instance: '',
        }),
      ];

      const result = filterWorkloads({
        guests,
        viewMode: 'app-container',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
        containerRuntime: 'docker',
      });

      expect(result.map((g) => g.id)).toEqual(['c-docker']);
    });

    it('does not apply containerRuntime filter when viewMode is not a container mode', () => {
      const guests = [
        makeGuest(1, {
          id: 'c1',
          name: 'redis',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'docker',
          contextLabel: 'host-a',
        }),
        makeGuest(2, {
          id: 'c2',
          name: 'nginx',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'podman',
          contextLabel: 'host-b',
        }),
      ];

      const result = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
        containerRuntime: 'docker',
      });

      // Both returned: isContainerWorkloadViewMode('all') is false
      expect(result.map((g) => g.id)).toEqual(['c1', 'c2']);
    });

    it('excludes pods from host-hint filtering even when contextLabel matches', () => {
      const guests = [
        makeGuest(1, {
          id: 'pod-x',
          name: 'api-pod',
          type: 'pod',
          workloadType: 'pod',
          instance: 'ctx',
          node: 'worker',
          contextLabel: 'edge-host',
          namespace: 'default',
          status: 'running',
        }),
        makeGuest(2, {
          id: 'docker-x',
          name: 'redis',
          type: 'app-container',
          workloadType: 'app-container',
          contextLabel: 'edge-host',
          node: '',
          instance: '',
          status: 'running',
        }),
      ];

      const result = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: 'edge',
        selectedKubernetesContext: null,
      });

      // Pod excluded (resolveWorkloadType === 'pod' -> false), app-container kept
      expect(result.map((g) => g.id)).toEqual(['docker-x']);
    });

    it('returns all guests unchanged when search term is only commas and spaces', () => {
      const guests = [makeGuest(1), makeGuest(2)];

      const result = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: '  ,  ,  ',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });

      expect(result).toBe(guests);
    });

    it('matches workload text search across displayId and clusterName candidates', () => {
      const guests = [
        makeGuest(1, {
          id: 'g1',
          name: 'alpha',
          displayId: 'display-uniq-001',
          type: 'vm',
          workloadType: 'vm',
          status: 'running',
        }),
        makeGuest(2, {
          id: 'g2',
          name: 'beta',
          clusterName: 'cluster-uniq-002',
          type: 'vm',
          workloadType: 'vm',
          status: 'running',
        }),
        makeGuest(3, {
          id: 'g3',
          name: 'gamma',
          type: 'vm',
          workloadType: 'vm',
          status: 'running',
        }),
      ];

      const byDisplay = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: 'display-uniq-001',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });
      expect(byDisplay.map((g) => g.id)).toEqual(['g1']);

      const byCluster = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: 'cluster-uniq-002',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });
      expect(byCluster.map((g) => g.id)).toEqual(['g2']);
    });

    it('matches workload text search across platformScopes, dockerHostId, kubernetesAgentId, and containerId candidates', () => {
      const guests = [
        makeGuest(1, {
          id: 'scope-guest',
          name: 'sc',
          type: 'app-container',
          workloadType: 'app-container',
          platformScopes: ['truenas', 'scope-uniq-aaa'],
          status: 'running',
        }),
        makeGuest(2, {
          id: 'docker-guest',
          name: 'dk',
          type: 'app-container',
          workloadType: 'app-container',
          dockerHostId: 'host-uniq-bbb',
          status: 'running',
        }),
        makeGuest(3, {
          id: 'k8s-guest',
          name: 'k8',
          type: 'pod',
          workloadType: 'pod',
          kubernetesAgentId: 'agent-uniq-ccc',
          instance: 'ctx',
          node: 'worker',
          contextLabel: 'ctx',
          namespace: 'default',
          status: 'running',
        }),
        makeGuest(4, {
          id: 'cid-guest',
          name: 'cd',
          type: 'app-container',
          workloadType: 'app-container',
          containerId: 'containerid-uniq-ddd',
          status: 'running',
        }),
      ];

      const baseParams = {
        viewMode: 'all' as const,
        statusMode: 'all',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      };

      expect(
        filterWorkloads({ ...baseParams, guests, searchTerm: 'scope-uniq-aaa' }).map((g) => g.id),
      ).toEqual(['scope-guest']);

      expect(
        filterWorkloads({ ...baseParams, guests, searchTerm: 'host-uniq-bbb' }).map((g) => g.id),
      ).toEqual(['docker-guest']);

      expect(
        filterWorkloads({ ...baseParams, guests, searchTerm: 'agent-uniq-ccc' }).map((g) => g.id),
      ).toEqual(['k8s-guest']);

      expect(
        filterWorkloads({ ...baseParams, guests, searchTerm: 'containerid-uniq-ddd' }).map((g) => g.id),
      ).toEqual(['cid-guest']);
    });
  });

  describe('getDiskUsagePercent', () => {
    it('treats usage of exactly 1 as a ratio (100) not a percentage', () => {
      expect(
        getDiskUsagePercent(makeGuest(1, { disk: { total: 100, used: 100, free: 0, usage: 1 } })),
      ).toBe(100);
    });

    it('returns 0 for usage of 0 and clamps negative usage to 0', () => {
      expect(
        getDiskUsagePercent(makeGuest(1, { disk: { total: 100, used: 0, free: 100, usage: 0 } })),
      ).toBe(0);
      expect(
        getDiskUsagePercent(makeGuest(2, { disk: { total: 100, used: 0, free: 100, usage: -5 } })),
      ).toBe(0);
    });

    it('falls back to used/total when usage is non-finite (Infinity)', () => {
      expect(
        getDiskUsagePercent(
          makeGuest(1, { disk: { total: 200, used: 50, free: 150, usage: Infinity } }),
        ),
      ).toBe(25);
    });

    it('clamps used/total result above 100 to 100', () => {
      expect(
        getDiskUsagePercent(
          makeGuest(1, { disk: { total: 100, used: 150, free: 0, usage: NaN } }),
        ),
      ).toBe(100);
    });

    it('returns null when total is 0 in the used/total fallback', () => {
      expect(
        getDiskUsagePercent(
          makeGuest(1, { disk: { total: 0, used: 50, free: 0, usage: NaN } }),
        ),
      ).toBeNull();
    });

    it('returns null when usage is non-numeric and used/total is invalid', () => {
      expect(
        getDiskUsagePercent(
          makeGuest(1, {
            disk: {
              total: 0,
              used: 0,
              free: 0,
              usage: 'n/a' as unknown as number,
            },
          }),
        ),
      ).toBeNull();
    });

    it('returns null when guest or disk is null/undefined', () => {
      expect(getDiskUsagePercent(null as unknown as WorkloadGuest)).toBeNull();
      expect(getDiskUsagePercent(undefined as unknown as WorkloadGuest)).toBeNull();
      expect(
        getDiskUsagePercent(
          makeGuest(1, { disk: undefined as unknown as WorkloadGuest['disk'] }),
        ),
      ).toBeNull();
    });
  });

  describe('getWorkloadGroupKey', () => {
    it('uses instance-node for system-container type', () => {
      const guest = makeGuest(1, {
        type: 'lxc',
        workloadType: 'system-container',
        instance: 'inst-1',
        node: 'node-1',
      });
      expect(getWorkloadGroupKey(guest)).toBe('inst-1-node-1');
    });

    it('falls back through contextLabel -> node -> instance -> namespace -> id', () => {
      // contextLabel empty, node empty -> instance
      const byInstance = makeGuest(1, {
        id: 'fallback-1',
        type: 'app-container',
        workloadType: 'app-container',
        contextLabel: '',
        node: '',
        instance: 'inst-only',
        namespace: 'ns-only',
      });
      expect(getWorkloadGroupKey(byInstance)).toBe('app-container:inst-only');

      // contextLabel + node + instance empty -> namespace
      const byNamespace = makeGuest(2, {
        id: 'fallback-2',
        type: 'app-container',
        workloadType: 'app-container',
        contextLabel: '',
        node: '',
        instance: '',
        namespace: 'ns-only',
      });
      expect(getWorkloadGroupKey(byNamespace)).toBe('app-container:ns-only');

      // everything empty -> id
      const byId = makeGuest(3, {
        id: 'last-resort-id',
        type: 'app-container',
        workloadType: 'app-container',
        contextLabel: '',
        node: '',
        instance: '',
        namespace: '',
      });
      expect(getWorkloadGroupKey(byId)).toBe('app-container:last-resort-id');
    });
  });

  describe('getWorkloadGroupLabel', () => {
    it('returns plural label for vm prefix when guests array is empty', () => {
      expect(getWorkloadGroupLabel('vm:ctx-vm', [])).toEqual({ type: 'VMs', name: 'ctx-vm' });
    });

    it('returns plural label for system-container prefix when guests array is empty', () => {
      expect(getWorkloadGroupLabel('system-container:ctx-lxc', [])).toEqual({
        type: 'LXC',
        name: 'ctx-lxc',
      });
    });

    it('returns plural label for pod prefix when guests array is empty', () => {
      expect(getWorkloadGroupLabel('pod:ctx-pod', [])).toEqual({ type: 'Pods', name: 'ctx-pod' });
    });

    it('uses normalizedNodeName and groupTypeLabel when provided', () => {
      const guests = [makeGuest(1, { type: 'vm', workloadType: 'vm' })];
      expect(getWorkloadGroupLabel('any-key', guests, '  my-host  ', '  HostBadge  ')).toEqual({
        type: 'HostBadge',
        name: 'my-host',
      });
    });

    it('returns cluster as type and node as name for vm guest with both', () => {
      const guests = [
        makeGuest(1, {
          type: 'vm',
          workloadType: 'vm',
          instance: 'inst-a',
          node: 'pve1',
          clusterName: 'my-cluster',
        }),
      ];
      // groupKey = 'inst-a-pve1' (no colon) -> prefix not recognized -> first guest fallback
      expect(getWorkloadGroupLabel('', guests)).toEqual({ type: 'my-cluster', name: 'pve1' });
    });

    it('returns empty type and node name for vm guest with node but no cluster', () => {
      const guests = [
        makeGuest(1, {
          type: 'vm',
          workloadType: 'vm',
          instance: 'inst-a',
          node: 'pve2',
          clusterName: '',
        }),
      ];
      expect(getWorkloadGroupLabel('', guests)).toEqual({ type: '', name: 'pve2' });
    });

    it('falls through to context-only when guests empty and prefix is unrecognized', () => {
      expect(getWorkloadGroupLabel('plainkey', [])).toEqual({ type: '', name: 'plainkey' });
    });
  });

  describe('buildWorkloadSummaryGroupScope', () => {
    it('returns null when all guests produce empty canonical IDs', () => {
      const guest = makeGuest(1, { id: '', type: 'app-container', workloadType: 'app-container' });
      expect(buildWorkloadSummaryGroupScope('grp', [guest], { type: 'T', name: 'N' })).toBeNull();
    });

    it('builds singular label for a single guest', () => {
      const guest = makeGuest(1, {
        id: 'cid-1',
        type: 'app-container',
        workloadType: 'app-container',
      });
      const scope = buildWorkloadSummaryGroupScope('grp', [guest], { type: 'Container', name: 'frigate' });
      expect(scope).not.toBeNull();
      expect(scope!.label).toBe('frigate · Container (1 workload)');
      expect(scope!.seriesIds).toEqual(['cid-1']);
    });

    it('builds plural label and deduplicates canonical IDs for multiple guests', () => {
      const guests = [
        makeGuest(1, { id: 'dup-id', type: 'app-container', workloadType: 'app-container' }),
        makeGuest(2, { id: 'dup-id', type: 'app-container', workloadType: 'app-container' }),
      ];
      const scope = buildWorkloadSummaryGroupScope('grp', guests, { type: 'C', name: 'host' });
      expect(scope).not.toBeNull();
      expect(scope!.label).toBe('host · C (2 workloads)');
      expect(scope!.seriesIds).toEqual(['dup-id']); // deduped
    });

    it('uses workload count alone when label name and type are both empty', () => {
      const guest = makeGuest(1, { id: 'x-1', type: 'app-container', workloadType: 'app-container' });
      const scope = buildWorkloadSummaryGroupScope('grp', [guest], { type: '', name: '' });
      expect(scope).not.toBeNull();
      expect(scope!.label).toBe('1 workload');
    });

    it('trims the groupId for the scope id', () => {
      const guest = makeGuest(1, { id: 'x-2', type: 'app-container', workloadType: 'app-container' });
      const scope = buildWorkloadSummaryGroupScope('  padded-grp  ', [guest], { type: 'T', name: 'N' });
      expect(scope).not.toBeNull();
      expect(scope!.id).toBe('padded-grp');
    });
  });

  describe('buildWorkloadSummaryGroupScopeMap', () => {
    it('returns an empty map when groupingMode is flat', () => {
      const result = buildWorkloadSummaryGroupScopeMap({
        guests: [makeGuest(1)],
        nodes: [],
        groupingMode: 'flat',
        sortComparator: null,
      });
      expect(result.size).toBe(0);
    });

    it('resolves node display name via buildNodeByInstance for the group label', () => {
      const guests = [
        makeGuest(1, { type: 'vm', workloadType: 'vm', instance: 'cluster-a', node: 'node-a' }),
      ];
      const nodes = [makeNode('n1', 'cluster-a', 'node-a')];

      const result = buildWorkloadSummaryGroupScopeMap({
        guests,
        nodes,
        groupingMode: 'grouped',
        sortComparator: null,
      });

      // groupKey = 'cluster-a-node-a' matches node via instance-name key
      // getNodeDisplayName -> 'node-a' -> label includes it
      const scope = result.get('cluster-a-node-a');
      expect(scope).toBeDefined();
      expect(scope!.label).toBe('node-a (1 workload)');
      expect(scope!.seriesIds).toEqual(['cluster-a:node-a:101']);
    });

    it('falls back to lowercase badge key when exact-case lookup misses', () => {
      const guests = [
        makeGuest(1, {
          id: 'c-1',
          name: 'frigate',
          type: 'app-container',
          workloadType: 'app-container',
          contextLabel: 'Frigate', // capital F -> groupKey 'app-container:Frigate'
        }),
      ];

      const result = buildWorkloadSummaryGroupScopeMap({
        guests,
        nodes: [],
        groupingMode: 'grouped',
        sortComparator: null,
        groupLabelBadges: {
          'app-container:frigate': { label: 'Docker' }, // lowercase key
        },
      });

      // badge found via lowercase fallback -> label includes 'Docker'
      const scope = result.get('app-container:Frigate');
      expect(scope).toBeDefined();
      expect(scope!.label).toBe('Frigate · Docker (1 workload)');
    });

    it('omits groups whose scope is null (empty canonical IDs)', () => {
      const guests = [
        makeGuest(1, { id: '', type: 'app-container', workloadType: 'app-container' }),
      ];

      const result = buildWorkloadSummaryGroupScopeMap({
        guests,
        nodes: [],
        groupingMode: 'grouped',
        sortComparator: null,
      });

      expect(result.size).toBe(0);
    });
  });

  describe('groupWorkloads', () => {
    it('sorts the single flat group when a comparator is provided', () => {
      const guests = [
        makeGuest(1, { id: 'b-id', name: 'beta', instance: 'x', node: 'y', type: 'vm', workloadType: 'vm' }),
        makeGuest(2, { id: 'a-id', name: 'alpha', instance: 'x', node: 'y', type: 'vm', workloadType: 'vm' }),
      ];

      const comparator = createWorkloadSortComparator('name', 'asc');
      const result = groupWorkloads(guests, 'flat', comparator);

      expect(Object.keys(result)).toEqual(['']);
      expect(result[''].map((g) => g.id)).toEqual(['a-id', 'b-id']);
    });

    it('groups without sorting when comparator is null in grouped mode', () => {
      const guests = [
        makeGuest(1, {
          id: 'b-id',
          name: 'beta',
          type: 'vm',
          workloadType: 'vm',
          instance: 'inst',
          node: 'nd',
        }),
        makeGuest(2, {
          id: 'a-id',
          name: 'alpha',
          type: 'vm',
          workloadType: 'vm',
          instance: 'inst',
          node: 'nd',
        }),
      ];

      const result = groupWorkloads(guests, 'grouped', null);

      expect(Object.keys(result)).toEqual(['inst-nd']);
      // Original insertion order preserved (no comparator)
      expect(result['inst-nd'].map((g) => g.id)).toEqual(['b-id', 'a-id']);
    });

    it('returns an empty object for empty guests in grouped mode', () => {
      const result = groupWorkloads([], 'grouped', null);
      expect(Object.keys(result)).toEqual([]);
    });
  });
});
