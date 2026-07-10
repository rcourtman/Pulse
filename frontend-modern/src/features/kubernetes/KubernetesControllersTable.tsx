import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableNumberValue,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTextValue,
  getPlatformTableCellClassForKind,
  PlatformTableShell,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  compareKubernetesControllers,
  filterKubernetesResources,
  kubernetesScopeLabel,
  mapKubernetesControllerStatus,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

const controllerName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const controllerKind = (resource: Resource): string =>
  resource.kubernetes?.resourceKind || getResourceTypeLabel(resource.type) || resource.type;

const targetValue = (resource: Resource): string => {
  switch (resource.type) {
    case 'k8s-replicaset':
    case 'k8s-statefulset':
      return `${resource.kubernetes?.desiredReplicas ?? 0} pods`;
    case 'k8s-daemonset':
      return `${resource.kubernetes?.desiredNumberScheduled ?? 0} nodes`;
    case 'k8s-job':
      return `${resource.kubernetes?.desiredReplicas ?? 0} completions`;
    case 'k8s-cronjob':
      return formatPlatformTableTextValue(resource.kubernetes?.schedule);
    default:
      return '—';
  }
};

const currentValue = (resource: Resource): number | undefined => {
  switch (resource.type) {
    case 'k8s-daemonset':
      return resource.kubernetes?.currentNumberScheduled;
    case 'k8s-job':
    case 'k8s-cronjob':
      return resource.kubernetes?.active;
    default:
      return resource.kubernetes?.currentReplicas;
  }
};

const readyOrDoneValue = (resource: Resource): number | undefined => {
  switch (resource.type) {
    case 'k8s-daemonset':
      return resource.kubernetes?.numberReady;
    case 'k8s-job':
      return resource.kubernetes?.succeeded;
    case 'k8s-cronjob':
      return undefined;
    default:
      return resource.kubernetes?.readyReplicas;
  }
};

const availableValue = (resource: Resource): number | undefined => {
  switch (resource.type) {
    case 'k8s-replicaset':
    case 'k8s-statefulset':
      return resource.kubernetes?.availableReplicas;
    case 'k8s-daemonset':
      return resource.kubernetes?.numberAvailable;
    default:
      return undefined;
  }
};

const exceptionSummary = (resource: Resource): string => {
  switch (resource.type) {
    case 'k8s-replicaset':
    case 'k8s-statefulset': {
      const desired = resource.kubernetes?.desiredReplicas ?? 0;
      const ready = resource.kubernetes?.readyReplicas ?? 0;
      const notReady = Math.max(0, desired - ready);
      return notReady > 0 ? `${notReady} not ready` : '—';
    }
    case 'k8s-daemonset': {
      const unavailable = resource.kubernetes?.numberUnavailable ?? 0;
      const misscheduled = resource.kubernetes?.numberMisscheduled ?? 0;
      if (unavailable === 0 && misscheduled === 0) return '—';
      return `Unavailable: ${unavailable} / Misscheduled: ${misscheduled}`;
    }
    case 'k8s-job': {
      const failed = resource.kubernetes?.failed ?? 0;
      return failed > 0 ? `Failed: ${failed}` : '—';
    }
    case 'k8s-cronjob':
      return resource.kubernetes?.suspend ? 'Suspended' : '—';
    default:
      return '—';
  }
};

const apiDetail = (resource: Resource): string => {
  switch (resource.type) {
    case 'k8s-replicaset': {
      if (typeof resource.kubernetes?.fullyLabeledReplicas === 'number') {
        return `Fully labeled: ${resource.kubernetes.fullyLabeledReplicas}`;
      }
      return typeof resource.kubernetes?.observedGeneration === 'number'
        ? `Observed: ${resource.kubernetes.observedGeneration}`
        : '—';
    }
    case 'k8s-statefulset':
      return resource.kubernetes?.serviceName ? `Service: ${resource.kubernetes.serviceName}` : '—';
    case 'k8s-daemonset':
      return typeof resource.kubernetes?.updatedReplicas === 'number'
        ? `Updated: ${resource.kubernetes.updatedReplicas}`
        : '—';
    case 'k8s-job':
      if (resource.kubernetes?.completionTime) {
        return `Completed: ${resource.kubernetes.completionTime}`;
      }
      return resource.kubernetes?.startTime ? `Started: ${resource.kubernetes.startTime}` : '—';
    case 'k8s-cronjob':
      if (resource.kubernetes?.lastSuccessfulTime) {
        return `Last success: ${resource.kubernetes.lastSuccessfulTime}`;
      }
      return resource.kubernetes?.lastScheduleTime
        ? `Last schedule: ${resource.kubernetes.lastScheduleTime}`
        : '—';
    default:
      return '—';
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
    <Show
      when={props.resources.length > 0}
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
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search controllers"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="controllers"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No controllers match current filters"
              description="Adjust the search or status filter to see more Kubernetes controllers."
            />
          }
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
                  class="md:w-[18%]"
                >
                  Controller
                </PlatformSortableTableHead>
                <PlatformSortableTableHead kind="text" sort={sort} sortKey="kind" class="md:w-[9%]">
                  Kind
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="scope"
                  class="hidden md:table-cell md:w-[14%]"
                >
                  Scope
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="target"
                  class="md:w-[12%]"
                >
                  Target
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="current"
                  class="md:w-[8%]"
                >
                  Current
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="ready"
                  class="md:w-[10%]"
                >
                  Ready/Done
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="available"
                  class="hidden md:table-cell md:w-[9%]"
                >
                  Available
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="exceptions"
                  class="md:w-[11%]"
                >
                  Exceptions
                </PlatformSortableTableHead>
                <PlatformSortableTableHead kind="text" sort={sort} class="hidden md:table-cell md:w-[9%]">
                  Detail
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(resource) => {
                    const indicator = () => mapKubernetesControllerStatus(resource);
                    const name = () => controllerName(resource);
                    const kind = () => controllerKind(resource);
                    const scope = () => kubernetesScopeLabel(resource);
                    const target = () => targetValue(resource);
                    const exceptions = () => exceptionSummary(resource);
                    const detail = () => apiDetail(resource);
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-controller-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                          onKeyDown={drawer.handleActivationKey(resource)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
                                ariaHidden
                              />
                              <span class="truncate font-semibold text-base-content" title={name()}>
                                {name()}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {kind()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="inline-block max-w-[12rem] truncate" title={scope()}>
                              {scope()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span class="inline-block max-w-[12rem] truncate" title={target()}>
                              {target()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
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
                            <span class="inline-block max-w-[13rem] truncate" title={exceptions()}>
                              {exceptions()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="inline-block max-w-[14rem] truncate" title={detail()}>
                              {detail()}
                            </span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={9}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource)}
                        />
                      </>
                    );
                  }}
                </For>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default KubernetesControllersTable;
