import { describe, expect, it } from 'vitest';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import {
  buildGuestParentNodeMap,
  buildNodeByInstance,
  computeWorkloadIOEmphasis,
  computeWorkloadStats,
  createWorkloadSortComparator,
  filterWorkloads,
  getDiscoveryHostIdForWorkload,
  getDiscoveryResourceIdForWorkload,
  getDiskUsagePercent,
  getKubernetesContextKey,
  getWorkloadDockerHostId,
  getWorkloadGroupKey,
  groupWorkloads,
  workloadNodeScopeId,
} from '@/components/Dashboard/workloadSelectors';

const makeGuest = (i: number, overrides?: Partial<WorkloadGuest>): WorkloadGuest => ({
  id: `guest-${i}`,
  vmid: 100 + i,
  name: `workload-${i}`,
  node: `node-${i % 5}`,
  instance: `cluster-${i % 3}`,
  status: i % 7 === 0 ? 'stopped' : 'running',
  type: i % 4 === 0 ? 'lxc' : i % 3 === 0 ? 'docker' : 'vm',
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
  workloadType: (i % 4 === 0 ? 'lxc' : i % 3 === 0 ? 'docker' : 'vm') as any,
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

describe('workloadSelectors', () => {
  describe('filterWorkloads', () => {
    it('returns input when no filters are active', () => {
      const guests = [makeGuest(1), makeGuest(2), makeGuest(3)];
      const result = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });

      expect(result).toBe(guests);
    });

    it('filters by view mode and status mode semantics', () => {
      const guests = [
        makeGuest(1, { status: 'running', type: 'vm', workloadType: 'vm' }),
        makeGuest(2, { status: 'warning', type: 'vm', workloadType: 'vm' }),
        makeGuest(3, { status: 'migrating', type: 'vm', workloadType: 'vm' }),
        makeGuest(4, { status: 'offline', type: 'vm', workloadType: 'vm' }),
        makeGuest(5, { status: 'running', type: 'docker', workloadType: 'docker' }),
      ];

      const vmOnly = filterWorkloads({
        guests,
        viewMode: 'vm',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });
      expect(vmOnly).toHaveLength(4);

      const degraded = filterWorkloads({
        guests,
        viewMode: 'vm',
        statusMode: 'degraded',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });
      expect(degraded.map((g) => g.status)).toEqual(['warning', 'migrating']);

      const stopped = filterWorkloads({
        guests,
        viewMode: 'vm',
        statusMode: 'stopped',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });
      expect(stopped.map((g) => g.status)).toEqual(['warning', 'migrating', 'offline']);
    });

    it('applies text and metric search filters, and supports combined filtering', () => {
      const guests = [
        makeGuest(1, { name: 'alpha-api', vmid: 201, cpu: 0.8, node: 'node-a', status: 'running' }),
        makeGuest(2, {
          name: 'beta-worker',
          vmid: 202,
          cpu: 0.25,
          node: 'node-b',
          status: 'running',
        }),
        makeGuest(3, { name: 'gamma-db', vmid: 303, cpu: 0.9, node: 'node-c', status: 'stopped' }),
      ];

      const textOnly = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: 'alpha',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });
      expect(textOnly.map((g) => g.name)).toEqual(['alpha-api']);

      const metricOnly = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: 'cpu>70',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });
      expect(metricOnly.map((g) => g.name)).toEqual(['alpha-api', 'gamma-db']);

      const combined = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'running',
        searchTerm: 'cpu>70, alpha',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: null,
      });
      expect(combined.map((g) => g.name)).toEqual(['alpha-api']);
    });

    it('filters k8s by selected context key and filters non-k8s by host hint', () => {
      const guests = [
        makeGuest(1, {
          name: 'vm-a',
          type: 'vm',
          workloadType: 'vm',
          node: 'node-a',
          instance: 'cluster-a',
          status: 'running',
        }),
        makeGuest(2, {
          name: 'docker-a',
          type: 'docker',
          workloadType: 'docker',
          node: 'docker-node',
          instance: 'docker-cluster',
          contextLabel: 'edge-host',
          status: 'running',
        }),
        makeGuest(3, {
          name: 'k8s-a',
          type: 'k8s',
          workloadType: 'k8s',
          node: 'worker-a',
          instance: 'cluster-prod',
          contextLabel: 'prod-context',
          namespace: 'default',
          status: 'running',
        }),
        makeGuest(4, {
          name: 'k8s-b',
          type: 'k8s',
          workloadType: 'k8s',
          node: 'worker-b',
          instance: 'cluster-stage',
          contextLabel: 'stage-context',
          namespace: 'default',
          status: 'running',
        }),
      ];

      const withHostHint = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: 'EDGE',
        selectedKubernetesContext: null,
      });
      expect(withHostHint.map((g) => g.name)).toEqual(['docker-a']);

      const withNodeScope = filterWorkloads({
        guests,
        viewMode: 'all',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: workloadNodeScopeId(guests[0]),
        selectedHostHint: 'edge',
        selectedKubernetesContext: null,
      });
      expect(withNodeScope.map((g) => g.name)).toEqual(['vm-a']);

      const withK8sContext = filterWorkloads({
        guests,
        viewMode: 'k8s',
        statusMode: 'all',
        searchTerm: '',
        selectedNode: null,
        selectedHostHint: null,
        selectedKubernetesContext: 'prod-context',
      });
      expect(withK8sContext.map((g) => g.name)).toEqual(['k8s-a']);
    });
  });

  describe('createWorkloadSortComparator', () => {
    it('sorts cpu and name with direction toggle', () => {
      const guests = [
        makeGuest(1, { id: 'b', name: 'beta', cpu: 0.5 }),
        makeGuest(2, { id: 'a', name: 'alpha', cpu: 0.5 }),
        makeGuest(3, { id: 'c', name: 'gamma', cpu: 0.2 }),
      ];

      const cpuAsc = createWorkloadSortComparator('cpu', 'asc');
      const cpuDesc = createWorkloadSortComparator('cpu', 'desc');
      const nameDesc = createWorkloadSortComparator('name', 'desc');

      expect(cpuAsc).not.toBeNull();
      expect(cpuDesc).not.toBeNull();
      expect(nameDesc).not.toBeNull();

      expect([...guests].sort(cpuAsc!).map((g) => g.id)).toEqual(['c', 'a', 'b']);
      expect([...guests].sort(cpuDesc!).map((g) => g.id)).toEqual(['a', 'b', 'c']);
      expect([...guests].sort(nameDesc!).map((g) => g.id)).toEqual(['c', 'b', 'a']);
    });

    it('pushes null/empty values to the end and keeps tiebreaker stability', () => {
      const guests = [
        makeGuest(1, { id: 'id-2', name: 'same', cpu: 0.4 }),
        makeGuest(2, { id: 'id-1', name: 'same', cpu: 0.4 }),
        makeGuest(3, { id: 'id-3', name: '', cpu: 0.8 }),
      ];

      const nameAsc = createWorkloadSortComparator('name', 'asc');
      const cpuAsc = createWorkloadSortComparator('cpu', 'asc');

      expect([...guests].sort(nameAsc!).map((g) => g.id)).toEqual(['id-1', 'id-2', 'id-3']);
      expect([...guests].sort(cpuAsc!).map((g) => g.id)).toEqual(['id-1', 'id-2', 'id-3']);
      expect(createWorkloadSortComparator('', 'asc')).toBeNull();
    });
  });

  describe('groupWorkloads', () => {
    it('returns a single group in flat mode', () => {
      const guests = [makeGuest(1), makeGuest(2)];
      const grouped = groupWorkloads(guests, 'flat', null);
      expect(Object.keys(grouped)).toEqual(['']);
      expect(grouped['']).toBe(guests);
    });

    it('groups workloads by key and sorts each group when comparator is provided', () => {
      const guests = [
        makeGuest(1, {
          id: 'vm-b',
          name: 'vm-b',
          type: 'vm',
          workloadType: 'vm',
          instance: 'cluster-a',
          node: 'node-a',
        }),
        makeGuest(2, {
          id: 'vm-a',
          name: 'vm-a',
          type: 'vm',
          workloadType: 'vm',
          instance: 'cluster-a',
          node: 'node-a',
        }),
        makeGuest(3, {
          id: 'docker-1',
          name: 'docker-1',
          type: 'docker',
          workloadType: 'docker',
          contextLabel: 'edge',
        }),
      ];

      const comparator = createWorkloadSortComparator('name', 'asc');
      const grouped = groupWorkloads(guests, 'grouped', comparator);

      expect(Object.keys(grouped).sort()).toEqual(['cluster-a-node-a', 'docker:edge']);
      expect(grouped['cluster-a-node-a'].map((g) => g.id)).toEqual(['vm-a', 'vm-b']);
    });
  });

  describe('computeWorkloadStats', () => {
    it('computes counts by status and type', () => {
      const guests = [
        makeGuest(1, { type: 'vm', workloadType: 'vm', status: 'running' }),
        makeGuest(2, { type: 'vm', workloadType: 'vm', status: 'warning' }),
        makeGuest(3, { type: 'lxc', workloadType: 'system-container', status: 'offline' }),
        makeGuest(4, { type: 'docker', workloadType: 'docker', status: 'running' }),
        makeGuest(5, { type: 'k8s', workloadType: 'k8s', status: 'migrating' }),
      ];

      expect(computeWorkloadStats(guests)).toEqual({
        total: 5,
        running: 2,
        degraded: 2,
        stopped: 1,
        vms: 2,
        containers: 1,
        docker: 1,
        k8s: 1,
      });
    });
  });

  describe('getDiskUsagePercent', () => {
    it('handles ratio and percentage disk usage with clamping', () => {
      expect(
        getDiskUsagePercent(
          makeGuest(1, { disk: { total: 100, used: 42, free: 58, usage: 0.42 } }),
        ),
      ).toBe(42);
      expect(
        getDiskUsagePercent(makeGuest(2, { disk: { total: 100, used: 85, free: 15, usage: 85 } })),
      ).toBe(85);
      expect(
        getDiskUsagePercent(makeGuest(3, { disk: { total: 100, used: 120, free: 0, usage: 120 } })),
      ).toBe(100);
    });

    it('falls back to used/total and returns null for invalid inputs', () => {
      expect(
        getDiskUsagePercent(
          makeGuest(1, { disk: { total: 200, used: 50, free: 150, usage: NaN } }),
        ),
      ).toBe(25);
      expect(
        getDiskUsagePercent(makeGuest(2, { disk: null as unknown as WorkloadGuest['disk'] })),
      ).toBeNull();
    });
  });

  describe('getWorkloadGroupKey', () => {
    it('uses instance-node for vm/lxc, and type:context for docker/k8s', () => {
      const vm = makeGuest(1, {
        type: 'vm',
        workloadType: 'vm',
        instance: 'inst-a',
        node: 'node-a',
      });
      const docker = makeGuest(2, {
        type: 'docker',
        workloadType: 'docker',
        contextLabel: 'docker-edge',
        instance: 'inst-b',
        node: 'node-b',
      });
      const k8s = makeGuest(3, {
        type: 'k8s',
        workloadType: 'k8s',
        contextLabel: '',
        node: 'worker-2',
        instance: 'cluster-z',
      });

      expect(getWorkloadGroupKey(vm)).toBe('inst-a-node-a');
      expect(getWorkloadGroupKey(docker)).toBe('docker:docker-edge');
      expect(getWorkloadGroupKey(k8s)).toBe('k8s:worker-2');
    });
  });

  describe('buildNodeByInstance and buildGuestParentNodeMap', () => {
    it('maps nodes by id and legacy instance-name key without overriding first legacy key', () => {
      const nodeA = makeNode('cluster-a-node-a', 'cluster-a', 'node-a');
      const nodeAAlt = makeNode('custom-node-id', 'cluster-a', 'node-a');
      const nodeB = makeNode('cluster-b-node-b', 'cluster-b', 'node-b');

      const map = buildNodeByInstance([nodeA, nodeAAlt, nodeB]);

      expect(map['cluster-a-node-a']).toBe(nodeA);
      expect(map['custom-node-id']).toBe(nodeAAlt);
      expect(map['cluster-a-node-a']).toBe(nodeA);
      expect(map['cluster-b-node-b']).toBe(nodeB);
    });

    it('builds guest parent node mapping using id lookup first, then composite fallback', () => {
      const nodeA = makeNode('cluster-a-node-a', 'cluster-a', 'node-a');
      const nodeB = makeNode('cluster-b-node-b', 'cluster-b', 'node-b');
      const nodeMap = buildNodeByInstance([nodeA, nodeB]);

      const guestWithIdLookup = makeGuest(1, {
        id: 'cluster-a-node-a-101',
        vmid: 101,
        type: 'vm',
        workloadType: 'vm',
        instance: 'cluster-a',
        node: 'node-a',
      });
      const guestWithFallback = makeGuest(2, {
        id: 'unmatched-id',
        vmid: 102,
        type: 'vm',
        workloadType: 'vm',
        instance: 'cluster-b',
        node: 'node-b',
      });
      const guestWithoutParent = makeGuest(3, {
        id: 'unknown-103',
        vmid: 103,
        type: 'vm',
        workloadType: 'vm',
        instance: 'cluster-c',
        node: 'node-c',
      });

      const mapping = buildGuestParentNodeMap(
        [guestWithIdLookup, guestWithFallback, guestWithoutParent],
        nodeMap,
      );

      expect(mapping['cluster-a:node-a:101']).toBe(nodeA);
      expect(mapping['cluster-b:node-b:102']).toBe(nodeB);
      expect(mapping['cluster-c:node-c:103']).toBeUndefined();
    });
  });

  describe('workloadNodeScopeId and getKubernetesContextKey', () => {
    it('builds node scope as instance-node with trimming', () => {
      const guest = makeGuest(1, { instance: ' cluster-a ', node: ' node-a ' });
      expect(workloadNodeScopeId(guest)).toBe('cluster-a-node-a');
    });

    it('returns first non-empty kubernetes context candidate', () => {
      const guest = makeGuest(1, {
        contextLabel: ' ',
        instance: 'cluster-a',
        node: 'worker-a',
        namespace: 'default',
      });
      expect(getKubernetesContextKey(guest)).toBe('cluster-a');
    });
  });

  describe('workload discovery/action IDs', () => {
    it('prefers docker hostSourceId and falls back to node/instance', () => {
      const dockerWithHostId = makeGuest(1, {
        type: 'docker',
        workloadType: 'docker',
        dockerHostId: 'docker-host-1',
        node: 'node-a',
        instance: 'inst-a',
      });
      const dockerFallback = makeGuest(2, {
        type: 'docker',
        workloadType: 'docker',
        dockerHostId: '',
        node: 'node-b',
        instance: 'inst-b',
      });
      expect(getWorkloadDockerHostId(dockerWithHostId)).toBe('docker-host-1');
      expect(getWorkloadDockerHostId(dockerFallback)).toBe('node-b');
    });

    it('maps discovery host/resource IDs for docker, k8s, and vm', () => {
      const docker = makeGuest(1, {
        id: 'container-abc123',
        type: 'docker',
        workloadType: 'docker',
        dockerHostId: 'docker-host-1',
      });
      const k8s = makeGuest(2, {
        id: 'k8s:cluster-a:pod:pod-uid-1',
        type: 'k8s',
        workloadType: 'k8s',
        kubernetesAgentId: 'k8s-agent-1',
        instance: 'cluster-a',
        node: 'worker-a',
      });
      const vm = makeGuest(3, {
        id: 'vm-resource-hash',
        vmid: 103,
        type: 'vm',
        workloadType: 'vm',
        node: 'pve1',
      });

      expect(getDiscoveryHostIdForWorkload(docker)).toBe('docker-host-1');
      expect(getDiscoveryResourceIdForWorkload(docker)).toBe('container-abc123');

      expect(getDiscoveryHostIdForWorkload(k8s)).toBe('k8s-agent-1');
      expect(getDiscoveryResourceIdForWorkload(k8s)).toBe('pod-uid-1');

      expect(getDiscoveryHostIdForWorkload(vm)).toBe('pve1');
      expect(getDiscoveryResourceIdForWorkload(vm)).toBe('103');
    });
  });

  describe('computeWorkloadIOEmphasis', () => {
    it('computes io distribution from workload network and disk io totals', () => {
      const guests = [
        makeGuest(1, { networkIn: 2, networkOut: 1, diskRead: 3, diskWrite: 4 }),
        makeGuest(2, { networkIn: -10, networkOut: 0, diskRead: -2, diskWrite: 0 }),
      ];

      expect(computeWorkloadIOEmphasis(guests)).toEqual({
        network: { median: 1.5, mad: 1.5, max: 3, p97: 3, p99: 3, count: 2 },
        diskIO: { median: 3.5, mad: 3.5, max: 7, p97: 7, p99: 7, count: 2 },
      });
    });
  });
});
