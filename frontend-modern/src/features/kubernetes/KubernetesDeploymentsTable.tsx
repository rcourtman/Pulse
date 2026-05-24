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
import { asTrimmedString } from '@/utils/stringUtils';
import { formatRelativeTime } from '@/utils/format';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  createPlatformTableFilterState,
  filterPlatformResources,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import {
  compareKubernetesDeployments,
  mapKubernetesDeploymentStatus,
} from './kubernetesPageModel';

// Kubernetes Deployments are scheduling abstractions over their controlled
// pods, so the generic infrastructure table's CPU / Memory / Disk I/O /
// Uptime / Temperature columns are conceptually N/A on these rows and
// render as dashes. This deployment-native table reuses canonical shared
// primitives (Card, Table, SearchInput, FilterButtonGroup, StatusDot) but
// surfaces deployment-meaningful columns only: namespace, cluster,
// desired / updated / ready / available replicas, observed generation, and
// metadata age.

const replicaCount = (value: number | undefined): JSX.Element => (
  <span class="tabular-nums">{value ?? 0}</span>
);

const formatAge = (createdAt: string | undefined): string =>
  formatRelativeTime(createdAt, { compact: true, emptyText: '—' }) || '—';

export const KubernetesDeploymentsTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterPlatformResources,
  });
  const sortedRows = createMemo(() =>
    [...tableState.filtered()].sort(compareKubernetesDeployments),
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
            searchPlaceholder="Search deployments"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="deployments"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No deployments match current filters"
              description="Adjust the search or status filter to see more deployments."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Deployments'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1320px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  {/*
                    Desktop widths: Deployment, Namespace, and Cluster take
                    the biggest shares because their content can be long.
                    The integer-count columns (Desired / Updated / Ready /
                    Available / Observed) trim to what their headers plus
                    1-2 digit values need. Mobile widths are unchanged.
                  */}
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[22%]`}>
                    Deployment
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[17%]`}
                  >
                    Namespace
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                  >
                    Cluster
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[8%]`}
                  >
                    Desired
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[8%]`}
                  >
                    Updated
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[7%]`}
                  >
                    Ready
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[9%]`}
                  >
                    Available
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[9%]`}
                  >
                    Observed
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[6%]`}
                  >
                    Age
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={sortedRows()}>
                  {(deployment) => {
                    const name = () => asTrimmedString(deployment.name) || deployment.id;
                    const ns = () => asTrimmedString(deployment.kubernetes?.namespace) || '—';
                    const cluster = () =>
                      asTrimmedString(deployment.kubernetes?.clusterName) ||
                      asTrimmedString(deployment.kubernetes?.clusterId) ||
                      '—';
                    const indicator = () => mapKubernetesDeploymentStatus(deployment);
                    const age = () => formatAge(deployment.kubernetes?.createdAt);
                    return (
                      <TableRow
                        class="text-[11px] sm:text-xs"
                        data-kubernetes-deployment-row={deployment.id}
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
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          {ns()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          {cluster()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                        >
                          {replicaCount(deployment.kubernetes?.desiredReplicas)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                        >
                          {replicaCount(deployment.kubernetes?.updatedReplicas)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {replicaCount(deployment.kubernetes?.readyReplicas)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {replicaCount(deployment.kubernetes?.availableReplicas)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                        >
                          {replicaCount(deployment.kubernetes?.observedGeneration)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          <span class="tabular-nums" title={deployment.kubernetes?.createdAt || ''}>
                            {age()}
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

export default KubernetesDeploymentsTable;
