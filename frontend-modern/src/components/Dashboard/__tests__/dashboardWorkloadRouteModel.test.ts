import { describe, expect, it } from 'vitest';

import type { WorkloadGuest } from '@/types/workloads';
import {
  buildDashboardContainerRuntimeOptions,
  buildDashboardKubernetesContextOptions,
  buildDashboardKubernetesNamespaceOptions,
  buildDashboardWorkloadNodeOptions,
  deserializeDashboardWorkloadViewMode,
} from '../dashboardWorkloadRouteModel';

const makeGuest = (overrides?: Partial<WorkloadGuest>): WorkloadGuest =>
  ({
    id: 'guest-1',
    vmid: 101,
    name: 'guest-1',
    node: 'node-a',
    instance: 'cluster-a',
    status: 'running',
    type: 'vm',
    cpu: 0,
    cpus: 2,
    memory: { total: 1024, used: 256, free: 768, usage: 0.25 },
    disk: { total: 1024, used: 256, free: 768, usage: 0.25 },
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 0,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: new Date().toISOString(),
    workloadType: 'vm',
    ...overrides,
  }) as WorkloadGuest;

describe('dashboardWorkloadRouteModel', () => {
  it('deserializes persisted workload view-mode aliases through the canonical helper', () => {
    expect(deserializeDashboardWorkloadViewMode('all')).toBe('all');
    expect(deserializeDashboardWorkloadViewMode('docker')).toBe('app-container');
    expect(deserializeDashboardWorkloadViewMode('Kubernetes')).toBe('pod');
    expect(deserializeDashboardWorkloadViewMode('invalid')).toBe('all');
    expect(deserializeDashboardWorkloadViewMode(null)).toBe('all');
  });

  it('builds workload node options with instance disambiguation only when needed', () => {
    const options = buildDashboardWorkloadNodeOptions([
      makeGuest({ id: 'vm-a', node: 'node-a', instance: 'cluster-a' }),
      makeGuest({ id: 'vm-b', node: 'node-a', instance: 'cluster-b' }),
      makeGuest({ id: 'vm-c', node: 'node-z', instance: 'cluster-a' }),
      makeGuest({ id: 'pod-a', type: 'pod', workloadType: 'pod', node: 'worker-a' }),
    ]);

    expect(options).toEqual([
      { value: 'cluster-a-node-a', label: 'node-a (cluster-a)' },
      { value: 'cluster-b-node-a', label: 'node-a (cluster-b)' },
      { value: 'cluster-a-node-z', label: 'node-z' },
    ]);
  });

  it('builds kubernetes context and namespace options from canonical pod scope', () => {
    const guests = [
      makeGuest({
        id: 'pod-a',
        type: 'pod',
        workloadType: 'pod',
        contextLabel: 'prod',
        namespace: 'default',
      }),
      makeGuest({
        id: 'pod-b',
        type: 'pod',
        workloadType: 'pod',
        contextLabel: 'prod',
        namespace: 'kube-system',
      }),
      makeGuest({
        id: 'pod-c',
        type: 'pod',
        workloadType: 'pod',
        contextLabel: 'stage',
        namespace: 'default',
      }),
    ];

    expect(buildDashboardKubernetesContextOptions(guests)).toEqual(['prod', 'stage']);
    expect(buildDashboardKubernetesNamespaceOptions(guests, 'prod')).toEqual([
      'default',
      'kube-system',
    ]);
  });
  it('builds container runtime options from canonical app-container guests', () => {
    expect(
      buildDashboardContainerRuntimeOptions([
        makeGuest({
          id: 'docker-a',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'docker',
        }),
        makeGuest({
          id: 'docker-b',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'containerd',
        }),
        makeGuest({
          id: 'docker-c',
          type: 'app-container',
          workloadType: 'app-container',
          containerRuntime: 'docker',
        }),
      ]),
    ).toEqual(['containerd', 'docker']);
  });

  it('builds kubernetes namespace options with the selected context scope applied', () => {
    const guests = [
      makeGuest({
        id: 'pod-a',
        type: 'pod',
        workloadType: 'pod',
        contextLabel: 'prod',
        namespace: 'default',
      }),
      makeGuest({
        id: 'pod-b',
        type: 'pod',
        workloadType: 'pod',
        contextLabel: 'prod',
        namespace: 'kube-system',
      }),
      makeGuest({
        id: 'pod-c',
        type: 'pod',
        workloadType: 'pod',
        contextLabel: 'stage',
        namespace: 'observability',
      }),
    ];

    expect(buildDashboardKubernetesNamespaceOptions(guests, 'prod')).toEqual([
      'default',
      'kube-system',
    ]);
    expect(buildDashboardKubernetesNamespaceOptions(guests, 'stage')).toEqual(['observability']);
  });
});
