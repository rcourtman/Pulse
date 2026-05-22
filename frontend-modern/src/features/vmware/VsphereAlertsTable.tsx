import { For, Show, type Component, type JSX } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  platformChipStatusDot,
  type PlatformTableFilterOption,
} from '@/features/platformPage/sharedPlatformPage';
import {
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { ResourceType } from '@/types/resource';
import type { StatusIndicatorVariant } from '@/utils/status';
import { getAlertFilteredEmptyState } from '@/utils/alertOverviewPresentation';
import {
  filterVmwareIncidents,
  type VmwareIncidentRow,
  type VmwareIncidentSeverityFilter,
} from './vmwarePageModel';

const VSPHERE_INCIDENT_STATUS_OPTIONS: PlatformTableFilterOption<VmwareIncidentSeverityFilter>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'critical',
    label: 'Critical',
    tone: 'danger',
    leading: platformChipStatusDot('bg-red-500'),
  },
  {
    value: 'warning',
    label: 'Warning',
    tone: 'warning',
    leading: platformChipStatusDot('bg-amber-500'),
  },
  {
    value: 'info',
    label: 'Info',
    tone: 'success',
    leading: platformChipStatusDot('bg-emerald-500'),
  },
];

const severityVariant = (severity: VmwareIncidentRow['severityBucket']): StatusIndicatorVariant => {
  switch (severity) {
    case 'critical':
      return 'danger';
    case 'warning':
      return 'warning';
    case 'info':
      return 'muted';
  }
};

const severityTextClass = (severity: VmwareIncidentRow['severityBucket']): string => {
  switch (severity) {
    case 'critical':
      return 'text-red-700 dark:text-red-300';
    case 'warning':
      return 'text-amber-700 dark:text-amber-300';
    case 'info':
      return 'text-muted';
  }
};

const severityLabel = (severity: string): string => {
  const normalized = severity.trim();
  if (!normalized) return 'Info';
  return normalized.charAt(0).toUpperCase() + normalized.slice(1).toLowerCase();
};

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

const formatCode = (code: string): string => {
  const normalized = code.trim().replace(/^vmware_/, '');
  if (!normalized) return '-';
  return normalized
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');
};

const formatStartedAt = (value: string | undefined): string => {
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

const AlertDetail: Component<{ incident: VmwareIncidentRow; onClose: () => void }> = (props) => {
  const meta = () => props.incident.resource.vmware;
  return (
    <div data-testid="vsphere-alert-detail" class="space-y-3">
      <div class="flex min-w-0 items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="text-sm font-semibold text-base-content">vSphere health detail</div>
          <div class="mt-0.5 truncate text-xs text-muted" title={props.incident.summary}>
            {severityLabel(props.incident.severity)} · {formatCode(props.incident.code)}
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
        <DetailField label="Summary" value={props.incident.summary} />
        <DetailField label="Signal" value={props.incident.label} />
        <DetailField label="Severity" value={severityLabel(props.incident.severity)} />
        <DetailField label="Entity" value={formatEntityType(props.incident.entityType)} />
        <DetailField label="Managed object" value={props.incident.managedObjectId} />
        <DetailField label="Resource" value={props.incident.resourceName} />
        <DetailField label="vCenter" value={meta()?.connectionName || meta()?.vcenterHost} />
        <DetailField label="Datacenter" value={meta()?.datacenterName} />
        <DetailField label="Cluster" value={meta()?.clusterName || meta()?.computeResourceName} />
        <DetailField label="Started" value={detailDateTime(props.incident.startedAt)} />
        <DetailField label="Source" value={props.incident.source} />
        <DetailField label="Action" value={props.incident.action} />
      </dl>
    </div>
  );
};

export const VsphereAlertsTable: Component<{
  incidents: VmwareIncidentRow[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.incidents,
    initialStatus: 'all' as VmwareIncidentSeverityFilter,
    filter: filterVmwareIncidents,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'vsphere-alert-drawer' });
  const filteredEmptyState = () => getAlertFilteredEmptyState('vSphere health signals', 'severity');

  return (
    <Show
      when={props.incidents.length > 0}
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
            searchPlaceholder="Search vSphere health"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={VSPHERE_INCIDENT_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="signals"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title={filteredEmptyState().title}
              description={filteredEmptyState().description}
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Health Signals" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1040px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[20%]`}>
                    Resource
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[10%]`}>
                    Severity
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[34%]`}>
                    Signal
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                  >
                    vCenter
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[12%]`}
                  >
                    Entity
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden xl:table-cell md:w-[10%]`}
                  >
                    Started
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(incident) => {
                    const meta = () => incident.resource.vmware;
                    const detailRowId = () => drawer.detailRowId(incident);
                    const isExpanded = () => drawer.isExpanded(incident);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-vsphere-alert-row={incident.id}
                          onClick={() => drawer.toggle(incident)}
                          onKeyDown={drawer.handleActivationKey(incident)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <StatusDot
                                size="sm"
                                variant={severityVariant(incident.severityBucket)}
                                title={severityLabel(incident.severity)}
                              />
                              <div class="min-w-0">
                                <div
                                  class="truncate font-medium text-base-content"
                                  title={incident.resourceName}
                                >
                                  {incident.resourceName}
                                </div>
                                <div class="truncate text-[10px] text-muted">
                                  {formatResourceType(incident.resourceType)}
                                  <Show when={incident.resource.parentName}>
                                    {' '}
                                    on {incident.resource.parentName}
                                  </Show>
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <span
                              class={`text-[11px] font-semibold ${severityTextClass(
                                incident.severityBucket,
                              )}`}
                            >
                              {severityLabel(incident.severity)}
                            </span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span class="block truncate text-base-content" title={incident.summary}>
                              {incident.summary}
                            </span>
                            <span class="block truncate text-[10px] text-muted">
                              {incident.label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden md:table-cell`}
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
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                          >
                            <span class="block truncate" title={incident.managedObjectId}>
                              {formatEntityType(incident.entityType)}
                            </span>
                            <span class="block truncate text-[10px] text-muted">
                              {incident.managedObjectId}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content xl:table-cell`}
                          >
                            {formatStartedAt(incident.startedAt)}
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <TableRow
                            data-inline-detail-for={incident.id}
                            data-vsphere-alert-detail-row={incident.id}
                          >
                            <TableCell
                              id={detailRowId()}
                              colspan={6}
                              class="border-b border-border bg-surface-alt p-0"
                            >
                              <div
                                class="px-2 py-3 sm:px-4 sm:py-4"
                                onClick={(event) => event.stopPropagation()}
                              >
                                <AlertDetail
                                  incident={incident}
                                  onClose={() => drawer.close(incident)}
                                />
                              </div>
                            </TableCell>
                          </TableRow>
                        </Show>
                      </>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default VsphereAlertsTable;
