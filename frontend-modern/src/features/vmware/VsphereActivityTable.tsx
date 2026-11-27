import { Show, createMemo, type Component, type JSX } from 'solid-js';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import {
  PlatformWindowedRows,
  PlatformSortableTableHead,
  PlatformTableRelativeTimeValue,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTitleCaseValue,
  getPlatformTableCellClassForKind,
  getPlatformTableDateTimeSortValue,
  type PlatformTableFilterOption,
  type PlatformTableSortValue,
  PlatformTableShell,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  getPlatformInlineStateClass,
  getPlatformStatusTextToneClass,
  isPlatformAttentionVariant,
} from '@/features/platformPage/platformStatusTone';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import { RecordOwnerLink } from '@/features/platformPage/RecordOwnerLink';
import {
  ensureStaleNoticeSentence,
  getPlatformResourceStaleNotice,
} from '@/features/platformPage/resourceStaleness';
import {
  formatPlatformAlertDetailDateTime,
  formatPlatformAlertEntityType,
  formatPlatformAlertResourceType,
} from '@/utils/alertDetailPresentation';
import {
  filterVmwareActivity,
  formatVmwareIdentifierLabel,
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

const ACTIVITY_STATE_TONE: Record<
  VmwareActivityStateBucket,
  'success' | 'warning' | 'danger' | 'muted'
> = {
  success: 'success',
  running: 'warning',
  failed: 'danger',
  unknown: 'muted',
};

const formatEntityType = (value: string): string =>
  value.trim() ? formatPlatformAlertEntityType(value) : '—';

const formatActivityState = (row: VmwareActivityRow): string =>
  row.state.trim() ? formatVmwareIdentifierLabel(row.state) : '—';

// An event's title is vCenter's own message when it sent one, so the message
// and description rows would only repeat it.
const distinctFromTitle = (activity: VmwareActivityRow, value: string): string =>
  value.trim() && value.trim() !== activity.title.trim() ? value : '';

// Recent activity reads as an age ("3h ago"); the exact time is on hover and
// in the drawer. vCenter reports unset times as dates before 2000.
const activityTimestamp = (activity: VmwareActivityRow): string | undefined => {
  const value = activity.occurredAt || activity.observedAt;
  if (!value) return undefined;
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) || parsed.getUTCFullYear() < 2000 ? undefined : value;
};

const buildActivityDetailSections = (activity: VmwareActivityRow): DetailSection[] => {
  const meta = activity.resource.vmware;
  return compactDetailSections([
    {
      label: 'Activity',
      rows: compactDetailRows([
        makeDetailRow('Kind', formatPlatformTableTitleCaseValue(activity.activityKind)),
        makeDetailRow('State', formatActivityState(activity), {
          tone: ACTIVITY_STATE_TONE[activity.stateBucket],
        }),
        makeDetailRow('Activity', activity.title),
        makeDetailRow('Event type', activity.eventType),
        makeDetailRow('Message', distinctFromTitle(activity, activity.message)),
        makeDetailRow('Description', distinctFromTitle(activity, activity.description)),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        makeDetailRow('Resource', activity.resourceName),
        makeDetailRow('Type', formatPlatformAlertResourceType(activity.resourceType, 'vmware')),
        makeDetailRow('Entity', formatEntityType(activity.entityType)),
        makeDetailRow('Managed object', activity.managedObjectId),
        makeDetailRow('vCenter', meta?.connectionName || meta?.vcenterHost),
        makeDetailRow('Datacenter', meta?.datacenterName),
        makeDetailRow('Cluster', meta?.clusterName || meta?.computeResourceName),
        makeDetailRow('Resource ID', activity.resourceId),
      ]),
    },
    {
      label: 'Source',
      rows: compactDetailRows([
        makeDetailRow('Native ID', activity.nativeId),
        makeDetailRow('Actor', activity.actor),
        makeDetailRow('Source', formatVmwareIdentifierLabel(activity.source)),
        makeDetailRow('Occurred', formatPlatformAlertDetailDateTime(activity.occurredAt)),
        makeDetailRow('Observed', formatPlatformAlertDetailDateTime(activity.observedAt)),
      ]),
    },
  ]);
};

const ActivityDetail: Component<{ activity: VmwareActivityRow; onClose: () => void }> = (props) => {
  const staleNotice = () => getPlatformResourceStaleNotice('vmware', props.activity.resource);
  return (
    <InlineDetailPanel
      testId="vsphere-activity-detail"
      detailFor={props.activity.id}
      title={props.activity.resourceName || props.activity.title}
      status={{
        variant: ACTIVITY_STATE_TONE[props.activity.stateBucket],
        label: formatPlatformTableTitleCaseValue(props.activity.stateBucket),
      }}
      summary={`${formatPlatformTableTitleCaseValue(props.activity.activityKind)} · ${props.activity.title}`}
      actions={
        <RecordOwnerLink
          subject={{
            id: props.activity.resourceId,
            name: props.activity.resourceName,
            type: props.activity.resourceType,
            resource: props.activity.resource,
          }}
        />
      }
      sections={buildActivityDetailSections(props.activity)}
      detailAttributes={{ 'data-vsphere-activity-detail-for': props.activity.id }}
      onClose={props.onClose}
    >
      <Show when={staleNotice()}>
        {(stale) => (
          <div data-testid="vsphere-activity-stale-readings-notice">
            <InlineNotice tone="warning" role="status">
              <span class="font-medium">{stale().label}.</span>{' '}
              {ensureStaleNoticeSentence(stale().detail)} Activity records reflect the last report
              received and newer vCenter tasks or events may be missing.
            </InlineNotice>
          </div>
        )}
      </Show>
    </InlineDetailPanel>
  );
};

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
      return formatPlatformTableTitleCaseValue(activity.activityKind);
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
  // One vCenter connection: its name would repeat on every row, so the
  // column hides (the row drawer still names it).
  singleVcenter?: boolean;
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
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              VSPHERE_ACTIVITY_STATUS_OPTIONS,
              tableState.countForStatus,
            )}
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
                  class="platform-table-mobile-w-30 md:w-[20%]"
                >
                  Resource
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="type"
                  class="platform-table-mobile-w-15 md:w-[8%]"
                >
                  Type
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="activity"
                  class="platform-table-mobile-w-25 md:w-[34%]"
                >
                  Activity
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="badge"
                  sort={sort}
                  sortKey="state"
                  class="platform-table-phone-hidden md:w-[10%]"
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
                <Show when={!props.singleVcenter}>
                  <PlatformSortableTableHead
                    kind="text"
                    sort={sort}
                    sortKey="vcenter"
                    class="hidden lg:table-cell md:w-[10%]"
                  >
                    vCenter
                  </PlatformSortableTableHead>
                </Show>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="when"
                  class="platform-table-mobile-w-15 md:w-[8%]"
                >
                  When
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
                  {(activity) => {
                    const meta = () => activity.resource.vmware;
                    const tone = () => ACTIVITY_STATE_TONE[activity.stateBucket];
                    const staleNotice = () =>
                      getPlatformResourceStaleNotice('vmware', activity.resource);
                    const detailRowId = () => drawer.detailRowId(activity);
                    const isExpanded = () => drawer.isExpanded(activity);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-vsphere-activity-row={activity.id}
                          onClick={() => drawer.toggle(activity)}
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
                                variant={tone()}
                                title={formatActivityState(activity)}
                              />
                              <div class="min-w-0 flex-1">
                                <div class="flex min-w-0 items-center gap-1.5">
                                  <div
                                    class="min-w-0 truncate font-medium text-base-content"
                                    title={[
                                      activity.resourceName,
                                      formatPlatformAlertResourceType(
                                        activity.resourceType,
                                        'vmware',
                                      ),
                                      activity.resource.parentName
                                        ? `on ${activity.resource.parentName}`
                                        : '',
                                    ]
                                      .filter(Boolean)
                                      .join(' · ')}
                                  >
                                    {activity.resourceName}
                                  </div>
                                  {/* A phone hides State; a state that needs attention joins the
                                      name on its line. */}
                                  <span
                                    class={`platform-table-phone-only-inline text-[11px] ${getPlatformInlineStateClass(isPlatformAttentionVariant(tone()))} ${getPlatformStatusTextToneClass(tone())}`}
                                    data-vsphere-activity-state-phone={tone()}
                                  >
                                    <span class="min-w-0 truncate">
                                      {formatActivityState(activity)}
                                    </span>
                                  </span>
                                </div>
                                <Show when={staleNotice()}>
                                  {(stale) => (
                                    <span
                                      class="block truncate text-[10px] font-medium text-amber-800 dark:text-amber-300"
                                      title={stale().detail}
                                      data-vsphere-activity-stale="true"
                                    >
                                      {stale().label}
                                    </span>
                                  )}
                                </Show>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <span class="text-[11px] font-semibold text-base-content">
                              {formatPlatformTableTitleCaseValue(activity.activityKind)}
                            </span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span
                              class="block truncate text-base-content"
                              title={[
                                activity.title,
                                distinctFromTitle(activity, activity.message) ||
                                  distinctFromTitle(activity, activity.description) ||
                                  activity.nativeId,
                              ]
                                .filter(Boolean)
                                .join(' · ')}
                            >
                              {activity.title}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('badge')} platform-table-phone-hidden`}
                          >
                            <span
                              class={`text-[11px] font-semibold ${getPlatformStatusTextToneClass(tone()) || 'text-base-content'}`}
                              data-vsphere-activity-state={tone()}
                            >
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
                          <Show when={!props.singleVcenter}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} hidden lg:table-cell`}
                            >
                              <span
                                class="block truncate text-base-content"
                                title={[
                                  meta()?.connectionName || meta()?.vcenterHost,
                                  meta()?.datacenterName || meta()?.clusterName,
                                ]
                                  .filter(Boolean)
                                  .join(' · ')}
                              >
                                {meta()?.connectionName || meta()?.vcenterHost || '-'}
                              </span>
                            </TableCell>
                          </Show>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <span
                              title={formatPlatformAlertDetailDateTime(activityTimestamp(activity))}
                            >
                              <PlatformTableRelativeTimeValue
                                value={activityTimestamp(activity)}
                                emptyText="—"
                              />
                            </span>
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={props.singleVcenter ? 6 : 7}
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
                </PlatformWindowedRows>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default VsphereActivityTable;
