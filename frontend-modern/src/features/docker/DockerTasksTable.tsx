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
  dockerNumberValue,
  dockerResourceName,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import {
  compareDockerTasks,
  filterDockerResources,
  mapDockerTaskStatus,
  type DockerResourceStatusFilter,
} from './dockerPageModel';
import type { Resource } from '@/types/resource';

const DOCKER_TASK_SORT_KEYS = [
  'task',
  'service',
  'slot',
  'desired',
  'current',
  'node',
  'started',
] as const;

type DockerTaskSortKey = (typeof DOCKER_TASK_SORT_KEYS)[number];

const getDockerTaskSortValue = (
  resource: Resource,
  key: DockerTaskSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'task':
      return dockerResourceName(resource);
    case 'service':
      return asTrimmedString(resource.docker?.serviceName) || null;
    case 'slot':
      return typeof resource.docker?.slot === 'number' ? resource.docker.slot : null;
    case 'desired':
      return asTrimmedString(resource.docker?.desiredState) || null;
    case 'current':
      return asTrimmedString(resource.docker?.currentState) || null;
    case 'node':
      return asTrimmedString(resource.docker?.nodeName || resource.docker?.nodeId) || null;
    case 'started':
      return getPlatformTableDateTimeSortValue(resource.docker?.startedAt);
    default:
      key satisfies never;
      return null;
  }
};

export const DockerTasksTable: Component<DockerNativeTableProps> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
  });
  const sort = createPlatformTableSortState({
    storageKey: 'dockerTasks',
    sortKeys: DOCKER_TASK_SORT_KEYS,
    descendingFirst: ['started'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows([...tableState.filtered()].sort(compareDockerTasks), getDockerTaskSortValue),
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
            searchPlaceholder="Search Swarm tasks"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="tasks"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No Swarm tasks match current filters"
              description="Adjust the search or status filter to see more Docker Swarm tasks."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Swarm Tasks'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1160px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="task"
                  class="md:w-[18%]"
                >
                  Task
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="service"
                  class="hidden md:table-cell md:w-[18%]"
                >
                  Service
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="slot"
                  class="hidden md:table-cell md:w-[8%]"
                >
                  Slot
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="desired"
                  class="md:w-[12%]"
                >
                  Desired
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="current"
                  class="md:w-[16%]"
                >
                  Current
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="node"
                  class="hidden md:table-cell md:w-[16%]"
                >
                  Node
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="started"
                  class="hidden md:table-cell md:w-[12%]"
                >
                  Started
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(resource) => (
                    <TableRow class="text-[11px] sm:text-xs" data-docker-task-row={resource.id}>
                      <DockerResourceNameCell
                        resource={resource}
                        indicator={mapDockerTaskStatus(resource)}
                      />
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                      >
                        {dockerTextValue(resource.docker?.serviceName)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                      >
                        {dockerNumberValue(resource.docker?.slot)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        {dockerTextValue(resource.docker?.desiredState)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <span
                          class="inline-block max-w-[14rem] truncate"
                          title={dockerTextValue(
                            resource.docker?.error ||
                              resource.docker?.message ||
                              resource.docker?.currentState,
                          )}
                        >
                          {dockerTextValue(resource.docker?.currentState)}
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                      >
                        {dockerTextValue(resource.docker?.nodeName || resource.docker?.nodeId)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                      >
                        <span
                          class="inline-block max-w-[12rem] truncate"
                          title={dockerTextValue(resource.docker?.startedAt)}
                        >
                          {dockerTextValue(resource.docker?.startedAt)}
                        </span>
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

export default DockerTasksTable;
