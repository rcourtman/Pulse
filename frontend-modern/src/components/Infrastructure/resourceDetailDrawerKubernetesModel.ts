import type { Resource, ResourceKubernetesPodContainerStatus } from '@/types/resource';
import {
  compactDetailRows as compactRows,
  compactDetailSections as compactSections,
  formatDetailBytesValue,
  formatDetailCountValue,
  formatDetailIntegerValue,
  makeDetailRow as makeRow,
  type DetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/detailSectionModel';
import {
  formatPlatformTableDateTimeValue,
  formatPlatformTableDurationValue,
  formatPlatformTableRelativeTimeValue,
} from '@/features/platformPage/sharedPlatformPage';

export type ResourceDetailDrawerKubernetesSection = DetailSection;

const asString = (value?: string | null): string | null => {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
};

const formatNodeBudget = (cores?: number, memoryBytes?: number, pods?: number): string | null => {
  const parts: string[] = [];
  if (typeof cores === 'number' && Number.isFinite(cores) && cores > 0) {
    parts.push(`${cores} cores`);
  }
  const memory = formatDetailBytesValue(memoryBytes);
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

const countRow = (
  label: string,
  value: number | undefined,
  options: { noun?: string; tone?: (count: number) => DetailValueTone } = {},
): DetailRow | null => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return null;
  const text = options.noun
    ? formatDetailCountValue(value, options.noun)
    : formatDetailIntegerValue(value);
  return makeRow(label, text, options.tone ? { tone: options.tone(value) } : {});
};

const warnWhenPositive = (count: number): DetailValueTone => (count > 0 ? 'warning' : 'default');
const dangerWhenPositive = (count: number): DetailValueTone => (count > 0 ? 'danger' : 'default');

// Kubernetes reports controller timestamps as RFC 3339 strings. The table row
// only has room for an age, so the expansion carries the absolute time first
// and the age after it: "Oct 2, 2026, 09:13 PM (1h ago)". The value wraps
// because a phone-width detail cell is too narrow for both on one line. The
// age is measured from `now`, which an open drawer passes from the shared
// relative-time clock so the age keeps moving while the drawer stays open.
const timestampRow = (
  label: string,
  timestamp: string | null | undefined,
  now: number | undefined,
): DetailRow | null => {
  const raw = asString(timestamp);
  if (!raw) return null;
  const absolute = formatPlatformTableDateTimeValue(raw, {
    emptyText: '',
    dateTimeFormat: { year: 'numeric' },
  });
  if (!absolute) return makeRow(label, raw, { wrap: true });
  const relative = formatPlatformTableRelativeTimeValue(raw, { emptyText: '', now });
  return makeRow(label, relative ? `${absolute} (${relative})` : absolute, { wrap: true });
};

const elapsedSeconds = (start?: string | null, end?: string | null): number | undefined => {
  const startedAt = Date.parse(asString(start) ?? '');
  const endedAt = Date.parse(asString(end) ?? '');
  if (!Number.isFinite(startedAt) || !Number.isFinite(endedAt) || endedAt < startedAt) {
    return undefined;
  }
  return (endedAt - startedAt) / 1000;
};

type ControllerRows = { label: string; rows: Array<DetailRow | null> };

// Each kind leads with what its narrow table row drops: the Detail column
// (service name, timestamps) below the large layout and Target on a phone.
const controllerRows = (resource: Resource, now: number | undefined): ControllerRows | null => {
  const k = resource.kubernetes;
  if (!k) return null;
  switch (resource.type) {
    case 'k8s-replicaset':
      return {
        label: 'ReplicaSet',
        rows: [
          countRow('Target', k.desiredReplicas, { noun: 'pod' }),
          countRow('Current', k.currentReplicas),
          countRow('Ready', k.readyReplicas),
          countRow('Available', k.availableReplicas),
          countRow('Fully labeled', k.fullyLabeledReplicas),
          countRow('Observed generation', k.observedGeneration),
        ],
      };
    case 'k8s-statefulset':
      return {
        label: 'StatefulSet',
        rows: [
          makeRow('Service', k.serviceName),
          countRow('Target', k.desiredReplicas, { noun: 'pod' }),
          countRow('Current', k.currentReplicas),
          countRow('Ready', k.readyReplicas),
          countRow('Available', k.availableReplicas),
          countRow('Updated', k.updatedReplicas),
        ],
      };
    case 'k8s-daemonset':
      return {
        label: 'DaemonSet',
        rows: [
          countRow('Target', k.desiredNumberScheduled, { noun: 'node' }),
          countRow('Current', k.currentNumberScheduled),
          countRow('Ready', k.numberReady),
          countRow('Available', k.numberAvailable),
          countRow('Updated', k.updatedReplicas),
          countRow('Unavailable', k.numberUnavailable, { tone: warnWhenPositive }),
          countRow('Misscheduled', k.numberMisscheduled, { tone: warnWhenPositive }),
        ],
      };
    case 'k8s-job':
      return {
        label: 'Job',
        rows: [
          timestampRow('Started', k.startTime, now),
          timestampRow('Completed', k.completionTime, now),
          makeRow(
            'Duration',
            formatPlatformTableDurationValue(elapsedSeconds(k.startTime, k.completionTime), {
              emptyText: '',
            }),
          ),
          countRow('Target', k.desiredReplicas, { noun: 'completion' }),
          countRow('Active', k.active),
          countRow('Succeeded', k.succeeded),
          countRow('Failed', k.failed, { tone: dangerWhenPositive }),
        ],
      };
    case 'k8s-cronjob':
      return {
        label: 'CronJob',
        rows: [
          makeRow('Schedule', k.schedule, { valueClass: 'font-mono' }),
          timestampRow('Last run', k.lastScheduleTime, now),
          timestampRow('Last success', k.lastSuccessfulTime, now),
          makeRow('Suspended', k.suspend === true ? 'Yes' : null, { tone: 'warning' }),
          countRow('Active', k.active),
        ],
      };
    default:
      return null;
  }
};

// Workload-controller facts for the always-visible summary of the resource
// drawer, the same slot Docker containers use for their timestamps. The
// controllers table hides Scope and Detail below its large layout and Target
// on a phone, so without this a Job's completion time or a CronJob's last
// success is unreachable from a narrow row.
export const buildKubernetesControllerSection = (
  resource: Resource,
  now?: number,
): DetailSection | null => {
  const controller = controllerRows(resource, now);
  if (!controller) return null;
  const rows = compactRows([
    ...controller.rows,
    makeRow('Namespace', resource.kubernetes?.namespace),
    makeRow('Cluster', resource.kubernetes?.clusterName),
  ]);
  if (rows.length === 0) return null;
  return {
    label: controller.label,
    rows,
    testId: 'resource-kubernetes-controller-section',
  };
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
