import { For, Show, createMemo, type Component } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  getPlatformTableCellClassForKind,
  getPlatformTableDateTimeSortValue,
  PlatformTableShell,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  DockerResourceNameCell,
  dockerHostName,
  dockerLabelsSummary,
  dockerResourceName,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import { filterDockerResources, type DockerResourceStatusFilter } from './dockerPageModel';
import type { Resource } from '@/types/resource';

const DOCKER_CONFIG_SORT_KEYS = ['config', 'template', 'created', 'host'] as const;

type DockerConfigSortKey = (typeof DOCKER_CONFIG_SORT_KEYS)[number];

const getDockerConfigSortValue = (
  resource: Resource,
  key: DockerConfigSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'config':
      return dockerResourceName(resource);
    case 'template':
      return asTrimmedString(resource.docker?.templatingDriver) || null;
    case 'created':
      return getPlatformTableDateTimeSortValue(resource.docker?.objectCreatedAt);
    case 'host': {
      const host = dockerHostName(resource);
      return host === '—' ? null : host;
    }
    default:
      key satisfies never;
      return null;
  }
};

export const DockerConfigsTable: Component<DockerNativeTableProps> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
  });
  const sort = createPlatformTableSortState({
    storageKey: 'dockerConfigs',
    sortKeys: DOCKER_CONFIG_SORT_KEYS,
    descendingFirst: ['created'],
  });
  const sortedRows = createMemo(() => sort.sortRows(tableState.filtered(), getDockerConfigSortValue));

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
            searchPlaceholder="Search Swarm configs"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="configs"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No Swarm configs match current filters"
              description="Adjust the search or status filter to see more Docker Swarm configs."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Swarm Configs'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1040px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="config"
                  class="md:w-[28%]"
                >
                  Config
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="template"
                  class="md:w-[16%]"
                >
                  Template
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="created"
                  class="hidden md:table-cell md:w-[16%]"
                >
                  Created
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  class="hidden md:table-cell md:w-[24%]"
                >
                  Labels
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="host"
                  class="md:w-[16%]"
                >
                  Host
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(resource) => (
                    <TableRow class="text-[11px] sm:text-xs" data-docker-config-row={resource.id}>
                      <DockerResourceNameCell resource={resource} />
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        {dockerTextValue(resource.docker?.templatingDriver)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                      >
                        <span
                          class="inline-block max-w-[14rem] truncate"
                          title={dockerTextValue(resource.docker?.objectCreatedAt)}
                        >
                          {dockerTextValue(resource.docker?.objectCreatedAt)}
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                      >
                        <span
                          class="inline-block max-w-[22rem] truncate"
                          title={dockerLabelsSummary(resource.docker?.labels)}
                        >
                          {dockerLabelsSummary(resource.docker?.labels)}
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        {dockerHostName(resource)}
                      </TableCell>
                    </TableRow>
                  )}
                </For>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default DockerConfigsTable;
