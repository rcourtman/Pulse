import { For, Show, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { formatBytes } from '@/utils/format';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import type { DockerStorageUsageMeta, Resource } from '@/types/resource';
import {
  filterDockerResources,
  hasDockerEngineStorageUsage,
  hasDockerStorageUsageBucket,
  type DockerResourceStatusFilter,
} from './dockerPageModel';

const bucketValue = (bucket?: DockerStorageUsageMeta): JSX.Element => {
  if (!hasDockerStorageUsageBucket(bucket)) return <span class="text-muted">—</span>;
  const totalSize = bucket?.totalSizeBytes ?? 0;
  const reclaimable = bucket?.reclaimableBytes ?? 0;
  const count = bucket?.totalCount ?? 0;
  const active = bucket?.activeCount ?? 0;
  return (
    <span class="inline-flex min-w-0 flex-col items-end leading-tight">
      <span class="tabular-nums text-base-content">{formatBytes(totalSize)}</span>
      <span class="truncate text-[10px] text-muted">
        {count} total, {active} active, {formatBytes(reclaimable)} reclaimable
      </span>
    </span>
  );
};

export const DockerStorageUsageTable: Component<{
  hosts: Resource[];
  sourceCount?: number;
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
  externalSearch?: () => string;
  externalStatus?: () => DockerResourceStatusFilter;
}> = (props) => {
  const storageHosts = () => props.hosts.filter(hasDockerEngineStorageUsage);
  const tableState = createPlatformTableFilterState({
    resources: storageHosts,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
    externalSearch: props.externalSearch,
    externalStatus: props.externalStatus,
  });
  const hasFilteredSourceRows = () => (props.sourceCount ?? props.hosts.length) > 0;

  return (
    <Show
      when={storageHosts().length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={hasFilteredSourceRows() ? 'No engine storage usage reported' : props.emptyTitle}
          description={
            hasFilteredSourceRows()
              ? 'Hosts are present, but none have reported the Docker / Podman disk-usage snapshot yet.'
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
            searchPlaceholder="Search storage usage"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="hosts"
            hasActiveFilters={tableState.hasActiveFilters()}
            onResetFilters={tableState.resetFilters}
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No storage rows match current filters"
              description="Adjust the search or status filter to see more rows."
            />
          }
        >
          <PlatformTableShell
            title="Engine Storage Usage"
            tableClass="min-w-full table-fixed text-xs md:min-w-[1080px]"
            header={
              <>
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[22%]`}>
                  Host
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[19%]`}
                >
                  Images
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[19%]`}
                >
                  Containers
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[20%]`}
                >
                  Volumes
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[20%]`}
                >
                  Build Cache
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
                  {(host) => {
                    const name = () => asTrimmedString(host.name) || host.id;
                    const indicator = () => getSimpleStatusIndicator(host.status);
                    return (
                      <TableRow class="text-[11px] sm:text-xs" data-docker-storage-row={host.id}>
                        <TableCell class={getPlatformTableCellClassForKind('name')}>
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={host.status || 'unknown'}
                              ariaHidden
                            />
                            <span class="truncate font-semibold text-base-content" title={name()}>
                              {name()}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {bucketValue(host.docker?.imagesUsage)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {bucketValue(host.docker?.containersUsage)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                        >
                          {bucketValue(host.docker?.volumesUsage)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                        >
                          {bucketValue(host.docker?.buildCacheUsage)}
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

export default DockerStorageUsageTable;
