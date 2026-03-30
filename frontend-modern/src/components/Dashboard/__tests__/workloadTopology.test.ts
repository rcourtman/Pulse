import { describe, expect, it } from 'vitest';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import {
  buildGuestParentNodeMap,
  buildNodeByInstance,
  getDiscoveryHostIdForWorkload,
  getWorkloadContainerHostId,
  getDiscoveryResourceIdForWorkload,
  getKubernetesContextKey,
  getWorkloadDockerHostId,
  workloadNodeScopeId,
} from '@/components/Dashboard/workloadTopology';

const makeGuest = (i: number, overrides?: Partial<WorkloadGuest>): WorkloadGuest => ({
  id: `guest-${i}`,
  vmid: 100 + i,
  name: `workload-${i}`,
  node: `node-${i % 5}`,
  instance: `cluster-${i % 3}`,
  status: i % 7 === 0 ? 'stopped' : 'running',
  type: i % 4 === 0 ? 'lxc' : i % 3 === 0 ? 'app-container' : 'vm',
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
  workloadType: (i % 4 === 0 ? 'lxc' : i % 3 === 0 ? 'app-container' : 'vm') as any,
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

describe('workloadTopology', () => {
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

  describe('workload identity helpers', () => {
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
    it('keeps Docker action hosts explicit and uses broader host fallback for discovery', () => {
      const dockerWithHostId = makeGuest(1, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: 'docker-host-1',
        node: 'node-a',
        instance: 'inst-a',
      });
      const dockerFallback = makeGuest(2, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: '',
        node: 'node-b',
        instance: 'inst-b',
      });
      const truenasApp = makeGuest(3, {
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'truenas',
        dockerHostId: '',
        node: 'truenas-main',
        instance: 'truenas-main',
      });
      expect(getWorkloadDockerHostId(dockerWithHostId)).toBe('docker-host-1');
      expect(getWorkloadDockerHostId(dockerFallback)).toBe('');
      expect(getWorkloadDockerHostId(truenasApp)).toBe('');
      expect(getWorkloadContainerHostId(dockerFallback)).toBe('node-b');
      expect(getWorkloadContainerHostId(truenasApp)).toBe('truenas-main');
    });

    it('maps discovery host and resource IDs for app-container, pod, and vm', () => {
      const docker = makeGuest(1, {
        id: 'app-container:docker-host-1:container-abc123',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'docker',
        dockerHostId: 'docker-host-1',
        containerId: 'container-abc123',
      });
      const truenas = makeGuest(2, {
        id: 'app-container:truenas-main:nextcloud',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'truenas',
        dockerHostId: '',
        node: 'truenas-main',
        instance: 'truenas-main',
      });
      const truenasWithExplicitDiscovery = makeGuest(2, {
        id: 'app-container:truenas-main:nextcloud',
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'truenas',
        dockerHostId: '',
        discoveryTarget: {
          resourceType: 'app-container',
          agentId: 'truenas-helper',
          resourceId: 'nextcloud',
        },
      });
      const k8s = makeGuest(3, {
        id: 'k8s:cluster-a:pod:pod-uid-1',
        type: 'pod',
        workloadType: 'pod',
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
      expect(getDiscoveryResourceIdForWorkload(docker)).toBe(
        'app-container:docker-host-1:container-abc123',
      );
      expect(getDiscoveryHostIdForWorkload(truenas)).toBe('');
      expect(getDiscoveryResourceIdForWorkload(truenas)).toBe('');
      expect(getDiscoveryHostIdForWorkload(truenasWithExplicitDiscovery)).toBe('truenas-helper');
      expect(getDiscoveryResourceIdForWorkload(truenasWithExplicitDiscovery)).toBe('nextcloud');

      expect(getDiscoveryHostIdForWorkload(k8s)).toBe('k8s-agent-1');
      expect(getDiscoveryResourceIdForWorkload(k8s)).toBe('pod-uid-1');

      expect(getDiscoveryHostIdForWorkload(vm)).toBe('pve1');
      expect(getDiscoveryResourceIdForWorkload(vm)).toBe('103');
    });
  });
});
