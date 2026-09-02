import { Show, createMemo, type Component } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
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
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  DockerNativeDetailPanel,
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
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-task-detail' });

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
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              PLATFORM_HEALTH_FILTER_OPTIONS,
              tableState.countForStatus,
            )}
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
                  class="platform-table-mobile-w-30 md:w-[18%]"
                >
                  Task
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="service"
                  class="platform-table-mobile-w-15 md:w-[18%]"
                >
                  Service
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="slot"
                  class="platform-table-narrow-hidden hidden md:table-cell md:w-[8%]"
                >
                  Slot
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="desired"
                  class="platform-table-phone-hidden md:w-[12%]"
                >
                  Desired
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="current"
                  class="platform-table-mobile-w-15 md:w-[16%]"
                >
                  Current
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="node"
                  class="platform-table-mobile-w-15 md:w-[16%]"
                >
                  Node
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="started"
                  class="platform-table-narrow-hidden hidden md:table-cell md:w-[12%]"
                >
                  Started
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
                          data-docker-task-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                        >
                          <DockerResourceNameCell
                            resource={resource}
                            indicator={mapDockerTaskStatus(resource)}
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
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {dockerTextValue(resource.docker?.serviceName)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} platform-table-narrow-hidden hidden text-base-content md:table-cell`}
                          >
                            {dockerNumberValue(resource.docker?.slot)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden text-base-content`}
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
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {dockerTextValue(resource.docker?.nodeName || resource.docker?.nodeId)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} platform-table-narrow-hidden hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[12rem] truncate"
                              title={dockerTextValue(resource.docker?.startedAt)}
                            >
                              {dockerTextValue(resource.docker?.startedAt)}
                            </span>
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={7}
                            data-inline-docker-task-detail-for={resource.id}
                          >
                            <DockerNativeDetailPanel
                              title={dockerResourceName(resource)}
                              fields={[
                                ['Service', dockerTextValue(resource.docker?.serviceName)],
                                ['Slot', String(resource.docker?.slot ?? '—')],
                                ['Desired', dockerTextValue(resource.docker?.desiredState)],
                                ['Current', dockerTextValue(resource.docker?.currentState)],
                                [
                                  'Node',
                                  dockerTextValue(
                                    resource.docker?.nodeName || resource.docker?.nodeId,
                                  ),
                                ],
                                ['Started', dockerTextValue(resource.docker?.startedAt)],
                              ]}
                            />
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

export default DockerTasksTable;
