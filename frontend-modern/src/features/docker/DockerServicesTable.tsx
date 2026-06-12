import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import {
  compareDockerServices,
  dockerServiceStack,
  filterDockerResources,
  mapDockerServiceStatus,
  type DockerResourceStatusFilter,
} from './dockerPageModel';

// Docker Swarm services are cluster-scoped declarations, not running
// processes — they have no CPU / Memory / Disk / Disk I/O / Uptime /
// Temperature of their own (those metrics live on the controlled tasks
// and the underlying nodes). The canonical infrastructure table renders
// dashes for those columns on docker-service rows. This service-native
// table reuses canonical shared primitives (Card, Table, SearchInput,
// FilterButtonGroup, StatusDot) but surfaces operator columns that the
// data actually backs: image, mode, replica counts, update state, ports, host.

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

const formatServiceUpdate = (
  update: NonNullable<Resource['docker']>['serviceUpdate'],
): { label: string; title: string } => {
  const state = asTrimmedString(update?.state);
  const message = asTrimmedString(update?.message);
  const completedAt = asTrimmedString(update?.completedAt);
  if (!state && !message && !completedAt) {
    return { label: 'Stable', title: 'No active service update reported' };
  }

  const label = state || 'Updating';
  const title = [state, message, completedAt].filter(Boolean).join(' | ') || label;
  return { label, title };
};

export const DockerServicesTable: Component<{
  resources: Resource[];
  sourceCount?: number;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
  });
  const sortedRows = createMemo(() => [...tableState.filtered()].sort(compareDockerServices));

  const hasFilteredSourceRows = () => (props.sourceCount ?? props.resources.length) > 0;

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={
            hasFilteredSourceRows() ? 'No Swarm services match current filters' : props.emptyTitle
          }
          description={
            hasFilteredSourceRows()
              ? 'Adjust the shared Docker page filters to see more services.'
              : props.emptyDescription
          }
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
          <PlatformTableShell
            title={props.title ?? 'Swarm Services'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1320px]"
            header={
              <>
                {/*
                    Desktop widths: Service and Image take the lion's share
                    because their content is long (registry refs, fully
                    qualified service names). Mode / Desired / Running trim
                    to short text and 1-2 digit counts. Update, Ports, and
                    Host get middle slices for rollout state, port lists, and
                    hostnames. Mobile widths are unchanged.
                  */}
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[16%]`}>
                  Service
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[9%]`}
                >
                  Stack
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[19%]`}
                >
                  Image
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[8%]`}>
                  Mode
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[8%]`}
                >
                  Desired
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[8%]`}>
                  Running
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[12%]`}
                >
                  Update
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                >
                  Ports
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                >
                  Host
                </TableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(service) => {
                    const name = () => asTrimmedString(service.name) || service.id;
                    const stack = () => dockerServiceStack(service) || '—';
                    const image = () => asTrimmedString(service.docker?.image) || '—';
                    const mode = () => asTrimmedString(service.docker?.mode) || '—';
                    const host = () => asTrimmedString(service.docker?.hostname) || '—';
                    const indicator = () => mapDockerServiceStatus(service);
                    const update = () => formatServiceUpdate(service.docker?.serviceUpdate);
                    return (
                      <TableRow class="text-[11px] sm:text-xs" data-docker-service-row={service.id}>
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
                          <span class="truncate inline-block max-w-[8rem]" title={stack()}>
                            {stack()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="truncate inline-block max-w-[18rem]" title={image()}>
                            {image()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          {mode()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                        >
                          {replicaCount(service.docker?.desiredTasks)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {replicaCount(service.docker?.runningTasks)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="truncate inline-block max-w-[10rem]" title={update().title}>
                            {update().label}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="font-mono text-[11px]" title={formatPorts(service.docker)}>
                            {formatPorts(service.docker)}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          {host()}
                        </TableCell>
                      </TableRow>
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

export default DockerServicesTable;
