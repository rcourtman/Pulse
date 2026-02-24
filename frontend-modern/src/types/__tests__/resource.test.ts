/**
 * Tests for resource type guards and helper functions
 */
import { describe, expect, it } from 'vitest';
import {
  isInfrastructure,
  isWorkload,
  isStorage,
  getDisplayName,
  getCpuPercent,
  getMemoryPercent,
  getDiskPercent,
  type Resource,
  type ResourceType,
} from '@/types/resource';

// Helper to create a minimal resource for testing
function createResource(overrides: Partial<Resource> = {}): Resource {
  return {
    id: 'test-1',
    type: 'vm',
    name: 'test-resource',
    displayName: '',
    platformId: 'platform-1',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'running',
    lastSeen: Date.now(),
    ...overrides,
  };
}

describe('Resource Type Guards', () => {
  describe('isInfrastructure', () => {
    const infrastructureTypes: ResourceType[] = [
      'node',
      'host',
      'docker-host',
      'k8s-node',
      'truenas',
    ];
    const nonInfrastructureTypes: ResourceType[] = [
      'vm',
      'container',
      'docker-container',
      'pod',
      'jail',
      'storage',
    ];

    it.each(infrastructureTypes)('returns true for %s', (type) => {
      const resource = createResource({ type });
      expect(isInfrastructure(resource)).toBe(true);
    });

    it.each(nonInfrastructureTypes)('returns false for %s', (type) => {
      const resource = createResource({ type });
      expect(isInfrastructure(resource)).toBe(false);
    });
  });

  describe('isWorkload', () => {
    const workloadTypes: ResourceType[] = ['vm', 'container', 'docker-container', 'pod', 'jail'];
    const nonWorkloadTypes: ResourceType[] = ['node', 'host', 'docker-host', 'storage', 'pbs'];

    it.each(workloadTypes)('returns true for %s', (type) => {
      const resource = createResource({ type });
      expect(isWorkload(resource)).toBe(true);
    });

    it.each(nonWorkloadTypes)('returns false for %s', (type) => {
      const resource = createResource({ type });
      expect(isWorkload(resource)).toBe(false);
    });
  });

  describe('isStorage', () => {
    const storageTypes: ResourceType[] = ['storage', 'datastore', 'pool', 'dataset'];
    const nonStorageTypes: ResourceType[] = ['vm', 'node', 'container', 'docker-host'];

    it.each(storageTypes)('returns true for %s', (type) => {
      const resource = createResource({ type });
      expect(isStorage(resource)).toBe(true);
    });

    it.each(nonStorageTypes)('returns false for %s', (type) => {
      const resource = createResource({ type });
      expect(isStorage(resource)).toBe(false);
    });
  });
});

describe('Resource Helper Functions', () => {
  describe('getDisplayName', () => {
    it('returns displayName when set', () => {
      const resource = createResource({ name: 'machine-1', displayName: 'Production Server' });
      expect(getDisplayName(resource)).toBe('Production Server');
    });

    it('returns name when displayName is empty', () => {
      const resource = createResource({ name: 'machine-1', displayName: '' });
      expect(getDisplayName(resource)).toBe('machine-1');
    });

    it('returns name when displayName is undefined', () => {
      const resource = createResource({ name: 'machine-1' });
      // Force displayName to be falsy
      (resource as any).displayName = undefined;
      expect(getDisplayName(resource)).toBe('machine-1');
    });
  });

  describe('getCpuPercent', () => {
    it('returns current CPU value when available', () => {
      const resource = createResource({ cpu: { current: 75.5 } });
      expect(getCpuPercent(resource)).toBe(75.5);
    });

    it('returns 0 when cpu is undefined', () => {
      const resource = createResource({});
      expect(getCpuPercent(resource)).toBe(0);
    });

    it('returns 0 when cpu.current is undefined', () => {
      const resource = createResource({ cpu: {} as any });
      expect(getCpuPercent(resource)).toBe(0);
    });
  });

  describe('getMemoryPercent', () => {
    it('calculates percentage from used/total when available', () => {
      const resource = createResource({
        memory: { current: 0, total: 1000, used: 250 },
      });
      expect(getMemoryPercent(resource)).toBe(25);
    });

    it('returns current when used/total not available', () => {
      const resource = createResource({
        memory: { current: 45.5 },
      });
      expect(getMemoryPercent(resource)).toBe(45.5);
    });

    it('returns 0 when memory is undefined', () => {
      const resource = createResource({});
      expect(getMemoryPercent(resource)).toBe(0);
    });

    it('handles zero total gracefully', () => {
      const resource = createResource({
        memory: { current: 50, total: 0, used: 0 },
      });
      // When total is 0 (falsy), it should fall back to current
      expect(getMemoryPercent(resource)).toBe(50);
    });
  });

  describe('getDiskPercent', () => {
    it('calculates percentage from used/total when available', () => {
      const resource = createResource({
        disk: { current: 0, total: 1000000000, used: 500000000 },
      });
      expect(getDiskPercent(resource)).toBe(50);
    });

    it('returns current when used/total not available', () => {
      const resource = createResource({
        disk: { current: 80.2 },
      });
      expect(getDiskPercent(resource)).toBe(80.2);
    });

    it('returns 0 when disk is undefined', () => {
      const resource = createResource({});
      expect(getDiskPercent(resource)).toBe(0);
    });
  });
});

describe('Resource Interface', () => {
  it('allows all valid resource types', () => {
    const types: ResourceType[] = [
      'node',
      'host',
      'docker-host',
      'k8s-node',
      'truenas',
      'vm',
      'container',
      'docker-container',
      'pod',
      'jail',
      'docker-service',
      'k8s-deployment',
      'k8s-service',
      'storage',
      'datastore',
      'pool',
      'dataset',
      'pbs',
      'pmg',
    ];

    types.forEach((type) => {
      const resource = createResource({ type });
      expect(resource.type).toBe(type);
    });
  });

  it('supports hierarchy with parentId and clusterId', () => {
    const vm = createResource({
      type: 'vm',
      parentId: 'node-1',
      clusterId: 'pve-cluster-1',
    });

    expect(vm.parentId).toBe('node-1');
    expect(vm.clusterId).toBe('pve-cluster-1');
  });

  it('supports tags and labels', () => {
    const resource = createResource({
      tags: ['production', 'web'],
      labels: { env: 'prod', role: 'frontend' },
    });

    expect(resource.tags).toEqual(['production', 'web']);
    expect(resource.labels).toEqual({ env: 'prod', role: 'frontend' });
  });

  it('supports alerts array', () => {
    const resource = createResource({
      alerts: [
        {
          id: 'alert-1',
          type: 'cpu',
          level: 'warning',
          message: 'High CPU usage',
          value: 85,
          threshold: 80,
          startTime: Date.now(),
        },
      ],
    });

    expect(resource.alerts).toHaveLength(1);
    expect(resource.alerts![0].type).toBe('cpu');
  });

  it('supports identity for deduplication', () => {
    const resource = createResource({
      identity: {
        hostname: 'server-1',
        machineId: 'abc-123',
        ips: ['192.168.1.10', '10.0.0.5'],
      },
    });

    expect(resource.identity?.hostname).toBe('server-1');
    expect(resource.identity?.machineId).toBe('abc-123');
    expect(resource.identity?.ips).toHaveLength(2);
  });

  it('supports platformData for type-specific data', () => {
    const dockerContainer = createResource({
      type: 'docker-container',
      platformData: {
        image: 'nginx:latest',
        health: 'healthy',
        restartCount: 0,
        ports: [{ hostPort: 8080, containerPort: 80 }],
      },
    });

    expect((dockerContainer.platformData as any).image).toBe('nginx:latest');
  });
});
