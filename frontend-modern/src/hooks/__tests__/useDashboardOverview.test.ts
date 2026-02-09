import { describe, expect, it } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { computeDashboardOverview } from '@/hooks/useDashboardOverview';

const EMPTY_STATUS_COUNTS: Record<string, number> = {
  online: 0,
  offline: 0,
  running: 0,
  stopped: 0,
  degraded: 0,
  paused: 0,
  unknown: 0,
};

function createResource(overrides: Partial<Resource> = {}): Resource {
  return {
    id: 'resource-1',
    type: 'host',
    name: 'resource-1',
    displayName: 'Resource 1',
    platformId: 'platform-1',
    platformType: 'host-agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    ...overrides,
  };
}

function createAlert(overrides: Partial<Alert> = {}): Alert {
  return {
    id: 'alert-1',
    type: 'cpu',
    level: 'warning',
    resourceId: 'resource-1',
    resourceName: 'Resource 1',
    node: 'node-1',
    instance: 'instance-1',
    message: 'CPU high',
    value: 91,
    threshold: 80,
    startTime: '2026-02-01T00:00:00.000Z',
    acknowledged: false,
    ...overrides,
  };
}

describe('computeDashboardOverview', () => {
  it('returns a fully-zeroed summary for empty state', () => {
    const overview = computeDashboardOverview([], []);

    expect(overview.health.totalResources).toBe(0);
    expect(overview.health.byStatus).toEqual(EMPTY_STATUS_COUNTS);
    expect(overview.infrastructure.total).toBe(0);
    expect(overview.infrastructure.byStatus).toEqual(EMPTY_STATUS_COUNTS);
    expect(overview.infrastructure.byType).toEqual({});
    expect(overview.infrastructure.topCPU).toEqual([]);
    expect(overview.infrastructure.topMemory).toEqual([]);
    expect(overview.workloads).toEqual({
      total: 0,
      running: 0,
      stopped: 0,
      byType: {},
    });
    expect(overview.storage).toEqual({
      total: 0,
      totalCapacity: 0,
      totalUsed: 0,
      warningCount: 0,
      criticalCount: 0,
    });
    expect(overview.alerts).toEqual({
      activeCritical: 0,
      activeWarning: 0,
      total: 0,
    });
  });

  it('handles infrastructure-only resources and returns top CPU/memory lists', () => {
    const resources: Resource[] = [
      createResource({ id: 'infra-1', type: 'host', displayName: 'Host 1', status: 'online', cpu: { current: 10 }, memory: { current: 40 } }),
      createResource({ id: 'infra-2', type: 'node', displayName: 'Node 2', status: 'online', cpu: { current: 90 }, memory: { current: 70 } }),
      createResource({ id: 'infra-3', type: 'k8s-node', displayName: 'K8S 3', status: 'degraded', cpu: { current: 50 }, memory: { current: 60 } }),
      createResource({ id: 'infra-4', type: 'docker-host', displayName: 'Docker 4', status: 'offline', cpu: { current: 30 }, memory: { current: 10 } }),
      createResource({ id: 'infra-5', type: 'truenas', displayName: 'NAS 5', status: 'online', cpu: { current: 70 }, memory: { current: 50 } }),
      createResource({ id: 'infra-6', type: 'k8s-cluster', displayName: 'Cluster 6', status: 'online', cpu: { current: 20 }, memory: { current: 80 } }),
    ];

    const overview = computeDashboardOverview(resources, []);

    expect(overview.infrastructure.total).toBe(6);
    expect(overview.infrastructure.byType).toEqual({
      host: 1,
      node: 1,
      'k8s-node': 1,
      'docker-host': 1,
      truenas: 1,
      'k8s-cluster': 1,
    });
    expect(overview.infrastructure.byStatus.online).toBe(4);
    expect(overview.infrastructure.byStatus.offline).toBe(1);
    expect(overview.infrastructure.byStatus.degraded).toBe(1);

    expect(overview.infrastructure.topCPU).toHaveLength(5);
    expect(overview.infrastructure.topCPU.map((item) => item.id)).toEqual([
      'infra-2',
      'infra-5',
      'infra-3',
      'infra-4',
      'infra-6',
    ]);
    expect(overview.infrastructure.topMemory.map((item) => item.id)).toEqual([
      'infra-6',
      'infra-2',
      'infra-3',
      'infra-5',
      'infra-1',
    ]);

    expect(overview.workloads.total).toBe(0);
    expect(overview.storage.total).toBe(0);
  });

  it('handles workloads-only resources and computes running/stopped counts', () => {
    const resources: Resource[] = [
      createResource({ id: 'wl-1', type: 'vm', status: 'running' }),
      createResource({ id: 'wl-2', type: 'container', status: 'online' }),
      createResource({ id: 'wl-3', type: 'docker-container', status: 'stopped' }),
      createResource({ id: 'wl-4', type: 'pod', status: 'offline' }),
    ];

    const overview = computeDashboardOverview(resources, []);

    expect(overview.workloads.total).toBe(4);
    expect(overview.workloads.running).toBe(2);
    expect(overview.workloads.stopped).toBe(2);
    expect(overview.workloads.byType).toEqual({
      vm: 1,
      container: 1,
      'docker-container': 1,
      pod: 1,
    });
    expect(overview.infrastructure.total).toBe(0);
    expect(overview.storage.total).toBe(0);
  });

  it('handles full mixed resources including storage thresholds and byte totals', () => {
    const resources: Resource[] = [
      createResource({
        id: 'infra-a',
        type: 'host',
        status: 'online',
        cpu: { current: 25 },
        memory: { current: 55 },
      }),
      createResource({
        id: 'workload-a',
        type: 'vm',
        status: 'running',
      }),
      createResource({
        id: 'storage-warning',
        type: 'dataset',
        status: 'online',
        disk: { current: 85, total: 1_000, used: 850 },
      }),
      createResource({
        id: 'storage-critical',
        type: 'pool',
        status: 'degraded',
        disk: { current: 95, total: 2_000, used: 1_950 },
      }),
      createResource({
        id: 'storage-no-disk',
        type: 'storage',
        status: 'unknown',
      }),
    ];

    const overview = computeDashboardOverview(resources, []);

    expect(overview.health.totalResources).toBe(5);
    expect(overview.health.byStatus.online).toBe(2);
    expect(overview.health.byStatus.running).toBe(1);
    expect(overview.health.byStatus.degraded).toBe(1);
    expect(overview.health.byStatus.unknown).toBe(1);

    expect(overview.infrastructure.total).toBe(1);
    expect(overview.workloads.total).toBe(1);
    expect(overview.storage.total).toBe(3);
    expect(overview.storage.totalCapacity).toBe(3_000);
    expect(overview.storage.totalUsed).toBe(2_800);
    expect(overview.storage.warningCount).toBe(1);
    expect(overview.storage.criticalCount).toBe(1);
  });

  it('counts alerts by severity for both health and alerts sections', () => {
    const alerts: Alert[] = [
      createAlert({ id: 'a-1', level: 'critical' }),
      createAlert({ id: 'a-2', level: 'warning' }),
      createAlert({ id: 'a-3', level: 'critical' }),
    ];

    const overview = computeDashboardOverview([], alerts);

    expect(overview.alerts).toEqual({
      activeCritical: 2,
      activeWarning: 1,
      total: 3,
    });
    expect(overview.health.criticalAlerts).toBe(2);
    expect(overview.health.warningAlerts).toBe(1);
  });
});
