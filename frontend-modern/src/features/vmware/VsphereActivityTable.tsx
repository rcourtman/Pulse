import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import {
  PlatformSortableTableHead,
  PlatformTableDateTimeValue,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  getPlatformTableCellClassForKind,
  getPlatformTableDateTimeSortValue,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { ResourceType } from '@/types/resource';
import type { StatusIndicatorVariant } from '@/utils/status';
import {
  filterVmwareActivity,
  type VmwareActivityKind,
  type VmwareActivityRow,
  type VmwareActivityStateBucket,
  type VmwareActivityStatusFilter,
} from './vmwarePageModel';

const VSPHERE_ACTIVITY_STATUS_OPTIONS: PlatformTableFilterOption<VmwareActivityStatusFilter>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'tasks',
    label: 'Tasks',
    tone: 'success',
    leading: filterChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'events',
    label: 'Events',
    tone: 'info',
    leading: filterChipStatusDot('bg-blue-500'),
  },
  {
    value: 'failed',
    label: 'Failed',
    tone: 'danger',
    leading: filterChipStatusDot('bg-red-500'),
  },
];

const formatResourceType = (type: ResourceType): string => {
  switch (type) {
    case 'agent':
      return 'Host';
    case 'vm':
      return 'VM';
    case 'storage':
      return 'Datastore';
    default:
      return type;
  }
};

const formatEntityType = (value: string): string => {
  const normalized = value.trim().toLowerCase();
  if (normalized === 'host') return 'Host';
  if (normalized === 'vm') return 'VM';
  if (normalized === 'datastore') return 'Datastore';
  return normalized ? normalized.charAt(0).toUpperCase() + normalized.slice(1) : '-';
};

const formatActivityKind = (kind: VmwareActivityKind): string => {
  switch (kind) {
    case 'task':
      return 'Task';
    case 'event':
      return 'Event';
    case 'activity':
      return 'Activity';
  }
};

const activityStateVariant = (bucket: VmwareActivityStateBucket): StatusIndicatorVariant => {
  switch (bucket) {
    case 'success':
      return 'success';
    case 'running':
      return 'warning';
    case 'failed':
      return 'danger';
    case 'unknown':
      return 'muted';
  }
};

const formatIdentifierLabel = (value: string): string =>
  value
    .trim()
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

const formatActivityState = (row: VmwareActivityRow): string =>
  row.state.trim() ? formatIdentifierLabel(row.state) : '-';

const detailDateTime = (value?: string): string => {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  if (parsed.getUTCFullYear() < 2000) return '-';
  return parsed.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

type ActivityDetailTone = DetailValueTone;
type ActivityDetailSection = DetailSection;

const detailRow = makeDetailRow;

const activityTone = (bucket: VmwareActivityStateBucket): ActivityDetailTone => {
  if (bucket === 'success') return 'success';
  if (bucket === 'running') return 'warning';
  if (bucket === 'failed') return 'danger';
  return 'muted';
};

const buildActivityDetailSections = (activity: VmwareActivityRow): ActivityDetailSection[] => {
  const meta = activity.resource.vmware;
  return compactDetailSections([
    {
      label: 'Activity',
      rows: compactDetailRows([
        detailRow('Kind', formatActivityKind(activity.activityKind)),
        detailRow('State', formatActivityState(activity), {
          tone: activityTone(activity.stateBucket),
        }),
        detailRow('Activity', activity.title),
        detailRow('Message', activity.message),
        detailRow('Description', activity.description),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        detailRow('Resource', activity.resourceName),
        detailRow('Type', formatResourceType(activity.resourceType)),
        detailRow('Entity', formatEntityType(activity.entityType)),
        detailRow('Managed object', activity.managedObjectId),
        detailRow('vCenter', meta?.connectionName || meta?.vcenterHost),
        detailRow('Datacenter', meta?.datacenterName),
        detailRow('Cluster', meta?.clusterName || meta?.computeResourceName),
        detailRow('Resource ID', activity.resourceId),
      ]),
    },
    {
      label: 'Source',
      rows: compactDetailRows([
        detailRow('Native ID', activity.nativeId),
        detailRow('Actor', activity.actor),
        detailRow('Source', formatIdentifierLabel(activity.source)),
        detailRow('Occurred', detailDateTime(activity.occurredAt)),
        detailRow('Observed', detailDateTime(activity.observedAt)),
      ]),
    },
  ]);
};

const ActivityDetail: Component<{ activity: VmwareActivityRow; onClose: () => void }> = (props) => (
  <InlineDetailPanel
    testId="vsphere-activity-detail"
    detailFor={props.activity.id}
    title="vSphere activity detail"
    summary={`${formatActivityKind(props.activity.activityKind)} · ${props.activity.title}`}
    sections={buildActivityDetailSections(props.activity)}
    detailAttributes={{ 'data-vsphere-activity-detail-for': props.activity.id }}
    onClose={props.onClose}
  />
);

// Columns a user can sort by: every activity column carries a scalar. When
// orders chronologically, newest first on the first click.
const VSPHERE_ACTIVITY_SORT_KEYS = [
  'resource',
  'type',
  'activity',
  'state',
  'actor',
  'vcenter',
  'when',
] as const;

type VsphereActivitySortKey = (typeof VSPHERE_ACTIVITY_SORT_KEYS)[number];

const getVsphereActivitySortValue = (
  activity: VmwareActivityRow,
  key: VsphereActivitySortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'resource':
      return activity.resourceName || null;
    case 'type':
      return formatActivityKind(activity.activityKind);
    case 'activity':
      return activity.title || null;
    case 'state': {
      const state = formatActivityState(activity);
      return state === '-' ? null : state;
    }
    case 'actor':
      return activity.actor || null;
    case 'vcenter': {
      const meta = activity.resource.vmware;
      return meta?.connectionName || meta?.vcenterHost || null;
    }
    case 'when':
      return getPlatformTableDateTimeSortValue(activity.occurredAt || activity.observedAt);
    default:
      key satisfies never;
      return null;
  }
};

export const VsphereActivityTable: Component<{
  activity: VmwareActivityRow[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.activity,
    initialStatus: 'all' as VmwareActivityStatusFilter,
    filter: filterVmwareActivity,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'vsphere-activity-drawer' });
  const sort = createPlatformTableSortState({
    storageKey: 'vsphereActivity',
    sortKeys: VSPHERE_ACTIVITY_SORT_KEYS,
    descendingFirst: ['when'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getVsphereActivitySortValue),
  );

  return (
    <Show
      when={props.activity.length > 0}
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
            searchPlaceholder="Search vSphere activity"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={VSPHERE_ACTIVITY_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="events"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No activity matches current filters"
              description="Adjust the search or activity filter to see more vCenter tasks and events."
            />
          }
        >
          <PlatformTableShell
            title="Recent Activity"
            tableClass="min-w-full table-fixed text-xs md:min-w-[1120px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="resource"
                  class="md:w-[20%]"
                >
                  Resource
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="type"
                  class="md:w-[8%]"
                >
                  Type
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="activity"
                  class="md:w-[34%]"
                >
                  Activity
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="state"
                  class="md:w-[10%]"
                >
                  State
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="actor"
                  class="hidden md:table-cell md:w-[10%]"
                >
                  Actor
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="vcenter"
                  class="hidden lg:table-cell md:w-[10%]"
                >
                  vCenter
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="when"
                  class="hidden xl:table-cell md:w-[8%]"
                >
                  When
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(activity) => {
                    const meta = () => activity.resource.vmware;
                    const detailRowId = () => drawer.detailRowId(activity);
                    const isExpanded = () => drawer.isExpanded(activity);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-vsphere-activity-row={activity.id}
                          onClick={() => drawer.toggle(activity)}
                          onKeyDown={drawer.handleActivationKey(activity)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={activity.resourceName}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(activity)}
                              />
                              <StatusDot
                                size="sm"
                                variant={activityStateVariant(activity.stateBucket)}
                                title={formatActivityState(activity)}
                              />
                              <div class="min-w-0">
                                <div
                                  class="truncate font-medium text-base-content"
                                  title={activity.resourceName}
                                >
                                  {activity.resourceName}
                                </div>
                                <div class="truncate text-[10px] text-muted">
                                  {formatResourceType(activity.resourceType)}
                                  <Show when={activity.resource.parentName}>
                                    on {activity.resource.parentName}
                                  </Show>
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <span class="text-[11px] font-semibold text-base-content">
                              {formatActivityKind(activity.activityKind)}
                            </span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span class="block truncate text-base-content" title={activity.title}>
                              {activity.title}
                            </span>
                            <span class="block truncate text-[10px] text-muted">
                              {activity.message || activity.description || activity.nativeId}
                            </span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <span class="text-[11px] font-semibold text-base-content">
                              {formatActivityState(activity)}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="block truncate" title={activity.actor}>
                              {activity.actor || '-'}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden lg:table-cell`}
                          >
                            <span
                              class="block truncate text-base-content"
                              title={meta()?.connectionName || meta()?.vcenterHost}
                            >
                              {meta()?.connectionName || meta()?.vcenterHost || '-'}
                            </span>
                            <span class="block truncate text-[10px] text-muted">
                              {meta()?.datacenterName || meta()?.clusterName || '-'}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content xl:table-cell`}
                          >
                            <PlatformTableDateTimeValue
                              value={activity.occurredAt || activity.observedAt}
                              emptyText="-"
                              minYear={2000}
                            />
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={7}
                            data-inline-detail-for={activity.id}
                            data-vsphere-activity-detail-row={activity.id}
                          >
                            <ActivityDetail
                              activity={activity}
                              onClose={() => drawer.close(activity)}
                            />
                          </InlineDetailTableRow>
                        </Show>
                      </>
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

export default VsphereActivityTable;
