import { For, Show, type Component, type JSX } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import {
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformTableFilterOption,
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

const formatActivityDate = (value: string | undefined): string => {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return '-';
  if (parsed.getUTCFullYear() < 2000) return '-';
  return parsed.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

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

const DetailField: Component<{ label: string; value: string | undefined }> = (props) => (
  <div class="min-w-0">
    <dt class="text-[10px] font-semibold uppercase tracking-wide text-muted">{props.label}</dt>
    <dd class="mt-1 truncate text-xs text-base-content" title={props.value || '-'}>
      {props.value || '-'}
    </dd>
  </div>
);

const ActivityDetail: Component<{ activity: VmwareActivityRow; onClose: () => void }> = (props) => {
  const meta = () => props.activity.resource.vmware;
  return (
    <div data-testid="vsphere-activity-detail" class="space-y-3">
      <div class="flex min-w-0 items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="text-sm font-semibold text-base-content">vSphere activity detail</div>
          <div class="mt-0.5 truncate text-xs text-muted" title={props.activity.title}>
            {formatActivityKind(props.activity.activityKind)} · {props.activity.title}
          </div>
        </div>
        <button
          type="button"
          class="inline-flex h-8 w-8 items-center justify-center rounded-md border border-border text-muted hover:bg-surface-hover hover:text-base-content"
          aria-label="Close"
          onClick={props.onClose}
        >
          <XIcon class="h-4 w-4" />
        </button>
      </div>

      <dl class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <DetailField label="Resource" value={props.activity.resourceName} />
        <DetailField label="Entity" value={formatEntityType(props.activity.entityType)} />
        <DetailField label="Managed object" value={props.activity.managedObjectId} />
        <DetailField label="Kind" value={formatActivityKind(props.activity.activityKind)} />
        <DetailField label="Activity" value={props.activity.title} />
        <DetailField label="State" value={formatActivityState(props.activity)} />
        <DetailField label="Message" value={props.activity.message} />
        <DetailField label="Description" value={props.activity.description} />
        <DetailField label="Actor" value={props.activity.actor} />
        <DetailField label="Native ID" value={props.activity.nativeId} />
        <DetailField label="vCenter" value={meta()?.connectionName || meta()?.vcenterHost} />
        <DetailField label="Datacenter" value={meta()?.datacenterName} />
        <DetailField label="Cluster" value={meta()?.clusterName || meta()?.computeResourceName} />
        <DetailField label="Occurred" value={detailDateTime(props.activity.occurredAt)} />
        <DetailField label="Observed" value={detailDateTime(props.activity.observedAt)} />
        <DetailField label="Source" value={formatIdentifierLabel(props.activity.source)} />
      </dl>
    </div>
  );
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
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[20%]`}>
                  Resource
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[8%]`}>
                  Type
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[34%]`}>
                  Activity
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[10%]`}>
                  State
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                >
                  Actor
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[10%]`}
                >
                  vCenter
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden xl:table-cell md:w-[8%]`}
                >
                  When
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
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
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content xl:table-cell`}
                          >
                            {formatActivityDate(activity.occurredAt || activity.observedAt)}
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
