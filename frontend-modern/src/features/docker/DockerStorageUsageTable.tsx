import { Show, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformResponsiveTableLabel,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  formatPlatformTableBytesValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableShell,
  withPlatformStatusCounts,
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
    <span
      class="block truncate tabular-nums text-base-content"
      title={`${formatPlatformTableBytesValue(totalSize, '0 B')} total · ${active}/${count} active · ${formatPlatformTableBytesValue(reclaimable, '0 B')} reclaimable`}
    >
      {formatPlatformTableBytesValue(totalSize, '0 B')} · {active}/{count}
    </span>
  );
};

const bucketDetail = (bucket?: DockerStorageUsageMeta): string => {
  if (!hasDockerStorageUsageBucket(bucket)) return '—';
  return `${formatPlatformTableBytesValue(bucket?.totalSizeBytes ?? 0, '0 B')} total · ${bucket?.activeCount ?? 0}/${bucket?.totalCount ?? 0} active · ${formatPlatformTableBytesValue(bucket?.reclaimableBytes ?? 0, '0 B')} reclaimable`;
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
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-storage-detail' });
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
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              PLATFORM_HEALTH_FILTER_OPTIONS,
              tableState.countForStatus,
            )}
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
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 md:w-[22%]`}
                >
                  Host
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[19%]`}
                >
                  Images
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[19%]`}
                >
                  <PlatformResponsiveTableLabel compact="Ctrs" full="Containers" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[20%]`}
                >
                  <PlatformResponsiveTableLabel compact="Vols" full="Volumes" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[20%]`}
                >
                  <PlatformResponsiveTableLabel compact="Cache" full="Build Cache" />
                </TableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={tableState.filtered} estimatedRowHeight={32}>
                  {(host) => {
                    const name = () => asTrimmedString(host.name) || host.id;
                    const indicator = () => getSimpleStatusIndicator(host.status);
                    const detailRowId = () => drawer.detailRowId(host);
                    const isExpanded = () => drawer.isExpanded(host);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-docker-storage-row={host.id}
                          onClick={() => drawer.toggle(host)}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(host)}
                              />
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
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {bucketValue(host.docker?.volumesUsage)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {bucketValue(host.docker?.buildCacheUsage)}
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={5}
                            data-inline-docker-storage-detail-for={host.id}
                          >
                            <div class="grid gap-x-4 gap-y-2 text-[11px] sm:grid-cols-2">
                              <div class="sm:col-span-2 text-xs font-semibold text-base-content">
                                {name()}
                              </div>
                              <div>
                                <span class="text-muted">Images</span>
                                <div class="break-words font-mono text-base-content">
                                  {bucketDetail(host.docker?.imagesUsage)}
                                </div>
                              </div>
                              <div>
                                <span class="text-muted">Containers</span>
                                <div class="break-words font-mono text-base-content">
                                  {bucketDetail(host.docker?.containersUsage)}
                                </div>
                              </div>
                              <div>
                                <span class="text-muted">Volumes</span>
                                <div class="break-words font-mono text-base-content">
                                  {bucketDetail(host.docker?.volumesUsage)}
                                </div>
                              </div>
                              <div>
                                <span class="text-muted">Build cache</span>
                                <div class="break-words font-mono text-base-content">
                                  {bucketDetail(host.docker?.buildCacheUsage)}
                                </div>
                              </div>
                            </div>
                          </InlineDetailTableRow>
                        </Show>
                      </>
                    );
                  }}
                </PlatformWindowedRows>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default DockerStorageUsageTable;
