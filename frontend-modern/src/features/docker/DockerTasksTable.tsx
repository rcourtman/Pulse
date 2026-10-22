import { Show, createMemo, type Component } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { asTrimmedString } from '@/utils/stringUtils';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';
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
  PlatformTableRelativeTimeValue,
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
  compactDetailRows,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import {
  DockerNativeDetailPanel,
  DockerResourceNameCell,
  dockerDetailTimestampRow,
  dockerDetailToneForVariant,
  dockerLabelsSection,
  dockerReportedBySection,
  dockerNumberValue,
  dockerResourceName,
  dockerStateTextToneClass,
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

// The drawer leads with why a task failed or what Docker last said about it,
// for a phone that cannot hover the Current cell; otherwise where it runs.
const taskReason = (resource: Resource): string =>
  asTrimmedString(resource.docker?.error) || asTrimmedString(resource.docker?.message) || '';

const taskSummary = (resource: Resource): string => {
  const reason = taskReason(resource);
  if (reason) return reason;
  const node = asTrimmedString(resource.docker?.nodeName || resource.docker?.nodeId);
  const service = asTrimmedString(resource.docker?.serviceName);
  return [service ? `Task of ${service}` : '', node ? `on ${node}` : ''].filter(Boolean).join(' ');
};

const buildTaskDetailSections = (resource: Resource, now: number): Array<DetailSection | null> => {
  const status = mapDockerTaskStatus(resource);
  return [
    {
      label: 'Task',
      rows: compactDetailRows([
        makeDetailRow('Service', asTrimmedString(resource.docker?.serviceName), { wrap: true }),
        typeof resource.docker?.slot === 'number'
          ? makeDetailRow('Slot', String(resource.docker.slot))
          : null,
        makeDetailRow(
          'Node',
          asTrimmedString(resource.docker?.nodeName || resource.docker?.nodeId),
          { wrap: true },
        ),
        makeDetailRow('Image', asTrimmedString(resource.docker?.image), { wrap: true }),
        makeDetailRow('Task ID', asTrimmedString(resource.docker?.taskId), { wrap: true }),
      ]),
    },
    {
      label: 'State',
      rows: compactDetailRows([
        makeDetailRow(
          'Desired',
          titleCaseDelimitedLabel(resource.docker?.desiredState, { fallback: '' }),
        ),
        makeDetailRow('Current', status.label, {
          title: asTrimmedString(resource.docker?.currentState) || undefined,
          tone: dockerDetailToneForVariant(status.variant),
        }),
        // The summary already reads the error, or the message when there is
        // none; the other one, if Docker sent both, stays here.
        asTrimmedString(resource.docker?.error)
          ? makeDetailRow('Message', asTrimmedString(resource.docker?.message), { wrap: true })
          : null,
        dockerDetailTimestampRow('Started', resource.docker?.startedAt, now),
        dockerDetailTimestampRow('Finished', resource.docker?.completedAt, now),
      ]),
    },
    dockerReportedBySection(resource),
    dockerLabelsSection('Labels', resource.docker?.labels),
  ];
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
                            {/* Read as words beside Current; Docker's token on hover. */}
                            <span title={dockerTextValue(resource.docker?.desiredState)}>
                              {titleCaseDelimitedLabel(resource.docker?.desiredState, {
                                fallback: '—',
                              })}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {/* The dot's own words and colour; Docker's raw state, error
                                or message stays on hover. */}
                            <span
                              class={`block truncate ${dockerStateTextToneClass(
                                mapDockerTaskStatus(resource).variant,
                              )}`}
                              title={dockerTextValue(
                                resource.docker?.error ||
                                  resource.docker?.message ||
                                  resource.docker?.currentState,
                              )}
                              data-docker-task-state={mapDockerTaskStatus(resource).variant}
                            >
                              {mapDockerTaskStatus(resource).label}
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
                            {/* Swarm reports a raw RFC3339 stamp with microseconds; an age
                                reads at a glance and the stamp stays in the title. */}
                            <span title={dockerTextValue(resource.docker?.startedAt)}>
                              <PlatformTableRelativeTimeValue value={resource.docker?.startedAt} />
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
                              testId="docker-task-detail"
                              resource={resource}
                              status={mapDockerTaskStatus(resource)}
                              summary={taskSummary(resource)}
                              sections={(now) => buildTaskDetailSections(resource, now)}
                              onClose={() => drawer.close(resource)}
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
