import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import {
  compareKubernetesControllers,
  filterKubernetesResources,
  mapKubernetesControllerStatus,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';

const numberValue = (value: number | undefined): JSX.Element =>
  typeof value === 'number' ? <span class="tabular-nums">{value}</span> : <span>—</span>;

const controllerName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const controllerKind = (resource: Resource): string =>
  resource.kubernetes?.resourceKind || getResourceTypeLabel(resource.type) || resource.type;

const controllerScope = (resource: Resource): string => {
  const cluster =
    asTrimmedString(resource.kubernetes?.clusterId) ||
    asTrimmedString(resource.kubernetes?.clusterName);
  const namespace = asTrimmedString(resource.kubernetes?.namespace);
  if (namespace) return cluster ? `${cluster}/${namespace}` : namespace;
  return cluster || 'Cluster';
};

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
      return textValue(resource.kubernetes?.schedule);
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

export const KubernetesControllersTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: filterKubernetesResources,
  });
  const sortedRows = createMemo(() =>
    [...tableState.filtered()].sort(compareKubernetesControllers),
  );

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
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Workload Controllers'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1120px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[18%]`}>
                    Controller
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[9%]`}>
                    Kind
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                  >
                    Scope
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[12%]`}>
                    Target
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[8%]`}
                  >
                    Current
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}
                  >
                    Ready/Done
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[9%]`}
                  >
                    Available
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[11%]`}>
                    Exceptions
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[9%]`}
                  >
                    Detail
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={sortedRows()}>
                  {(resource) => {
                    const indicator = () => mapKubernetesControllerStatus(resource);
                    const name = () => controllerName(resource);
                    const kind = () => controllerKind(resource);
                    const scope = () => controllerScope(resource);
                    const target = () => targetValue(resource);
                    const exceptions = () => exceptionSummary(resource);
                    const detail = () => apiDetail(resource);

                    return (
                      <TableRow
                        class="text-[11px] sm:text-xs"
                        data-kubernetes-controller-row={resource.id}
                      >
                        <TableCell class={getPlatformTableCellClassForKind('name')}>
                          <div class="flex min-w-0 items-center gap-2">
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
                          {numberValue(currentValue(resource))}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {numberValue(readyOrDoneValue(resource))}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                        >
                          {numberValue(availableValue(resource))}
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
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default KubernetesControllersTable;
