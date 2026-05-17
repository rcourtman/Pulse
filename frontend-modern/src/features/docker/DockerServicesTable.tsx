import { For, Show, type Component, type JSX } from 'solid-js';
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
  createPlatformTableFilterState,
  filterPlatformResources,
  getPlatformTableCellClass,
  getPlatformTableHeadClass,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';

// Docker Swarm services are cluster-scoped declarations, not running
// processes — they have no CPU / Memory / Disk / Disk I/O / Uptime /
// Temperature of their own (those metrics live on the controlled tasks
// and the underlying nodes). The canonical infrastructure table renders
// dashes for those columns on docker-service rows. This service-native
// table reuses canonical shared primitives (Card, Table, SearchInput,
// FilterButtonGroup, StatusDot) but surfaces operator columns that the
// data actually backs: image, mode, replica counts, ports, host.

const formatPorts = (ports: Resource['docker'] extends infer T ? T : never): string => {
  const entries =
    (
      ports as {
        endpointPorts?: Array<{ publishedPort?: number; targetPort?: number; protocol?: string }>;
      }
    )?.endpointPorts ?? [];
  if (entries.length === 0) return '—';
  return (
    entries
      .map((entry) => {
        const protocol = entry?.protocol ? `/${entry.protocol.toLowerCase()}` : '';
        if (entry?.publishedPort && entry?.targetPort) {
          return `${entry.publishedPort}:${entry.targetPort}${protocol}`;
        }
        const single = entry?.publishedPort ?? entry?.targetPort;
        return single ? `${single}${protocol}` : '';
      })
      .filter((part) => part.length > 0)
      .join(', ') || '—'
  );
};

const replicaCount = (value: number | undefined): JSX.Element => (
  <span class="tabular-nums">{value ?? 0}</span>
);

export const DockerServicesTable: Component<{
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
            searchPlaceholder="Search Swarm services"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="services"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No services match current filters"
              description="Adjust the search or status filter to see more services."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Swarm Services'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[900px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClass()}>Service</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Image
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass()}>Mode</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Desired
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Running</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Ports
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Host
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(service) => {
                    const name = () => asTrimmedString(service.name) || service.id;
                    const image = () => asTrimmedString(service.docker?.image) || '—';
                    const mode = () => asTrimmedString(service.docker?.mode) || '—';
                    const host = () => asTrimmedString(service.docker?.hostname) || '—';
                    const indicator = () => getSimpleStatusIndicator(service.status);
                    return (
                      <TableRow class="text-[11px] sm:text-xs">
                        <TableCell class={getPlatformTableCellClass()}>
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={service.status || 'unknown'}
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
                          <span class="truncate inline-block max-w-[18rem]" title={image()}>
                            {image()}
                          </span>
                        </TableCell>
                        <TableCell class={`${getPlatformTableCellClass()} text-base-content`}>
                          {mode()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                        >
                          {replicaCount(service.docker?.desiredTasks)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {replicaCount(service.docker?.runningTasks)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          <span class="font-mono text-[11px]" title={formatPorts(service.docker)}>
                            {formatPorts(service.docker)}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          {host()}
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

export default DockerServicesTable;
