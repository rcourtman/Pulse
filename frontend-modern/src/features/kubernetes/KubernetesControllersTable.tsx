import { createMemo, type Component, type JSX } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  PlatformTableNumberValue,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableDateTimeValue,
  formatPlatformTableRelativeTimeValue,
  formatPlatformTableTextValue,
  getPlatformResourceCountNoun,
  getPlatformTableCellClassForKind,
  PlatformTableShell,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import { getPlatformStatusTextToneClass } from '@/features/platformPage/platformStatusTone';
import {
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import {
  compareKubernetesControllers,
  filterKubernetesResources,
  hasStaleKubernetesReadings,
  kubernetesScopeLabel,
  mapKubernetesControllerStatus,
  type KubernetesResourceStatusFilter,
  isSingleKubernetesCluster,
} from './kubernetesPageModel';
import { KubernetesScopeText } from './KubernetesScopeText';
import { KubernetesNameText } from './KubernetesNameText';
import {
  KubernetesDetailRow,
  KubernetesRowLead,
  KubernetesStaleDataCue,
  KubernetesTableFrame,
} from './KubernetesRecordDetail';

const controllerName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const controllerKind = (resource: Resource): string =>
  resource.kubernetes?.resourceKind || getResourceTypeLabel(resource.type) || resource.type;

const countedTarget = (count: number | undefined, noun: string): string => {
  const value = count ?? 0;
  return `${value} ${getPlatformResourceCountNoun(noun, value)}`;
};

const targetValue = (resource: Resource): string => {
  const k = resource.kubernetes;
  switch (resource.type) {
    case 'k8s-replicaset':
    case 'k8s-statefulset':
      return countedTarget(k?.desiredReplicas, 'pods');
    case 'k8s-daemonset':
      return countedTarget(k?.desiredNumberScheduled, 'nodes');
    case 'k8s-job':
      return countedTarget(k?.desiredReplicas, 'completions');
    case 'k8s-cronjob':
      return formatPlatformTableTextValue(k?.schedule);
    default:
      return '—';
  }
};

const currentValue = (resource: Resource): number | undefined => {
  const k = resource.kubernetes;
  if (resource.type === 'k8s-daemonset') return k?.currentNumberScheduled;
  if (resource.type === 'k8s-job' || resource.type === 'k8s-cronjob') return k?.active;
  return k?.currentReplicas;
};

const readyOrDoneValue = (resource: Resource): number | undefined => {
  const k = resource.kubernetes;
  if (resource.type === 'k8s-daemonset') return k?.numberReady;
  if (resource.type === 'k8s-job') return k?.succeeded;
  return resource.type === 'k8s-cronjob' ? undefined : k?.readyReplicas;
};

const availableValue = (resource: Resource): number | undefined => {
  const k = resource.kubernetes;
  if (resource.type === 'k8s-replicaset' || resource.type === 'k8s-statefulset') {
    return k?.availableReplicas;
  }
  return resource.type === 'k8s-daemonset' ? k?.numberAvailable : undefined;
};

const exceptionSummary = (resource: Resource): string => {
  const k = resource.kubernetes;
  switch (resource.type) {
    case 'k8s-replicaset':
    case 'k8s-statefulset': {
      const notReady = Math.max(0, (k?.desiredReplicas ?? 0) - (k?.readyReplicas ?? 0));
      return notReady > 0 ? `${notReady} not ready` : '—';
    }
    case 'k8s-daemonset': {
      const unavailable = k?.numberUnavailable ?? 0;
      const misscheduled = k?.numberMisscheduled ?? 0;
      return unavailable === 0 && misscheduled === 0
        ? '—'
        : `Unavailable: ${unavailable} / Misscheduled: ${misscheduled}`;
    }
    case 'k8s-job': {
      const failed = k?.failed ?? 0;
      return failed > 0 ? `Failed: ${failed}` : '—';
    }
    case 'k8s-cronjob':
      return k?.suspend ? 'Suspended' : '—';
    default:
      return '—';
  }
};

type ControllerDetail = { text: string; title: string };

const plainDetail = (text: string): ControllerDetail => ({ text, title: text });

// Kubernetes reports job timestamps as RFC 3339 strings with microseconds,
// which never fit the Detail column. The cell says how long ago the event
// happened, measured from the shared clock so it keeps moving while the row
// stays mounted, and keeps the absolute time on hover.
const timeDetail = (label: string, timestamp: string, now: number): ControllerDetail => {
  const relative = formatPlatformTableRelativeTimeValue(timestamp, { emptyText: '', now });
  if (!relative) return plainDetail(`${label}: ${timestamp}`);
  const absolute = formatPlatformTableDateTimeValue(timestamp, {
    dateTimeFormat: { year: 'numeric' },
  });
  return { text: `${label} ${relative}`, title: `${label}: ${absolute}` };
};

const apiDetail = (resource: Resource, now: number): ControllerDetail => {
  const k = resource.kubernetes;
  switch (resource.type) {
    case 'k8s-replicaset':
      if (typeof k?.fullyLabeledReplicas === 'number') {
        return plainDetail(`Fully labeled: ${k.fullyLabeledReplicas}`);
      }
      return plainDetail(
        typeof k?.observedGeneration === 'number' ? `Observed: ${k.observedGeneration}` : '—',
      );
    case 'k8s-statefulset':
      return plainDetail(k?.serviceName ? `Service: ${k.serviceName}` : '—');
    case 'k8s-daemonset':
      return plainDetail(
        typeof k?.updatedReplicas === 'number' ? `Updated: ${k.updatedReplicas}` : '—',
      );
    case 'k8s-job':
      if (k?.completionTime) return timeDetail('Completed', k.completionTime, now);
      return k?.startTime ? timeDetail('Started', k.startTime, now) : plainDetail('—');
    case 'k8s-cronjob':
      if (k?.lastSuccessfulTime) return timeDetail('Last success', k.lastSuccessfulTime, now);
      return k?.lastScheduleTime
        ? timeDetail('Last run', k.lastScheduleTime, now)
        : plainDetail('—');
    default:
      return plainDetail('—');
  }
};

// The Detail column is a per-kind grab-bag (service names, timestamps,
// generation counters) with no single scalar meaning across rows, so it
// stays non-sortable.
const KUBERNETES_CONTROLLER_SORT_KEYS = [
  'controller',
  'kind',
  'scope',
  'target',
  'current',
  'ready',
  'available',
  'exceptions',
] as const;

type KubernetesControllerSortKey = (typeof KUBERNETES_CONTROLLER_SORT_KEYS)[number];

const getKubernetesControllerSortValue = (
  resource: Resource,
  key: KubernetesControllerSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'controller':
      return controllerName(resource);
    case 'kind':
      return controllerKind(resource);
    case 'scope':
      return kubernetesScopeLabel(resource);
    case 'target': {
      const target = targetValue(resource);
      return target === '—' ? null : target;
    }
    case 'current':
      return currentValue(resource) ?? null;
    case 'ready':
      return readyOrDoneValue(resource) ?? null;
    case 'available':
      return availableValue(resource) ?? null;
    case 'exceptions': {
      const exceptions = exceptionSummary(resource);
      return exceptions === '—' ? null : exceptions;
    }
    default:
      key satisfies never;
      return null;
  }
};

export const KubernetesControllersTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
  externalSearch?: () => string;
  externalStatus?: () => KubernetesResourceStatusFilter;
}> = (props) => {
  const now = useRelativeTimeNow();
  const singleCluster = createMemo(() => isSingleKubernetesCluster(props.resources));
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: filterKubernetesResources,
    externalSearch: props.externalSearch,
    externalStatus: props.externalStatus,
  });
  // User-controlled sorting layered over the attention-first default: rows
  // are pre-sorted by the status compare, so a user sort keeps that order
  // for ties and the table falls straight back to it when the sort clears.
  const sort = createPlatformTableSortState({
    storageKey: 'kubernetesControllers',
    sortKeys: KUBERNETES_CONTROLLER_SORT_KEYS,
    descendingFirst: ['current', 'ready', 'available'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(
      [...tableState.filtered()].sort(compareKubernetesControllers),
      getKubernetesControllerSortValue,
    ),
  );
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-controller-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);

  return (
    <KubernetesTableFrame
      hasResources={props.resources.length > 0}
      emptyIcon={props.emptyIcon}
      emptyTitle={props.emptyTitle}
      emptyDescription={props.emptyDescription}
      showToolbar={props.showToolbar}
      rowNoun="controllers"
      filteredEmptyNoun="Kubernetes controllers"
      tableState={tableState}
    >
      <PlatformTableShell
        title={props.title ?? 'Workload Controllers'}
        tableClass="min-w-full table-fixed text-xs md:min-w-[1120px]"
        header={
          <>
            <PlatformSortableTableHead
              kind="name"
              sort={sort}
              sortKey="controller"
              class="platform-table-mobile-w-30 md:w-[22%]"
            >
              Controller
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              sortKey="kind"
              class="platform-table-mobile-w-25 md:w-[8%]"
            >
              Kind
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              sortKey="scope"
              class="hidden lg:table-cell md:w-[14%]"
            >
              Scope
            </PlatformSortableTableHead>
            {/* The phone row keeps controller, kind, ready and issues. A
                    15% kind track clipped "DaemonSet" and "StatefulSet" and the
                    target track clipped "1 completions", so target moves to the
                    row expansion, which also carries the schedule and job
                    timestamps the narrower layouts drop. */}
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              sortKey="target"
              class="platform-table-phone-hidden md:w-[12%]"
            >
              Target
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="numeric-value"
              sort={sort}
              sortKey="current"
              class="hidden sm:table-cell md:w-[7%]"
            >
              Current
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="numeric-value"
              sort={sort}
              sortKey="ready"
              class="platform-table-mobile-w-15 md:w-[10%]"
            >
              <PlatformResponsiveTableLabel compact="Ready" full="Ready/Done" />
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="numeric-value"
              sort={sort}
              sortKey="available"
              class="hidden md:table-cell md:w-[8%]"
            >
              Available
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              sortKey="exceptions"
              class="platform-table-mobile-w-30 md:w-[11%]"
            >
              <PlatformResponsiveTableLabel compact="Issues" full="Exceptions" />
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              class="hidden lg:table-cell md:w-[12%]"
            >
              Detail
            </PlatformSortableTableHead>
          </>
        }
        body={
          <>
            <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
              {(resource) => {
                const indicator = () => mapKubernetesControllerStatus(resource);
                const readingsStale = () => hasStaleKubernetesReadings(resource);
                const name = () => controllerName(resource);
                const kind = () => controllerKind(resource);
                const target = () => targetValue(resource);
                const exceptions = () => exceptionSummary(resource);
                const detail = () => apiDetail(resource, now());
                const isExpanded = () => drawer.isExpanded(resource);

                return (
                  <>
                    <TableRow
                      class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                      data-kubernetes-controller-row={resource.id}
                      onClick={() => drawer.toggle(resource)}
                    >
                      <TableCell class={getPlatformTableCellClassForKind('name')}>
                        <KubernetesRowLead
                          resource={resource}
                          drawer={drawer}
                          label={name()}
                          status={indicator()}
                        >
                          <div class="min-w-0">
                            <KubernetesNameText
                              name={name()}
                              class="font-semibold text-base-content"
                            />
                            <KubernetesStaleDataCue when={readingsStale()} scope="controller" />
                          </div>
                        </KubernetesRowLead>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        {kind()}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                      >
                        <KubernetesScopeText
                          resource={resource}
                          singleCluster={singleCluster()}
                          class="block truncate"
                        />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden text-base-content`}
                      >
                        <span class="block truncate" title={target()}>
                          {target()}
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content sm:table-cell`}
                      >
                        <PlatformTableNumberValue value={currentValue(resource)} />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                      >
                        <PlatformTableNumberValue value={readyOrDoneValue(resource)} />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                      >
                        <PlatformTableNumberValue value={availableValue(resource)} />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        {/* The exception is why the dot is not green: its colour. */}
                        <span
                          class={`block truncate ${
                            exceptions() === '—'
                              ? ''
                              : getPlatformStatusTextToneClass(indicator().variant)
                          }`}
                          title={exceptions()}
                          data-kubernetes-controller-exception={
                            exceptions() === '—' ? 'none' : indicator().variant
                          }
                        >
                          {exceptions()}
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                      >
                        <span class="block truncate" title={detail().title}>
                          {detail().text}
                        </span>
                      </TableCell>
                    </TableRow>
                    <KubernetesDetailRow
                      resource={resource}
                      statusIndicator={indicator()}
                      drawer={drawer}
                      colSpan={9}
                      resolveResourceLabel={resolveResourceLabel}
                    />
                  </>
                );
              }}
            </PlatformWindowedRows>
          </>
        }
      />
    </KubernetesTableFrame>
  );
};

export default KubernetesControllersTable;
