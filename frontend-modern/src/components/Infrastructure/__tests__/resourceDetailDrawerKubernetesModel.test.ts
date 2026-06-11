import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  buildKubernetesDetailSections,
  buildKubernetesDetailsSummary,
  hasKubernetesDetailSections,
} from '../resourceDetailDrawerKubernetesModel';

const makeResource = ({
  id,
  type,
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'cluster-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'online',
  type,
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

const sectionByLabel = (resource: Resource, label: string) =>
  buildKubernetesDetailSections(resource).find((section) => section.label === label);

describe('buildKubernetesDetailSections', () => {
  it('returns nothing for resources without kubernetes metadata', () => {
    expect(buildKubernetesDetailSections(makeResource({ id: 'vm-1', type: 'vm' }))).toEqual([]);
    expect(hasKubernetesDetailSections(makeResource({ id: 'vm-1', type: 'vm' }))).toBe(false);
  });

  it('builds QoS and per-container rows for pods', () => {
    const pod = makeResource({
      id: 'pod-1',
      type: 'pod',
      kubernetes: {
        qosClass: 'Burstable',
        podContainers: [
          {
            name: 'api',
            image: 'ghcr.io/acme/api:1.2.3',
            ready: true,
            restartCount: 2,
            state: 'running',
          },
          {
            name: 'sidecar',
            image: 'ghcr.io/acme/sidecar:9',
            ready: false,
            restartCount: 7,
            state: 'waiting',
            reason: 'CrashLoopBackOff',
            message: 'back-off 5m0s restarting failed container',
          },
        ],
      },
    });

    const podSection = sectionByLabel(pod, 'Pod');
    expect(podSection?.rows).toEqual([{ label: 'QoS class', value: 'Burstable' }]);

    const containers = sectionByLabel(pod, 'Containers (2)');
    expect(containers?.rows).toHaveLength(2);
    expect(containers?.rows[0].label).toBe('api');
    expect(containers?.rows[0].value).toBe('running · ready · 2 restarts · ghcr.io/acme/api:1.2.3');
    expect(containers?.rows[0].tone).toBe('default');
    expect(containers?.rows[1].label).toBe('sidecar');
    expect(containers?.rows[1].value).toBe(
      'waiting (CrashLoopBackOff) · not ready · 7 restarts · ghcr.io/acme/sidecar:9',
    );
    expect(containers?.rows[1].tone).toBe('warning');
    expect(containers?.rows[1].title).toContain('back-off 5m0s restarting failed container');
  });

  it('builds node identity and capacity vs allocatable rows for k8s nodes', () => {
    const node = makeResource({
      id: 'node-1',
      type: 'k8s-node',
      kubernetes: {
        osImage: 'Ubuntu 22.04.5 LTS',
        kernelVersion: '6.6.32-1-lts',
        architecture: 'amd64',
        capacityCpuCores: 24,
        capacityMemoryBytes: 64 * 1024 ** 3,
        capacityPods: 110,
        allocatableCpuCores: 21,
        allocatableMemoryBytes: 60 * 1024 ** 3,
        allocatablePods: 105,
        unschedulable: true,
      },
    });

    const section = sectionByLabel(node, 'Kubernetes node');
    expect(section?.rows).toEqual([
      { label: 'OS image', value: 'Ubuntu 22.04.5 LTS' },
      { label: 'Kernel', value: '6.6.32-1-lts' },
      { label: 'Architecture', value: 'amd64' },
      { label: 'Capacity', value: '24 cores / 64.0 GB / 110 pods' },
      { label: 'Allocatable', value: '21 cores / 60.0 GB / 105 pods' },
      { label: 'Scheduling', value: 'Cordoned (unschedulable)', tone: 'warning' },
    ]);
  });

  it('treats agent rows carrying kubelet metadata as nodes', () => {
    const mergedNode = makeResource({
      id: 'agent-1',
      type: 'agent',
      kubernetes: { kubeletVersion: 'v1.31.2', osImage: 'Fedora CoreOS 40' },
    });
    expect(sectionByLabel(mergedNode, 'Kubernetes node')?.rows).toContainEqual({
      label: 'OS image',
      value: 'Fedora CoreOS 40',
    });
  });

  it('surfaces the API server endpoint for clusters', () => {
    const cluster = makeResource({
      id: 'cluster-1',
      type: 'k8s-cluster',
      kubernetes: {
        server: 'https://prod.k8s.local:6443',
        context: 'prod-admin',
        agentVersion: '0.9.1',
        pendingUninstall: true,
      },
    });

    const section = sectionByLabel(cluster, 'Cluster');
    expect(section?.rows).toEqual([
      { label: 'API server', value: 'https://prod.k8s.local:6443' },
      { label: 'Context', value: 'prod-admin' },
      { label: 'Agent version', value: '0.9.1' },
      { label: 'Pending uninstall', value: 'Yes', tone: 'warning' },
    ]);
  });
});

describe('buildKubernetesDetailsSummary', () => {
  it('summarizes pods by container count and QoS', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({
          id: 'pod-1',
          type: 'pod',
          kubernetes: { qosClass: 'Guaranteed', podContainers: [{ name: 'api' }] },
        }),
      ),
    ).toBe('1 container · Guaranteed');
  });

  it('summarizes nodes by OS image and cordon state', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({
          id: 'node-1',
          type: 'k8s-node',
          kubernetes: { osImage: 'Ubuntu 22.04.5 LTS', unschedulable: true },
        }),
      ),
    ).toBe('Ubuntu 22.04.5 LTS · Cordoned');
  });

  it('summarizes clusters by API server', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({
          id: 'cluster-1',
          type: 'k8s-cluster',
          kubernetes: { server: 'https://prod.k8s.local:6443' },
        }),
      ),
    ).toBe('https://prod.k8s.local:6443');
  });
});
