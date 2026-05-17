import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
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
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  filterPlatformResources,
  getPlatformTableCellClass,
  getPlatformTableHeadClass,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';

// Kubernetes Deployments are scheduling abstractions over their controlled
// pods, so the generic infrastructure table's CPU / Memory / Disk I/O /
// Uptime / Temperature columns are conceptually N/A on these rows and
// render as dashes. This deployment-native table reuses canonical shared
// primitives (Card, Table, SearchInput, FilterButtonGroup, StatusDot) but
// surfaces deployment-meaningful columns only: namespace, cluster,
// desired / updated / ready / available replicas.

const replicaCount = (value: number | undefined): JSX.Element => (
  <span class="tabular-nums">{value ?? 0}</span>
);

export const KubernetesDeploymentsTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');

  const filtered = createMemo(() => filterPlatformResources(props.resources, search(), status()));
  const visible = createMemo(() => filtered().length);
  const total = createMemo(() => props.resources.length);

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
            search={search}
            onSearchChange={setSearch}
            searchPlaceholder="Search deployments"
            status={status()}
            onStatusChange={setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={visible()}
            total={total()}
            rowNoun="deployments"
          />
        </Show>

        <Show
          when={filtered().length > 0}
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
            <Table class="min-w-full table-fixed text-xs md:min-w-[820px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClass()}>Deployment</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Namespace
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Cluster
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Desired
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Updated
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Ready</TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Available</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={filtered()}>
                  {(deployment) => {
                    const name = () => asTrimmedString(deployment.name) || deployment.id;
                    const ns = () => asTrimmedString(deployment.kubernetes?.namespace) || '—';
                    const cluster = () =>
                      asTrimmedString(deployment.kubernetes?.clusterName) ||
                      asTrimmedString(deployment.kubernetes?.clusterId) ||
                      '—';
                    const indicator = () => getSimpleStatusIndicator(deployment.status);
                    return (
                      <TableRow class="text-[11px] sm:text-xs">
                        <TableCell class={getPlatformTableCellClass()}>
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={deployment.status || 'unknown'}
                              ariaHidden
                            />
                            <span class="truncate font-semibold text-base-content" title={name()}>
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {ns()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {cluster()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                        >
                          {replicaCount(deployment.kubernetes?.desiredReplicas)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                        >
                          {replicaCount(deployment.kubernetes?.updatedReplicas)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {replicaCount(deployment.kubernetes?.readyReplicas)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {replicaCount(deployment.kubernetes?.availableReplicas)}
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
