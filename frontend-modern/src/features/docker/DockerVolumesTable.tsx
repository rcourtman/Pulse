import { Show, createMemo, type Component } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableRelativeTimeValue,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableDateTimeValue,
  getPlatformTableCellClassForKind,
  getPlatformTableDateTimeSortValue,
  PlatformTableShell,
  type PlatformTableSortValue,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  DockerResourceNameCell,
  dockerByteValue,
  dockerNumberValue,
  dockerResourceName,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import { filterDockerResources, type DockerResourceStatusFilter } from './dockerPageModel';
import type { Resource } from '@/types/resource';

const DOCKER_VOLUME_SORT_KEYS = [
  'volume',
  'driver',
  'scope',
  'size',
  'refs',
  'created',
  'mountpoint',
] as const;

type DockerVolumeSortKey = (typeof DOCKER_VOLUME_SORT_KEYS)[number];

const getDockerVolumeSortValue = (
  resource: Resource,
  key: DockerVolumeSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'volume':
      return dockerResourceName(resource);
    case 'driver':
      return asTrimmedString(resource.docker?.driver) || null;
    case 'scope':
      return asTrimmedString(resource.docker?.scope) || null;
    case 'size':
      return typeof resource.docker?.sizeBytes === 'number' && resource.docker.sizeBytes > 0
        ? resource.docker.sizeBytes
        : null;
    case 'refs':
      return typeof resource.docker?.refCount === 'number' ? resource.docker.refCount : null;
    case 'created':
      return getPlatformTableDateTimeSortValue(resource.docker?.createdAt);
    case 'mountpoint':
      return asTrimmedString(resource.docker?.mountpoint) || null;
    default:
      key satisfies never;
      return null;
  }
};

export const DockerVolumesTable: Component<DockerNativeTableProps> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
    externalSearch: props.externalSearch,
    externalStatus: props.externalStatus,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-volume-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);
  const sort = createPlatformTableSortState({
    storageKey: 'dockerVolumes',
    sortKeys: DOCKER_VOLUME_SORT_KEYS,
    descendingFirst: ['size', 'refs', 'created'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getDockerVolumeSortValue),
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
            searchPlaceholder="Search volumes"
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              PLATFORM_HEALTH_FILTER_OPTIONS,
              tableState.countForStatus,
            )}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="volumes"
            hasActiveFilters={tableState.hasActiveFilters()}
            onResetFilters={tableState.resetFilters}
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No volumes match current filters"
              description="Adjust the search or status filter to see more Docker volumes."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Volumes'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1120px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="volume"
                  class="platform-table-mobile-w-30 md:w-[22%]"
                >
                  Volume
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="driver"
                  class="platform-table-phone-hidden md:w-[12%]"
                >
                  Driver
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="scope"
                  class="platform-table-phone-hidden md:w-[10%]"
                >
                  Scope
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="size"
                  class="platform-table-mobile-w-15 md:w-[10%]"
                >
                  Size
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="refs"
                  class="platform-table-mobile-w-15 md:w-[8%]"
                >
                  Refs
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="created"
                  class="platform-table-mobile-w-15 md:w-[14%]"
                >
                  Created
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="mountpoint"
                  class="hidden md:table-cell md:w-[24%]"
                >
                  Mountpoint
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
                  {(resource) => {
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-docker-volume-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                        >
                          <DockerResourceNameCell
                            resource={resource}
                            detailToggle={
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={dockerResourceName(resource)}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                            }
                          />
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden text-base-content`}
                          >
                            {dockerTextValue(resource.docker?.driver)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden text-base-content`}
                          >
                            {dockerTextValue(resource.docker?.scope)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {dockerByteValue(resource.docker?.sizeBytes)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {dockerNumberValue(resource.docker?.refCount)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span
                              class="inline-block max-w-[12rem] truncate"
                              title={formatPlatformTableDateTimeValue(resource.docker?.createdAt)}
                            >
                              <PlatformTableRelativeTimeValue
                                value={resource.docker?.createdAt}
                                compact={false}
                              />
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[22rem] truncate"
                              title={dockerTextValue(resource.docker?.mountpoint)}
                            >
                              {dockerTextValue(resource.docker?.mountpoint)}
                            </span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={7}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource)}
                        />
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

export default DockerVolumesTable;
