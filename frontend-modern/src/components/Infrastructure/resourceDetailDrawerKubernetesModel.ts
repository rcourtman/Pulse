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
import { formatPlatformTableDurationValue } from '@/features/platformPage/sharedPlatformPage';
import { drawerContentCardTestId } from './resourceDetailDrawerOverviewModel';
import { makeDetailTimestampRow } from '@/features/platformPage/platformDetailTimestampRow';

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

// The drawer is where a phone reaches values the table truncates or hides, so
// identifiers an operator copies (images, addresses, names) wrap in full.
const wrapRow = (label: string, value?: string | null) => makeRow(label, value, { wrap: true });

const joinValues = (values?: readonly (string | null | undefined)[]): string | null => {
  const visible = (values ?? []).map((value) => asString(value)).filter(Boolean);
  return visible.length > 0 ? visible.join(', ') : null;
};

const formatKeyValues = (values?: Record<string, string>): string | null =>
  joinValues(
    Object.entries(values ?? {}).map(([key, value]) => (asString(value) ? `${key}=${value}` : key)),
  );

// A container of a finished pod (a Job's pod that ran to completion) is
// terminated and not ready by design: that is not something to check.
const containerNeedsAttention = (
  container: ResourceKubernetesPodContainerStatus,
  podPhase?: string,
): boolean => {
  if (container.ready === true) return false;
  if (asString(podPhase)?.toLowerCase() === 'succeeded') return false;
  const state = asString(container.state)?.toLowerCase();
  const reason = asString(container.reason)?.toLowerCase();
  return !(state === 'terminated' && reason === 'completed');
};

const containerRow = (
  container: ResourceKubernetesPodContainerStatus,
  index: number,
  podPhase?: string,
) => {
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
    tone: containerNeedsAttention(container, podPhase) ? 'warning' : 'default',
    wrap: true,
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
// A pod's containers are its content, so they stay in view on the Overview
// as a card, the way a guest's filesystems do.
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
        testId: drawerContentCardTestId('kubernetes-containers'),
        rows: compactRows(
          containers.map((container, index) => containerRow(container, index, k.podPhase)),
        ),
      });
    }
  }

  if (isKubernetesNodeResource(resource)) {
    sections.push({
      label: 'Kubernetes node',
      rows: compactRows([
        wrapRow('OS image', k.osImage),
        wrapRow('Kernel', k.kernelVersion),
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
        wrapRow('API server', k.server),
        wrapRow('Context', k.context),
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

// Kubernetes omits zero counts, so once a target is set a missing count means
// none. A count short of the target reads "2 of 3" and is lifted to Worth
// checking; one that meets it reads as a plain number.
const shortfallRow = (
  label: string,
  value: number | undefined,
  desired: number | undefined,
): DetailRow | null => {
  const target = typeof desired === 'number' && Number.isFinite(desired) ? desired : 0;
  const count = typeof value === 'number' && Number.isFinite(value) ? value : undefined;
  if (target <= 0) return countRow(label, count);
  const actual = count ?? 0;
  if (actual >= target) return countRow(label, actual);
  return makeRow(
    label,
    `${formatDetailIntegerValue(actual)} of ${formatDetailIntegerValue(target)}`,
    { tone: 'warning' },
  );
};

const servicePortLabel = (
  port: NonNullable<NonNullable<Resource['kubernetes']>['servicePorts']>[number],
): string | null => {
  if (typeof port.port !== 'number' || !Number.isFinite(port.port)) return null;
  const protocol = asString(port.protocol);
  const target = asString(port.targetPort);
  const name = asString(port.name);
  const nodePort =
    typeof port.nodePort === 'number' && port.nodePort > 0 ? ` (node ${port.nodePort})` : '';
  const mapping =
    target && target !== String(port.port) ? `${port.port}→${target}` : `${port.port}`;
  return `${name ? `${name} ` : ''}${mapping}${protocol ? `/${protocol}` : ''}${nodePort}`;
};

// A claim that cannot bind, or a volume that failed, needs a look; Bound and
// Available are the settled states.
const volumePhaseTone = (phase?: string): DetailValueTone => {
  const normalized = asString(phase)?.toLowerCase();
  if (normalized === 'lost' || normalized === 'failed') return 'danger';
  if (normalized === 'pending') return 'warning';
  return 'default';
};

const volumeSizeValue = (bytes?: number): string | null =>
  formatDetailBytesValue(bytes, { precision: 'compact', trimWhole: true });

const autoscalerRangeValue = (min?: number, max?: number): string | null => {
  const hasMax = typeof max === 'number' && max > 0;
  if (!hasMax) return null;
  // Kubernetes defaults minReplicas to 1 and omits it from the status.
  const floor = typeof min === 'number' && min > 0 ? min : 1;
  return `${formatDetailIntegerValue(floor)} to ${formatDetailCountValue(max, 'pod')}`;
};

// An autoscaler held at its ceiling cannot add pods however busy the target
// gets, which is worth a look.
const autoscalerAtLimitRow = (current?: number, max?: number): DetailRow | null => {
  if (typeof max !== 'number' || max <= 0) return null;
  if (typeof current !== 'number' || current < max) return null;
  return makeRow('Scaling', `At its maximum of ${formatDetailCountValue(max, 'pod')}`, {
    tone: 'warning',
  });
};
const dangerWhenPositive = (count: number): DetailValueTone => (count > 0 ? 'danger' : 'default');

// Kubernetes reports controller timestamps as RFC 3339 strings; the drawer
// shows the absolute time and its age (see makeDetailTimestampRow).
const timestampRow = makeDetailTimestampRow;

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
    case 'k8s-deployment':
      return {
        label: 'Deployment',
        rows: [
          countRow('Target', k.desiredReplicas, { noun: 'pod' }),
          shortfallRow('Updated', k.updatedReplicas, k.desiredReplicas),
          shortfallRow('Ready', k.readyReplicas, k.desiredReplicas),
          shortfallRow('Available', k.availableReplicas, k.desiredReplicas),
          countRow('Observed generation', k.observedGeneration),
        ],
      };
    case 'k8s-horizontal-pod-autoscaler':
      return {
        label: 'Autoscaler',
        rows: [
          wrapRow(
            'Target',
            [asString(k.targetKind), asString(k.targetName)].filter(Boolean).join(' '),
          ),
          makeRow('Range', autoscalerRangeValue(k.minReplicas, k.maxReplicas)),
          countRow('Current', k.currentReplicas ?? (k.maxReplicas ? 0 : undefined), {
            noun: 'pod',
          }),
          countRow('Desired', k.desiredReplicas ?? (k.maxReplicas ? 0 : undefined), {
            noun: 'pod',
          }),
          autoscalerAtLimitRow(k.currentReplicas, k.maxReplicas),
          wrapRow('Metrics', joinValues(k.metricTypes)),
        ],
      };
    case 'k8s-service':
      return {
        label: 'Service',
        rows: [
          makeRow('Type', k.serviceType),
          wrapRow('Cluster IP', k.clusterIp),
          wrapRow('External IPs', joinValues(k.externalIps)),
          wrapRow('Ports', joinValues((k.servicePorts ?? []).map(servicePortLabel))),
          wrapRow('Selector', formatKeyValues(k.selector)),
        ],
      };
    case 'k8s-ingress':
      return {
        label: 'Ingress',
        rows: [
          makeRow('Class', k.className),
          wrapRow('Hosts', joinValues(k.hosts)),
          wrapRow('Addresses', joinValues(k.addresses)),
        ],
      };
    case 'k8s-persistent-volume-claim':
      return {
        label: 'Volume claim',
        rows: [
          makeRow('Phase', k.phase, { tone: volumePhaseTone(k.phase) }),
          wrapRow('Volume', k.volumeName),
          makeRow('Storage class', k.storageClass),
          makeRow('Requested', volumeSizeValue(k.requestedBytes)),
          makeRow('Capacity', volumeSizeValue(k.capacityBytes)),
          makeRow('Access modes', joinValues(k.accessModes)),
        ],
      };
    case 'k8s-persistent-volume':
      return {
        label: 'Persistent volume',
        rows: [
          makeRow('Phase', k.phase, { tone: volumePhaseTone(k.phase) }),
          wrapRow(
            'Claim',
            asString(k.claimName)
              ? [asString(k.claimNamespace), asString(k.claimName)].filter(Boolean).join('/')
              : null,
          ),
          makeRow('Capacity', volumeSizeValue(k.capacityBytes)),
          makeRow('Storage class', k.storageClass),
          makeRow('Access modes', joinValues(k.accessModes)),
          makeRow('Reclaim policy', k.reclaimPolicy),
        ],
      };
    case 'k8s-storage-class':
      return {
        label: 'Storage class',
        rows: [
          wrapRow('Provisioner', k.provisioner),
          makeRow('Reclaim policy', k.reclaimPolicy),
          makeRow('Binding mode', k.volumeBindingMode),
          makeRow(
            'Volume expansion',
            k.allowVolumeExpansion === undefined
              ? null
              : k.allowVolumeExpansion
                ? 'Allowed'
                : 'Not allowed',
          ),
          wrapRow('Parameters', joinValues(k.parameterKeys)),
        ],
      };
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
          wrapRow('Service', k.serviceName),
          countRow('Target', k.desiredReplicas, { noun: 'pod' }),
          countRow('Current', k.currentReplicas),
          shortfallRow('Ready', k.readyReplicas, k.desiredReplicas),
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

// Workload-controller, network and storage facts for the resource drawer's
// inventory, the same slot Docker containers use for their timestamps. The
// Kubernetes tables hide Scope and Detail below their large layout and Target
// on a phone, so without this a Job's completion time, a Service's address or
// a claim's volume is unreachable from a narrow row. A shortfall (fewer ready
// pods than the target) is warning-toned, so the Overview lifts it.
export const buildKubernetesControllerSection = (
  resource: Resource,
  now?: number,
): DetailSection | null => {
  const controller = controllerRows(resource, now);
  if (!controller) return null;
  const rows = compactRows([
    ...controller.rows,
    wrapRow('Namespace', resource.kubernetes?.namespace),
    wrapRow('Cluster', resource.kubernetes?.clusterName),
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

// The facts line for a pod: where it runs and what owns it. The pods table
// hides Namespace, Owner and Node on a phone, so the drawer states them.
export const getKubernetesPodFacts = (resource: Resource): string[] => {
  if (resource.type !== 'pod') return [];
  const k = resource.kubernetes;
  if (!k) return [];
  const ownerName = asString(k.ownerName);
  return [
    asString(k.namespace) ? `Namespace ${asString(k.namespace)}` : '',
    ownerName ? [asString(k.ownerKind), ownerName].filter(Boolean).join(' ') : '',
    asString(k.nodeName) ? `Node ${asString(k.nodeName)}` : '',
  ].filter(Boolean);
};
