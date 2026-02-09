import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { computeDashboardOverview } from '@/hooks/useDashboardOverview';

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

describe('Dashboard panels data contract', () => {
  it('computes infrastructure and workloads summary data for dashboard panels', () => {
    const resources: Resource[] = [
      createResource({
        id: 'infra-1',
        type: 'host',
        name: 'host-alpha',
        displayName: 'Host Alpha',
        status: 'online',
        cpu: { current: 95 },
      }),
      createResource({
        id: 'infra-2',
        type: 'node',
        name: 'node-beta',
        displayName: 'Node Beta',
        status: 'offline',
        cpu: { current: 92 },
      }),
      createResource({
        id: 'infra-3',
        type: 'k8s-node',
        name: 'k8s-gamma',
        displayName: 'K8s Gamma',
        status: 'online',
        cpu: { current: 84 },
      }),
      createResource({
        id: 'infra-4',
        type: 'docker-host',
        name: 'docker-delta',
        displayName: 'Docker Delta',
        status: 'online',
        cpu: { current: 78 },
      }),
      createResource({
        id: 'infra-5',
        type: 'truenas',
        name: 'truenas-epsilon',
        displayName: 'TrueNAS Epsilon',
        status: 'degraded',
        cpu: { current: 65 },
      }),
      createResource({
        id: 'infra-6',
        type: 'k8s-cluster',
        name: 'cluster-zeta',
        displayName: 'Cluster Zeta',
        status: 'online',
        cpu: { current: 51 },
      }),
      createResource({
        id: 'work-1',
        type: 'vm',
        status: 'running',
      }),
      createResource({
        id: 'work-2',
        type: 'container',
        status: 'online',
      }),
      createResource({
        id: 'work-3',
        type: 'docker-container',
        status: 'stopped',
      }),
      createResource({
        id: 'work-4',
        type: 'pod',
        status: 'offline',
      }),
    ];

    const overview = computeDashboardOverview(resources, []);

    expect(overview.infrastructure.total).toBe(6);
    expect(overview.infrastructure.byStatus.online).toBe(4);
    expect(overview.infrastructure.byStatus.offline).toBe(1);
    expect(overview.infrastructure.topCPU).toHaveLength(5);
    expect(overview.infrastructure.topCPU.map((entry) => entry.id)).toEqual([
      'infra-1',
      'infra-2',
      'infra-3',
      'infra-4',
      'infra-5',
    ]);
    expect(overview.infrastructure.topCPU.map((entry) => entry.percent)).toEqual([95, 92, 84, 78, 65]);

    expect(overview.workloads.total).toBe(4);
    expect(overview.workloads.running).toBe(2);
    expect(overview.workloads.stopped).toBe(2);
    expect(overview.workloads.byType).toEqual({
      vm: 1,
      container: 1,
      'docker-container': 1,
      pod: 1,
    });
  });
});
