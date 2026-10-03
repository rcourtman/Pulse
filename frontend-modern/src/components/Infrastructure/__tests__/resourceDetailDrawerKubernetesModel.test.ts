import { afterEach, describe, expect, it, vi } from 'vitest';

import { formatPlatformTableDateTimeValue } from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import {
  buildKubernetesControllerSection,
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

describe('buildKubernetesControllerSection', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  const absolute = (timestamp: string): string =>
    formatPlatformTableDateTimeValue(timestamp, { dateTimeFormat: { year: 'numeric' } });

  it('returns nothing for resources that are not workload controllers', () => {
    expect(
      buildKubernetesControllerSection(
        makeResource({ id: 'pod-1', type: 'pod', kubernetes: { namespace: 'apps' } }),
      ),
    ).toBeNull();
    expect(buildKubernetesControllerSection(makeResource({ id: 'vm-1', type: 'vm' }))).toBeNull();
    expect(
      buildKubernetesControllerSection(makeResource({ id: 'job-1', type: 'k8s-job' })),
    ).toBeNull();
  });

  it('leads a Job with its absolute start and completion times, then the counts', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-24T13:31:00Z'));

    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'nightly-import',
        type: 'k8s-job',
        kubernetes: {
          clusterName: 'prod',
          namespace: 'batch',
          desiredReplicas: 10,
          active: 1,
          succeeded: 8,
          failed: 2,
          startTime: '2026-05-24T12:55:00Z',
          completionTime: '2026-05-24T13:00:00Z',
        },
      }),
    );

    expect(section?.label).toBe('Job');
    expect(section?.testId).toBe('resource-kubernetes-controller-section');
    expect(section?.rows).toEqual([
      { label: 'Started', value: `${absolute('2026-05-24T12:55:00Z')} (36m ago)`, wrap: true },
      { label: 'Completed', value: `${absolute('2026-05-24T13:00:00Z')} (31m ago)`, wrap: true },
      { label: 'Duration', value: '5m' },
      { label: 'Target', value: '10 completions' },
      { label: 'Active', value: '1' },
      { label: 'Succeeded', value: '8' },
      { label: 'Failed', value: '2', tone: 'danger' },
      { label: 'Namespace', value: 'batch' },
      { label: 'Cluster', value: 'prod' },
    ]);
  });

  it('keeps a running Job on its start time without a completion or duration row', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-24T13:31:00Z'));

    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'nightly-import',
        type: 'k8s-job',
        kubernetes: {
          namespace: 'batch',
          desiredReplicas: 1,
          active: 1,
          failed: 0,
          startTime: '2026-05-24T13:29:30Z',
        },
      }),
    );

    expect(section?.rows.map((row) => row.label)).toEqual([
      'Started',
      'Target',
      'Active',
      'Failed',
      'Namespace',
    ]);
    expect(section?.rows[0]).toEqual({
      label: 'Started',
      value: `${absolute('2026-05-24T13:29:30Z')} (1m ago)`,
      wrap: true,
    });
    expect(section?.rows[3]).toEqual({ label: 'Failed', value: '0', tone: 'default' });
  });

  it('leads a CronJob with its schedule and the last run and last success times', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-24T13:31:00Z'));

    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'billing-rollup',
        type: 'k8s-cronjob',
        kubernetes: {
          clusterName: 'prod',
          namespace: 'batch',
          schedule: '0 2 * * *',
          suspend: true,
          active: 0,
          lastScheduleTime: '2026-05-24T02:00:00Z',
          lastSuccessfulTime: '2026-05-23T02:00:00Z',
        },
      }),
    );

    expect(section?.label).toBe('CronJob');
    expect(section?.rows).toEqual([
      { label: 'Schedule', value: '0 2 * * *', valueClass: 'font-mono' },
      { label: 'Last run', value: `${absolute('2026-05-24T02:00:00Z')} (11h ago)`, wrap: true },
      { label: 'Last success', value: `${absolute('2026-05-23T02:00:00Z')} (1d ago)`, wrap: true },
      { label: 'Suspended', value: 'Yes', tone: 'warning' },
      { label: 'Active', value: '0' },
      { label: 'Namespace', value: 'batch' },
      { label: 'Cluster', value: 'prod' },
    ]);
  });

  it('carries the service name and counts for StatefulSets and DaemonSets', () => {
    const statefulSet = buildKubernetesControllerSection(
      makeResource({
        id: 'checkout-api-stateful',
        type: 'k8s-statefulset',
        kubernetes: {
          namespace: 'apps',
          desiredReplicas: 3,
          currentReplicas: 3,
          readyReplicas: 2,
          availableReplicas: 2,
          updatedReplicas: 2,
          serviceName: 'checkout-headless',
        },
      }),
    );
    expect(statefulSet?.label).toBe('StatefulSet');
    expect(statefulSet?.rows).toEqual([
      { label: 'Service', value: 'checkout-headless' },
      { label: 'Target', value: '3 pods' },
      { label: 'Current', value: '3' },
      { label: 'Ready', value: '2' },
      { label: 'Available', value: '2' },
      { label: 'Updated', value: '2' },
      { label: 'Namespace', value: 'apps' },
    ]);

    const daemonSet = buildKubernetesControllerSection(
      makeResource({
        id: 'node-exporter',
        type: 'k8s-daemonset',
        kubernetes: {
          namespace: 'observability',
          desiredNumberScheduled: 6,
          currentNumberScheduled: 6,
          numberReady: 5,
          numberAvailable: 5,
          numberUnavailable: 1,
          numberMisscheduled: 0,
          updatedReplicas: 5,
        },
      }),
    );
    expect(daemonSet?.label).toBe('DaemonSet');
    expect(daemonSet?.rows).toEqual([
      { label: 'Target', value: '6 nodes' },
      { label: 'Current', value: '6' },
      { label: 'Ready', value: '5' },
      { label: 'Available', value: '5' },
      { label: 'Updated', value: '5' },
      { label: 'Unavailable', value: '1', tone: 'warning' },
      { label: 'Misscheduled', value: '0', tone: 'default' },
      { label: 'Namespace', value: 'observability' },
    ]);
  });

  it('shows the ReplicaSet labelling and generation fields the Detail column carries', () => {
    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'checkout-api-replicaset',
        type: 'k8s-replicaset',
        kubernetes: {
          desiredReplicas: 4,
          readyReplicas: 3,
          fullyLabeledReplicas: 4,
          observedGeneration: 7,
        },
      }),
    );
    expect(section?.label).toBe('ReplicaSet');
    expect(section?.rows).toEqual([
      { label: 'Target', value: '4 pods' },
      { label: 'Ready', value: '3' },
      { label: 'Fully labeled', value: '4' },
      { label: 'Observed generation', value: '7' },
    ]);
  });
});
