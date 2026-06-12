import type { Resource, ResourceKubernetesPodContainerStatus } from '@/types/resource';
import {
  compactDetailRows as compactRows,
  compactDetailSections as compactSections,
  makeDetailRow as makeRow,
  type DetailSection,
} from '@/components/shared/detailSectionModel';

export type ResourceDetailDrawerKubernetesSection = DetailSection;

const asString = (value?: string | null): string | null => {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
};

const formatBytes = (bytes?: number): string | null => {
  if (typeof bytes !== 'number' || !Number.isFinite(bytes) || bytes <= 0) return null;
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let scaled = bytes;
  let unitIndex = 0;
  while (scaled >= 1024 && unitIndex < units.length - 1) {
    scaled /= 1024;
    unitIndex += 1;
  }
  return `${scaled.toFixed(scaled >= 100 ? 0 : scaled >= 10 ? 1 : 2)} ${units[unitIndex]}`;
};

const formatNodeBudget = (cores?: number, memoryBytes?: number, pods?: number): string | null => {
  const parts: string[] = [];
  if (typeof cores === 'number' && Number.isFinite(cores) && cores > 0) {
    parts.push(`${cores} cores`);
  }
  const memory = formatBytes(memoryBytes);
  if (memory) parts.push(memory);
  if (typeof pods === 'number' && Number.isFinite(pods) && pods > 0) {
    parts.push(`${pods} pods`);
  }
  return parts.length > 0 ? parts.join(' / ') : null;
};

const containerRow = (container: ResourceKubernetesPodContainerStatus, index: number) => {
  const state = asString(container.state) ?? 'unknown';
  const reason = asString(container.reason);
  const stateLabel =
    reason && reason.toLowerCase() !== state.toLowerCase() ? `${state} (${reason})` : state;
  const readiness = container.ready === true ? 'ready' : 'not ready';
  const restarts =
    typeof container.restartCount === 'number' && container.restartCount > 0
      ? `${container.restartCount} restarts`
      : null;
  const image = asString(container.image);
  const value = [stateLabel, readiness, restarts, image].filter(Boolean).join(' · ');
  const message = asString(container.message);
  return makeRow(asString(container.name) ?? `container ${index + 1}`, value, {
    title: message ? `${value}: ${message}` : value,
    tone: container.ready === true ? 'default' : 'warning',
  });
};

const isKubernetesNodeResource = (resource: Resource): boolean => {
  if (resource.type === 'k8s-node') return true;
  if (resource.type !== 'agent') return false;
  const k = resource.kubernetes;
  return Boolean(k && (asString(k.nodeUid) || asString(k.kubeletVersion)));
};

// Kubernetes-native detail sections for the resource drawer. Carries the
// fields the platform tables do not show: per-container status and QoS for
// pods, node identity (OS image / kernel / architecture) plus the capacity
// vs allocatable budget for nodes, and the API server endpoint for clusters.
// For nodes without a linked Pulse host agent this is the only place the OS
// identity surfaces at all; the generic host section reads agent data only.
export const buildKubernetesDetailSections = (
  resource: Resource,
): ResourceDetailDrawerKubernetesSection[] => {
  const k = resource.kubernetes;
  if (!k) return [];

  const sections: Array<DetailSection | null> = [];

  if (resource.type === 'pod') {
    const podRows = compactRows([makeRow('QoS class', k.qosClass)]);
    if (podRows.length > 0) {
      sections.push({ label: 'Pod', rows: podRows });
    }
    const containers = k.podContainers ?? [];
    if (containers.length > 0) {
      sections.push({
        label: `Containers (${containers.length})`,
        rows: compactRows(containers.map((container, index) => containerRow(container, index))),
      });
    }
  }

  if (isKubernetesNodeResource(resource)) {
    sections.push({
      label: 'Kubernetes node',
      rows: compactRows([
        makeRow('OS image', k.osImage),
        makeRow('Kernel', k.kernelVersion),
        makeRow('Architecture', k.architecture),
        makeRow(
          'Capacity',
          formatNodeBudget(k.capacityCpuCores, k.capacityMemoryBytes, k.capacityPods),
        ),
        makeRow(
          'Allocatable',
          formatNodeBudget(k.allocatableCpuCores, k.allocatableMemoryBytes, k.allocatablePods),
        ),
        makeRow('Scheduling', k.unschedulable === true ? 'Cordoned (unschedulable)' : null, {
          tone: 'warning',
        }),
      ]),
    });
  }

  if (resource.type === 'k8s-cluster') {
    sections.push({
      label: 'Cluster',
      rows: compactRows([
        makeRow('API server', k.server),
        makeRow('Context', k.context),
        makeRow('Agent version', k.agentVersion),
        makeRow('Pending uninstall', k.pendingUninstall === true ? 'Yes' : null, {
          tone: 'warning',
        }),
      ]),
    });
  }

  return compactSections(sections);
};

export const buildKubernetesDetailsSummary = (resource: Resource): string | null => {
  const k = resource.kubernetes;
  if (!k) return null;
  if (resource.type === 'pod') {
    const containers = k.podContainers?.length ?? 0;
    const parts = [
      containers > 0 ? `${containers} container${containers === 1 ? '' : 's'}` : null,
      asString(k.qosClass),
    ].filter(Boolean);
    return parts.length > 0 ? parts.join(' · ') : null;
  }
  if (isKubernetesNodeResource(resource)) {
    const parts = [asString(k.osImage), k.unschedulable === true ? 'Cordoned' : null].filter(
      Boolean,
    );
    return parts.length > 0 ? parts.join(' · ') : null;
  }
  if (resource.type === 'k8s-cluster') {
    return asString(k.server);
  }
  return null;
};

export const hasKubernetesDetailSections = (resource: Resource): boolean =>
  buildKubernetesDetailSections(resource).length > 0;
