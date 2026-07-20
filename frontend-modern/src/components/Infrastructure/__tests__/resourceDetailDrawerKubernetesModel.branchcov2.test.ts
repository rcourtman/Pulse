import { describe, expect, it } from 'vitest';

import type { ResourceKubernetesPodContainerStatus, Resource } from '@/types/resource';
import type { DetailRow } from '@/components/shared/detailSectionModel';
import {
  buildKubernetesDetailSections,
  buildKubernetesDetailsSummary,
} from '../resourceDetailDrawerKubernetesModel';

// Branch-coverage companion to resourceDetailDrawerKubernetesModel.test.ts.
// Drives the still-uncovered arms of containerRow (module-private, so exercised
// through the pod Containers section), buildKubernetesDetailsSummary and
// buildKubernetesDetailSections. Each case pins a concrete shape/string rather
// than truthiness.

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

const sectionsOf = (resource: Resource): ReturnType<typeof buildKubernetesDetailSections> =>
  buildKubernetesDetailSections(resource);

const sectionByLabel = (resource: Resource, label: string) =>
  sectionsOf(resource).find((section) => section.label === label);

const rowInSection = (
  resource: Resource,
  sectionLabel: string,
  rowLabel: string,
): DetailRow | undefined =>
  sectionByLabel(resource, sectionLabel)?.rows.find((row) => row.label === rowLabel);

// All fields of ResourceKubernetesPodContainerStatus are optional, so a partial
// object is accepted directly; build a single-container pod and return that
// container's rendered row.
const singleContainerRow = (
  container: ResourceKubernetesPodContainerStatus,
): DetailRow | undefined =>
  sectionByLabel(
    makeResource({
      id: 'pod-c',
      type: 'pod',
      kubernetes: { podContainers: [container] },
    }),
    'Containers (1)',
  )?.rows[0];

describe('containerRow (via Containers section)', () => {
  it('falls back to "unknown" state, "container N" label and warning tone when state/name/ready are absent', () => {
    expect(singleContainerRow({ image: 'ghcr.io/acme/api:1.2.3' })).toEqual({
      label: 'container 1',
      value: 'unknown · not ready · ghcr.io/acme/api:1.2.3',
      title: 'unknown · not ready · ghcr.io/acme/api:1.2.3',
      tone: 'warning',
    });
  });

  it('drops the parenthetical when reason matches state case-insensitively, and omits restarts at 0', () => {
    expect(
      singleContainerRow({
        name: 'api',
        state: 'running',
        reason: 'Running',
        ready: true,
        restartCount: 0,
        image: 'ghcr.io/acme/api:1.2.3',
      }),
    ).toEqual({
      label: 'api',
      value: 'running · ready · ghcr.io/acme/api:1.2.3',
      title: 'running · ready · ghcr.io/acme/api:1.2.3',
      tone: 'default',
    });
  });

  it('omits restarts/image parts and folds the message into the title', () => {
    expect(
      singleContainerRow({
        name: 'worker',
        state: 'waiting',
        reason: 'ImagePullBackOff',
        ready: false,
        message: 'manifest not found',
      }),
    ).toEqual({
      label: 'worker',
      value: 'waiting (ImagePullBackOff) · not ready',
      title: 'waiting (ImagePullBackOff) · not ready: manifest not found',
      tone: 'warning',
    });
  });

  it('treats a negative restartCount as zero restarts', () => {
    expect(
      singleContainerRow({ name: 'c', state: 'running', ready: true, restartCount: -3 })?.value,
    ).toBe('running · ready');
  });

  it('treats whitespace-only state/name/image as absent', () => {
    const row = singleContainerRow({ name: '   ', state: '   ', image: '   ', ready: true });
    expect(row?.label).toBe('container 1');
    expect(row?.value).toBe('unknown · ready');
    expect(row?.title).toBe('unknown · ready');
    expect(row?.tone).toBe('default');
  });
});

describe('buildKubernetesDetailsSummary', () => {
  it('returns null when there is no kubernetes metadata', () => {
    expect(buildKubernetesDetailsSummary(makeResource({ id: 'vm-1', type: 'vm' }))).toBeNull();
  });

  it('pluralizes the container count for multi-container pods', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({
          id: 'pod-1',
          type: 'pod',
          kubernetes: { podContainers: [{ name: 'a' }, { name: 'b' }, { name: 'c' }] },
        }),
      ),
    ).toBe('3 containers');
  });

  it('emits only QoS when the pod carries no containers', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({ id: 'pod-2', type: 'pod', kubernetes: { qosClass: 'Burstable' } }),
      ),
    ).toBe('Burstable');
  });

  it('returns null for a pod with no containers and no QoS', () => {
    expect(
      buildKubernetesDetailsSummary(makeResource({ id: 'pod-3', type: 'pod', kubernetes: {} })),
    ).toBeNull();
  });

  it('omits the QoS part when only containers are present', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({ id: 'pod-4', type: 'pod', kubernetes: { podContainers: [{ name: 'a' }] } }),
      ),
    ).toBe('1 container');
  });

  it('omits the cordon marker when a node is schedulable', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({ id: 'node-1', type: 'k8s-node', kubernetes: { osImage: 'Ubuntu 22.04' } }),
      ),
    ).toBe('Ubuntu 22.04');
  });

  it('surfaces only the cordon marker when the node has no OS image', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({ id: 'node-2', type: 'k8s-node', kubernetes: { unschedulable: true } }),
      ),
    ).toBe('Cordoned');
  });

  it('returns null for a node with neither OS image nor cordon state', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({ id: 'node-3', type: 'k8s-node', kubernetes: {} }),
      ),
    ).toBeNull();
  });

  it('returns null for a cluster without a server endpoint', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({ id: 'cluster-1', type: 'k8s-cluster', kubernetes: {} }),
      ),
    ).toBeNull();
  });

  it('returns null for a non-pod/node/cluster type that still carries kubernetes metadata', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({ id: 'vm-2', type: 'vm', kubernetes: { osImage: 'Talos 1.7' } }),
      ),
    ).toBeNull();
  });

  it('recognizes an agent carrying a nodeUid as a node', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({
          id: 'agent-uid',
          type: 'agent',
          kubernetes: { nodeUid: 'node-42', osImage: 'Flatcar' },
        }),
      ),
    ).toBe('Flatcar');
  });

  it('returns null for an agent that is not a kubernetes node', () => {
    expect(
      buildKubernetesDetailsSummary(
        makeResource({ id: 'agent-plain', type: 'agent', kubernetes: { podName: 'x' } }),
      ),
    ).toBeNull();
  });
});

describe('buildKubernetesDetailSections', () => {
  it('drops the Pod section when a pod has no QoS class', () => {
    const pod = makeResource({
      id: 'pod-q',
      type: 'pod',
      kubernetes: { podContainers: [{ name: 'a', state: 'running', ready: true }] },
    });
    expect(sectionsOf(pod).map((section) => section.label)).toEqual(['Containers (1)']);
  });

  it('drops the Containers section when a pod has no containers', () => {
    const pod = makeResource({
      id: 'pod-n',
      type: 'pod',
      kubernetes: { qosClass: 'Guaranteed' },
    });
    expect(sectionsOf(pod).map((section) => section.label)).toEqual(['Pod']);
    expect(sectionByLabel(pod, 'Pod')?.rows).toEqual([{ label: 'QoS class', value: 'Guaranteed' }]);
  });

  it('returns no sections for a pod with neither QoS nor containers', () => {
    expect(
      buildKubernetesDetailSections(makeResource({ id: 'pod-e', type: 'pod', kubernetes: {} })),
    ).toEqual([]);
  });

  it('omits the Scheduling row for a schedulable node', () => {
    const node = makeResource({
      id: 'node-s',
      type: 'k8s-node',
      kubernetes: { osImage: 'Ubuntu 22.04' },
    });
    const section = sectionByLabel(node, 'Kubernetes node');
    expect(section?.rows.find((row) => row.label === 'Scheduling')).toBeUndefined();
    expect(section?.rows.find((row) => row.label === 'OS image')?.value).toBe('Ubuntu 22.04');
  });

  it('formats node capacity as cores only when only CPU is set', () => {
    const node = makeResource({
      id: 'node-cpu',
      type: 'k8s-node',
      kubernetes: { capacityCpuCores: 8 },
    });
    expect(rowInSection(node, 'Kubernetes node', 'Capacity')?.value).toBe('8 cores');
  });

  it('formats node capacity as memory only when only memory is set', () => {
    const node = makeResource({
      id: 'node-mem',
      type: 'k8s-node',
      kubernetes: { capacityMemoryBytes: 2 * 1024 ** 3 },
    });
    expect(rowInSection(node, 'Kubernetes node', 'Capacity')?.value).toBe('2.00 GB');
  });

  it('formats node capacity as pods only when only the pod count is set', () => {
    const node = makeResource({
      id: 'node-pods',
      type: 'k8s-node',
      kubernetes: { capacityPods: 110 },
    });
    expect(rowInSection(node, 'Kubernetes node', 'Capacity')?.value).toBe('110 pods');
  });

  it('drops the Capacity row when no capacity field is finite/positive', () => {
    const node = makeResource({
      id: 'node-empty',
      type: 'k8s-node',
      kubernetes: { capacityCpuCores: 0, osImage: 'Ubuntu' },
    });
    const section = sectionByLabel(node, 'Kubernetes node');
    expect(section?.rows.find((row) => row.label === 'Capacity')).toBeUndefined();
    expect(section?.rows.find((row) => row.label === 'OS image')?.value).toBe('Ubuntu');
  });

  it('omits the Pending uninstall row for a cluster not pending uninstall', () => {
    const cluster = makeResource({
      id: 'cluster-ok',
      type: 'k8s-cluster',
      kubernetes: { server: 'https://k.local', context: 'admin' },
    });
    expect(sectionByLabel(cluster, 'Cluster')?.rows.map((row) => row.label)).toEqual([
      'API server',
      'Context',
    ]);
  });

  it('recognizes an agent carrying a nodeUid as a node and renders its OS image', () => {
    const node = makeResource({
      id: 'agent-node-uid',
      type: 'agent',
      kubernetes: { nodeUid: 'node-9', osImage: 'Flatcar 3600.0.0' },
    });
    expect(sectionByLabel(node, 'Kubernetes node')?.rows).toContainEqual({
      label: 'OS image',
      value: 'Flatcar 3600.0.0',
    });
  });

  it('returns no sections for a non-pod/node/cluster type carrying kubernetes metadata', () => {
    expect(
      buildKubernetesDetailSections(
        makeResource({ id: 'vm-k', type: 'vm', kubernetes: { osImage: 'Talos' } }),
      ),
    ).toEqual([]);
  });
});
