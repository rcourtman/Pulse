import { describe, expect, it } from 'vitest';

import type { WorkloadGuest } from '@/types/workloads';
import {
  buildWorkloadsContainerRuntimeOptions,
  buildWorkloadsKubernetesContextOptions,
  buildWorkloadsKubernetesNamespaceOptions,
  buildWorkloadsPlatformOptions,
  buildWorkloadNodeOptions,
  deserializeWorkloadViewMode,
} from '../workloadRouteModel';

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

describe('workloadRouteModel', () => {
  it('deserializes persisted workload view-mode aliases through the canonical helper', () => {
    expect(deserializeWorkloadViewMode('all')).toBe('all');
    expect(deserializeWorkloadViewMode('container')).toBe('container');
    expect(deserializeWorkloadViewMode('docker')).toBe('app-container');
    expect(deserializeWorkloadViewMode('Kubernetes')).toBe('pod');
    expect(deserializeWorkloadViewMode('invalid')).toBe('all');
    expect(deserializeWorkloadViewMode(null)).toBe('all');
  });

  it('builds workload node options with instance disambiguation only when needed', () => {
    const options = buildWorkloadNodeOptions([
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

  it('builds app-container host options from Docker host ids and host labels', () => {
    const options = buildWorkloadNodeOptions([
      makeGuest({
        id: 'docker-a',
        type: 'app-container',
        workloadType: 'app-container',
        node: '',
        instance: '',
        contextLabel: 'tower.local',
        dockerHostId: 'docker-host-1',
      }),
      makeGuest({
        id: 'docker-b',
        type: 'app-container',
        workloadType: 'app-container',
        node: '',
        instance: '',
        contextLabel: 'tower.local',
        dockerHostId: 'docker-host-1',
      }),
      makeGuest({
        id: 'truenas-nextcloud',
        type: 'app-container',
        workloadType: 'app-container',
        node: '',
        instance: 'nextcloud',
        contextLabel: 'truenas-main',
        dockerHostId: '',
        platformType: 'truenas',
      }),
    ]);

    expect(options).toEqual([
      { value: 'docker-host-1', label: 'tower.local' },
      { value: 'truenas-main', label: 'truenas-main' },
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

    expect(buildWorkloadsKubernetesContextOptions(guests)).toEqual(['prod', 'stage']);
    expect(buildWorkloadsKubernetesNamespaceOptions(guests, 'prod')).toEqual([
      'default',
      'kube-system',
    ]);
  });
  it('builds container runtime options from canonical app-container guests', () => {
    expect(
      buildWorkloadsContainerRuntimeOptions([
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

  it('builds canonical platform options from visible workload platforms', () => {
    expect(
      buildWorkloadsPlatformOptions(
        [
          makeGuest({
            id: 'app-a',
            type: 'app-container',
            workloadType: 'app-container',
            platformType: 'truenas',
          }),
          makeGuest({
            id: 'app-b',
            type: 'app-container',
            workloadType: 'app-container',
            platformType: 'docker',
            platformScopes: ['proxmox-pve', 'docker'],
          }),
          makeGuest({
            id: 'pod-a',
            type: 'pod',
            workloadType: 'pod',
            platformType: 'kubernetes',
          }),
        ],
        'app-container',
      ),
    ).toEqual([
      { value: 'truenas', label: 'TrueNAS' },
      { value: 'proxmox-pve', label: 'PVE' },
      { value: 'docker', label: 'Docker / Podman' },
    ]);
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

    expect(buildWorkloadsKubernetesNamespaceOptions(guests, 'prod')).toEqual([
      'default',
      'kube-system',
    ]);
    expect(buildWorkloadsKubernetesNamespaceOptions(guests, 'stage')).toEqual(['observability']);
  });
});
