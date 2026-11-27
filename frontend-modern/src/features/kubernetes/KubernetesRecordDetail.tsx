import { Show, type Component, type ComponentProps, type JSX } from 'solid-js';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell } from '@/components/shared/Table';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { unifiedPlatformOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import { RecordOwnerLink } from '@/features/platformPage/RecordOwnerLink';
import type { ResourcePageSubject } from '@/features/platformNavigation/resourcePagePath';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { Resource } from '@/types/resource';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import type { StatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformResourceDetailTableRow,
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  type PlatformResourceDetailState,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  getPlatformTableCellClassForKind,
  getPlatformTableFiniteMetric,
  type PlatformTableFilterOption,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  KUBERNETES_STALE_READINGS_LABEL,
  describeKubernetesStaleReadings,
  kubernetesClusterLabel,
  mapKubernetesEventSeverity,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';
import { makeDetailTimestampRow } from '@/features/platformPage/platformDetailTimestampRow';

export const createKubernetesDetailState = (idPrefix: string): PlatformResourceDetailState =>
  createPlatformResourceDetailState({ idPrefix });

export const getKubernetesMemorySortValue = (resource: Resource): number | null => {
  const total = getPlatformTableFiniteMetric(resource.memory?.total) ?? 0;
  if (total > 0) {
    return ((getPlatformTableFiniteMetric(resource.memory?.used) ?? 0) / total) * 100;
  }
  return getPlatformTableFiniteMetric(resource.memory?.current) ?? null;
};

export const KubernetesCpuMemoryCells: Component<{
  resource: Resource;
  canRenderMetrics: boolean;
  memoryFallback: JSX.Element;
}> = (props) => {
  const alertsActivation = useAlertsActivation();
  const alertResourceIds = () => unifiedPlatformOverrideIdCandidates(props.resource);
  const cpuPercent = () => getPlatformTableFiniteMetric(props.resource.cpu?.current);
  const memoryTotal = () => getPlatformTableFiniteMetric(props.resource.memory?.total) ?? 0;
  const memoryUsed = () => getPlatformTableFiniteMetric(props.resource.memory?.used) ?? 0;
  const memoryPercentOnly = () =>
    memoryTotal() > 0 ? undefined : getPlatformTableFiniteMetric(props.resource.memory?.current);
  const metricCellClass = `${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`;
  return (
    <>
      <TableCell class={metricCellClass}>
        <ResponsiveMetricCell
          class="w-full"
          value={cpuPercent() ?? 0}
          type="cpu"
          resourceId={buildMetricKeyForUnifiedResource(props.resource)}
          isRunning={props.canRenderMetrics && cpuPercent() !== undefined}
          showMobile={false}
          thresholds={alertsActivation.getMetricThresholds('kubernetes', 'cpu', alertResourceIds())}
        />
      </TableCell>
      <TableCell class={metricCellClass}>
        <Show
          when={props.canRenderMetrics && (memoryTotal() > 0 || memoryPercentOnly() !== undefined)}
          fallback={props.memoryFallback}
        >
          <StackedMemoryBar
            used={memoryUsed()}
            total={memoryTotal()}
            percentOnly={memoryPercentOnly()}
            thresholds={alertsActivation.getMetricThresholds(
              'kubernetes',
              'memory',
              alertResourceIds(),
            )}
          />
        </Show>
      </TableCell>
    </>
  );
};

export const KubernetesStaleDataCue: Component<{ when: boolean; scope: string }> = (props) => (
  <Show when={props.when}>
    <span
      class="block truncate text-[11px] font-medium text-amber-700 dark:text-amber-300"
      data-kubernetes-stale={props.scope}
    >
      {KUBERNETES_STALE_READINGS_LABEL}
    </span>
  </Show>
);

export const KubernetesRowLead: Component<{
  resource: Resource;
  drawer: PlatformResourceDetailState;
  label: string;
  status: StatusIndicator;
  dotTitle?: string;
  statusDot?: JSX.Element;
  children?: JSX.Element;
}> = (props) => (
  <div class="flex min-w-0 items-center gap-2">
    <PlatformResourceDetailToggleButton
      expanded={props.drawer.isExpanded(props.resource)}
      resourceLabel={props.label}
      controlsId={props.drawer.detailRowId(props.resource)}
      onToggle={() => props.drawer.toggle(props.resource)}
    />
    {props.statusDot ?? (
      <StatusDot
        size="sm"
        variant={props.status.variant}
        title={props.dotTitle ?? props.status.label}
        ariaHidden
      />
    )}
    {props.children ?? (
      <span class="truncate font-semibold text-base-content" title={props.label}>
        {props.label}
      </span>
    )}
  </div>
);

export const KubernetesDetailRow: Component<{
  resource: Resource;
  statusIndicator: StatusIndicator;
  drawer: PlatformResourceDetailState;
  colSpan: number;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
}> = (props) => {
  const staleMessage = () => describeKubernetesStaleReadings(props.resource);
  return (
    <PlatformResourceDetailTableRow
      resource={props.resource}
      statusIndicator={props.statusIndicator}
      open={props.drawer.isExpanded(props.resource)}
      detailRowId={props.drawer.detailRowId(props.resource)}
      colSpan={props.colSpan}
      resolveResourceLabel={props.resolveResourceLabel}
      context={
        <Show when={staleMessage()}>
          <div class="mb-3" data-testid="kubernetes-stale-readings-notice">
            <InlineNotice tone="warning" role="status">
              <span class="font-medium">{KUBERNETES_STALE_READINGS_LABEL}.</span> {staleMessage()}
            </InlineNotice>
          </div>
        </Show>
      }
      onClose={() => props.drawer.close(props.resource)}
    />
  );
};

export function KubernetesTableFrame<
  Status extends string = KubernetesResourceStatusFilter,
>(props: {
  hasResources: boolean;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
  rowNoun: string;
  searchPlaceholder?: string;
  filteredEmptyTitle?: string;
  filteredEmptyNoun?: string;
  statusOptions?: readonly PlatformTableFilterOption<Status>[];
  tableState: {
    search: () => string;
    setSearch: (value: string) => void;
    searchSuggestions: ComponentProps<typeof PlatformTableToolbar>['searchSuggestions'];
    status: () => Status;
    setStatus: (value: Status) => void;
    countForStatus: (value: Status) => number;
    visible: () => number;
    total: () => number;
    filtered: () => Resource[];
  };
  children: JSX.Element;
}) {
  return (
    <Show
      when={props.hasResources}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={props.tableState.search}
            onSearchChange={props.tableState.setSearch}
            searchPlaceholder={props.searchPlaceholder ?? `Search ${props.rowNoun}`}
            searchSuggestions={props.tableState.searchSuggestions}
            status={props.tableState.status()}
            onStatusChange={props.tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              (props.statusOptions ??
                PLATFORM_HEALTH_FILTER_OPTIONS) as readonly PlatformTableFilterOption<Status>[],
              props.tableState.countForStatus,
            )}
            visible={props.tableState.visible()}
            total={props.tableState.total()}
            rowNoun={props.rowNoun}
          />
        </Show>

        <Show
          when={props.tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title={props.filteredEmptyTitle ?? `No ${props.rowNoun} match current filters`}
              description={`Adjust the search or status filter to see more ${props.filteredEmptyNoun ?? props.rowNoun}.`}
            />
          }
        >
          {props.children}
        </Show>
      </div>
    </Show>
  );
}

// Events, ConfigMaps, Secrets, RBAC objects and namespace policies are
// records about a cluster, not objects Pulse monitors: they carry no alerts,
// history or maintenance of their own. Their drawer is a record panel headed
// by the object's name and its row's own status dot.
export const KubernetesRecordDetailPanel: Component<{
  testId: string;
  resource: Resource;
  title: string;
  status: StatusIndicator;
  summary?: JSX.Element;
  actions?: JSX.Element;
  /** Built against the shared relative-time clock so ages keep moving while open. */
  sections: (now: number) => Array<DetailSection | null>;
  onClose: () => void;
}> = (props) => {
  const now = useRelativeTimeNow();
  return (
    <InlineDetailPanel
      testId={props.testId}
      detailFor={props.resource.id}
      title={props.title}
      status={props.status}
      summary={props.summary}
      actions={props.actions}
      sections={compactDetailSections(props.sections(now()))}
      onClose={props.onClose}
    />
  );
};

export const kubernetesTimestampRow = makeDetailTimestampRow;

/** Where an object lives: its kind, namespace and cluster, and when it was made. */
export const kubernetesObjectSection = (
  resource: Resource,
  kind: string,
  now: number,
): DetailSection => ({
  label: 'Object',
  rows: compactDetailRows([
    makeDetailRow('Kind', kind),
    makeDetailRow('Namespace', asTrimmedString(resource.kubernetes?.namespace), { wrap: true }),
    makeDetailRow('Cluster', kubernetesClusterLabel(resource), { wrap: true }),
    kubernetesTimestampRow('Created', resource.kubernetes?.createdAt, now),
  ]),
});

// Every label, key above value, because keys such as
// app.kubernetes.io/managed-by are too long for the label column. Resources
// that only carry the registry's tags list them on one wrapped row.
export const kubernetesLabelsSection = (resource: Resource): DetailSection | null => {
  const labels = Object.entries(resource.kubernetes?.labels ?? {}).sort(([a], [b]) =>
    a.localeCompare(b),
  );
  if (labels.length > 0) {
    return {
      label: `Labels (${labels.length})`,
      rows: compactDetailRows(
        labels.map(([key, value]) =>
          makeDetailRow(key, asTrimmedString(value) || '(empty)', {
            layout: 'stacked',
            wrap: true,
          }),
        ),
      ),
    };
  }
  const tags = (resource.tags ?? []).map((tag) => asTrimmedString(tag)).filter(Boolean);
  if (tags.length === 0) return null;
  return {
    label: 'Labels',
    rows: compactDetailRows([makeDetailRow('Tags', tags.join(', '), { wrap: true })]),
  };
};

// The Kubernetes kinds whose objects a Kubernetes page lists and filters by
// name (KUBERNETES_TAB_BY_TYPE in resourcePagePath). An event about any other
// kind gets no link: there is no row to bring into view.
const KUBERNETES_KIND_RESOURCE_TYPES: Record<string, string> = {
  Node: 'k8s-node',
  Pod: 'pod',
  Deployment: 'k8s-deployment',
  StatefulSet: 'k8s-statefulset',
  DaemonSet: 'k8s-daemonset',
  ReplicaSet: 'k8s-replicaset',
  Job: 'k8s-job',
  CronJob: 'k8s-cronjob',
  Service: 'k8s-service',
  Ingress: 'k8s-ingress',
  EndpointSlice: 'k8s-endpoint-slice',
  PersistentVolume: 'k8s-persistent-volume',
  PersistentVolumeClaim: 'k8s-persistent-volume-claim',
  StorageClass: 'k8s-storage-class',
  Namespace: 'k8s-namespace',
  ConfigMap: 'k8s-configmap',
  Secret: 'k8s-secret',
  ServiceAccount: 'k8s-serviceaccount',
  Role: 'k8s-role',
  ClusterRole: 'k8s-cluster-role',
  RoleBinding: 'k8s-role-binding',
  ClusterRoleBinding: 'k8s-cluster-role-binding',
  ResourceQuota: 'k8s-resource-quota',
  NetworkPolicy: 'k8s-network-policy',
  LimitRange: 'k8s-limit-range',
  PodDisruptionBudget: 'k8s-pod-disruption-budget',
};

export const kubernetesEventInvolvedSubject = (resource: Resource): ResourcePageSubject | null => {
  const kind = asTrimmedString(resource.kubernetes?.involvedKind);
  const name = asTrimmedString(resource.kubernetes?.involvedName);
  const type = kind ? KUBERNETES_KIND_RESOURCE_TYPES[kind] : undefined;
  if (!name || !type) return null;
  const namespace = asTrimmedString(resource.kubernetes?.namespace);
  return { id: namespace ? `${namespace}/${name}` : name, name, type };
};

export const kubernetesEventInvolvedObject = (resource: Resource): string =>
  [resource.kubernetes?.involvedKind, resource.kubernetes?.involvedName]
    .map((part) => asTrimmedString(part))
    .filter(Boolean)
    .join('/');

export const kubernetesEventName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const buildEventDetailSections = (resource: Resource, now: number): DetailSection[] => {
  const k = resource.kubernetes;
  // Kubernetes keeps the first and last time it saw a repeating event;
  // Pulse stores the last one as the event's createdAt. The newer events API
  // reports one eventTime instead.
  const firstSeen = asTrimmedString(k?.firstSeen) || asTrimmedString(k?.eventTime);
  const lastSeen = asTrimmedString(k?.createdAt);
  const count = typeof k?.count === 'number' && k.count > 0 ? k.count : undefined;
  const severity = mapKubernetesEventSeverity(k?.eventType);
  return [
    {
      label: 'Event',
      rows: compactDetailRows([
        makeDetailRow('Reason', asTrimmedString(k?.reason), { wrap: true }),
        makeDetailRow('Type', severity.label, {
          tone: severity.variant === 'warning' ? 'warning' : undefined,
        }),
        count ? makeDetailRow('Count', count === 1 ? 'Once' : `${count} times`) : null,
        kubernetesTimestampRow(
          lastSeen && lastSeen !== firstSeen ? 'First seen' : 'Seen',
          firstSeen,
          now,
        ),
        lastSeen && lastSeen !== firstSeen
          ? kubernetesTimestampRow('Last seen', lastSeen, now)
          : null,
        makeDetailRow('Event name', kubernetesEventName(resource), { wrap: true }),
      ]),
    },
    {
      label: 'Object',
      rows: compactDetailRows([
        makeDetailRow('Kind', asTrimmedString(k?.involvedKind)),
        makeDetailRow('Name', asTrimmedString(k?.involvedName), { wrap: true }),
        makeDetailRow('Namespace', asTrimmedString(k?.namespace), { wrap: true }),
        makeDetailRow('Cluster', kubernetesClusterLabel(resource), { wrap: true }),
      ]),
    },
  ];
};

// An event is about another object: the drawer is headed by that object, says
// what happened in Kubernetes' own words, and links to the object where a
// Kubernetes page can filter to it.
export const KubernetesEventDetailPanel: Component<{
  resource: Resource;
  status: StatusIndicator;
  onClose: () => void;
}> = (props) => (
  <KubernetesRecordDetailPanel
    testId="kubernetes-event-detail"
    resource={props.resource}
    title={kubernetesEventInvolvedObject(props.resource) || kubernetesEventName(props.resource)}
    status={props.status}
    summary={
      asTrimmedString(props.resource.kubernetes?.message) ||
      asTrimmedString(props.resource.kubernetes?.reason) ||
      undefined
    }
    actions={
      <RecordOwnerLink
        subject={kubernetesEventInvolvedSubject(props.resource)}
        name={asTrimmedString(props.resource.kubernetes?.involvedName)}
      />
    }
    sections={(now) => buildEventDetailSections(props.resource, now)}
    onClose={props.onClose}
  />
);
