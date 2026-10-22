import { afterEach, describe, expect, it, vi } from 'vitest';

import { formatPlatformTableDateTimeValue } from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import {
  buildKubernetesControllerSection,
  buildKubernetesDetailSections,
  buildKubernetesDetailsSummary,
  getKubernetesPodFacts,
  hasKubernetesDetailSections,
} from '../resourceDetailDrawerKubernetesModel';
import { isDrawerContentCard } from '../resourceDetailDrawerOverviewModel';

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
      { label: 'OS image', value: 'Ubuntu 22.04.5 LTS', wrap: true },
      { label: 'Kernel', value: '6.6.32-1-lts', wrap: true },
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
      wrap: true,
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
      { label: 'API server', value: 'https://prod.k8s.local:6443', wrap: true },
      { label: 'Context', value: 'prod-admin', wrap: true },
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
      { label: 'Namespace', value: 'batch', wrap: true },
      { label: 'Cluster', value: 'prod', wrap: true },
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
      { label: 'Namespace', value: 'batch', wrap: true },
      { label: 'Cluster', value: 'prod', wrap: true },
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
      { label: 'Service', value: 'checkout-headless', wrap: true },
      { label: 'Target', value: '3 pods' },
      { label: 'Current', value: '3' },
      // Fewer ready pods than the target is a shortfall the Overview lifts.
      { label: 'Ready', value: '2 of 3', tone: 'warning' },
      { label: 'Available', value: '2' },
      { label: 'Updated', value: '2' },
      { label: 'Namespace', value: 'apps', wrap: true },
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
      { label: 'Namespace', value: 'observability', wrap: true },
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

describe('Kubernetes network, storage and scaling sections', () => {
  it('states a Deployment shortfall as "n of target" so the Overview lifts it', () => {
    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'checkout',
        type: 'k8s-deployment',
        kubernetes: {
          namespace: 'apps',
          desiredReplicas: 3,
          updatedReplicas: 3,
          readyReplicas: 2,
          observedGeneration: 4,
        },
      }),
    );
    expect(section?.label).toBe('Deployment');
    expect(section?.rows).toEqual([
      { label: 'Target', value: '3 pods' },
      { label: 'Updated', value: '3' },
      { label: 'Ready', value: '2 of 3', tone: 'warning' },
      // Kubernetes omits a zero count: none available.
      { label: 'Available', value: '0 of 3', tone: 'warning' },
      { label: 'Observed generation', value: '4' },
      { label: 'Namespace', value: 'apps', wrap: true },
    ]);
  });

  it('reads a settled Deployment as plain counts with nothing to lift', () => {
    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'checkout',
        type: 'k8s-deployment',
        kubernetes: {
          desiredReplicas: 2,
          updatedReplicas: 2,
          readyReplicas: 2,
          availableReplicas: 2,
        },
      }),
    );
    expect(section?.rows.some((row) => row.tone === 'warning')).toBe(false);
    expect(section?.rows.find((row) => row.label === 'Ready')?.value).toBe('2');
  });

  it('describes an autoscaler and flags one held at its maximum', () => {
    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'web-hpa',
        type: 'k8s-horizontal-pod-autoscaler',
        kubernetes: {
          targetKind: 'Deployment',
          targetName: 'web',
          minReplicas: 2,
          maxReplicas: 5,
          currentReplicas: 5,
          desiredReplicas: 5,
          metricTypes: ['Resource', 'Pods'],
        },
      }),
    );
    expect(section?.label).toBe('Autoscaler');
    expect(section?.rows).toEqual([
      { label: 'Target', value: 'Deployment web', wrap: true },
      { label: 'Range', value: '2 to 5 pods' },
      { label: 'Current', value: '5 pods' },
      { label: 'Desired', value: '5 pods' },
      { label: 'Scaling', value: 'At its maximum of 5 pods', tone: 'warning' },
      { label: 'Metrics', value: 'Resource, Pods', wrap: true },
    ]);
  });

  it('shows a Service address, ports and selector in full', () => {
    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'web',
        type: 'k8s-service',
        kubernetes: {
          namespace: 'apps',
          clusterName: 'prod',
          serviceType: 'NodePort',
          clusterIp: '10.96.12.4',
          externalIps: ['203.0.113.7'],
          servicePorts: [
            { name: 'http', port: 80, targetPort: '8080', protocol: 'TCP', nodePort: 30080 },
            { port: 443, targetPort: '443', protocol: 'TCP' },
          ],
          selector: { app: 'web', tier: 'frontend' },
        },
      }),
    );
    expect(section?.label).toBe('Service');
    expect(section?.rows).toEqual([
      { label: 'Type', value: 'NodePort' },
      { label: 'Cluster IP', value: '10.96.12.4', wrap: true },
      { label: 'External IPs', value: '203.0.113.7', wrap: true },
      { label: 'Ports', value: 'http 80→8080/TCP (node 30080), 443/TCP', wrap: true },
      { label: 'Selector', value: 'app=web, tier=frontend', wrap: true },
      { label: 'Namespace', value: 'apps', wrap: true },
      { label: 'Cluster', value: 'prod', wrap: true },
    ]);
  });

  it('shows an Ingress class, hosts and addresses', () => {
    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'web-ingress',
        type: 'k8s-ingress',
        kubernetes: {
          className: 'nginx',
          hosts: ['shop.example.com', 'www.shop.example.com'],
          addresses: ['198.51.100.20'],
        },
      }),
    );
    expect(section?.rows).toEqual([
      { label: 'Class', value: 'nginx' },
      { label: 'Hosts', value: 'shop.example.com, www.shop.example.com', wrap: true },
      { label: 'Addresses', value: '198.51.100.20', wrap: true },
    ]);
  });

  it('flags a claim that cannot bind and names its volume and size', () => {
    const pending = buildKubernetesControllerSection(
      makeResource({
        id: 'data-claim',
        type: 'k8s-persistent-volume-claim',
        kubernetes: {
          phase: 'Pending',
          storageClass: 'fast',
          requestedBytes: 10 * 1024 ** 3,
          accessModes: ['ReadWriteOnce'],
        },
      }),
    );
    expect(pending?.label).toBe('Volume claim');
    expect(pending?.rows).toEqual([
      { label: 'Phase', value: 'Pending', tone: 'warning' },
      { label: 'Storage class', value: 'fast' },
      { label: 'Requested', value: '10 GB' },
      { label: 'Access modes', value: 'ReadWriteOnce' },
    ]);

    const bound = buildKubernetesControllerSection(
      makeResource({
        id: 'data-claim',
        type: 'k8s-persistent-volume-claim',
        kubernetes: { phase: 'Bound', volumeName: 'pvc-0f3a', capacityBytes: 10 * 1024 ** 3 },
      }),
    );
    expect(bound?.rows).toEqual([
      { label: 'Phase', value: 'Bound', tone: 'default' },
      { label: 'Volume', value: 'pvc-0f3a', wrap: true },
      { label: 'Capacity', value: '10 GB' },
    ]);
  });

  it('shows a persistent volume claim reference and reclaim policy', () => {
    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'pvc-0f3a',
        type: 'k8s-persistent-volume',
        kubernetes: {
          phase: 'Failed',
          claimNamespace: 'apps',
          claimName: 'data-claim',
          capacityBytes: 10 * 1024 ** 3,
          storageClass: 'fast',
          accessModes: ['ReadWriteOnce'],
          reclaimPolicy: 'Retain',
        },
      }),
    );
    expect(section?.label).toBe('Persistent volume');
    expect(section?.rows).toEqual([
      { label: 'Phase', value: 'Failed', tone: 'danger' },
      { label: 'Claim', value: 'apps/data-claim', wrap: true },
      { label: 'Capacity', value: '10 GB' },
      { label: 'Storage class', value: 'fast' },
      { label: 'Access modes', value: 'ReadWriteOnce' },
      { label: 'Reclaim policy', value: 'Retain' },
    ]);
  });

  it('describes a storage class', () => {
    const section = buildKubernetesControllerSection(
      makeResource({
        id: 'fast',
        type: 'k8s-storage-class',
        kubernetes: {
          provisioner: 'rancher.io/local-path',
          reclaimPolicy: 'Delete',
          volumeBindingMode: 'WaitForFirstConsumer',
          allowVolumeExpansion: false,
          parameterKeys: ['nodePath'],
        },
      }),
    );
    expect(section?.rows).toEqual([
      { label: 'Provisioner', value: 'rancher.io/local-path', wrap: true },
      { label: 'Reclaim policy', value: 'Delete' },
      { label: 'Binding mode', value: 'WaitForFirstConsumer' },
      { label: 'Volume expansion', value: 'Not allowed' },
      { label: 'Parameters', value: 'nodePath', wrap: true },
    ]);
  });
});

describe('Kubernetes pod content and facts', () => {
  it("keeps a pod's containers as an Overview content card", () => {
    const containers = sectionByLabel(
      makeResource({
        id: 'pod-1',
        type: 'pod',
        kubernetes: { podContainers: [{ name: 'api', ready: true, state: 'running' }] },
      }),
      'Containers (1)',
    );
    expect(containers && isDrawerContentCard(containers)).toBe(true);
  });

  it('does not flag the finished containers of a completed pod', () => {
    const rows = sectionByLabel(
      makeResource({
        id: 'job-pod',
        type: 'pod',
        kubernetes: {
          podPhase: 'Succeeded',
          podContainers: [{ name: 'run', ready: false, state: 'terminated', reason: 'Completed' }],
        },
      }),
      'Containers (1)',
    )?.rows;
    expect(rows?.[0].tone).toBe('default');
  });

  it('states namespace, owner and node for a pod only', () => {
    expect(
      getKubernetesPodFacts(
        makeResource({
          id: 'web-7d9-abc',
          type: 'pod',
          kubernetes: {
            namespace: 'apps',
            ownerKind: 'ReplicaSet',
            ownerName: 'web-7d9',
            nodeName: 'worker-2',
          },
        }),
      ),
    ).toEqual(['Namespace apps', 'ReplicaSet web-7d9', 'Node worker-2']);
    expect(
      getKubernetesPodFacts(
        makeResource({ id: 'web', type: 'k8s-deployment', kubernetes: { namespace: 'apps' } }),
      ),
    ).toEqual([]);
  });
});
