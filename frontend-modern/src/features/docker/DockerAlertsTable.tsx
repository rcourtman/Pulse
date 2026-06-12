import { For, Show, type Component, type JSX } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  platformChipStatusDot,
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
import { getAlertFilteredEmptyState } from '@/utils/alertOverviewPresentation';
import {
  filterDockerIncidents,
  type DockerIncidentRow,
  type DockerIncidentSeverityFilter,
} from './dockerPageModel';

const DOCKER_INCIDENT_STATUS_OPTIONS: PlatformTableFilterOption<DockerIncidentSeverityFilter>[] = [
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

const severityVariant = (severity: DockerIncidentRow['severityBucket']): StatusIndicatorVariant => {
  switch (severity) {
    case 'critical':
      return 'danger';
    case 'warning':
      return 'warning';
    case 'info':
      return 'muted';
  }
};

const severityLabel = (severity: string): string => {
  const normalized = severity.trim();
  if (!normalized) return 'Info';
  return normalized.charAt(0).toUpperCase() + normalized.slice(1).toLowerCase();
};

const severityTextClass = (severity: DockerIncidentRow['severityBucket']): string => {
  switch (severity) {
    case 'critical':
      return 'text-red-700 dark:text-red-300';
    case 'warning':
      return 'text-amber-700 dark:text-amber-300';
    case 'info':
      return 'text-muted';
  }
};

const formatResourceType = (type: ResourceType): string => {
  switch (type) {
    case 'agent':
      return 'Host';
    case 'app-container':
      return 'Container';
    case 'docker-service':
      return 'Service';
    case 'docker-task':
      return 'Task';
    case 'docker-swarm-node':
      return 'Swarm Node';
    case 'docker-image':
      return 'Image';
    case 'docker-volume':
      return 'Volume';
    case 'docker-network':
      return 'Network';
    case 'docker-secret':
      return 'Secret';
    case 'docker-config':
      return 'Config';
    default:
      return type;
  }
};

const formatCode = (code: string): string => {
  const normalized = code.trim().replace(/^docker_/, '');
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

const AlertDetail: Component<{ incident: DockerIncidentRow; onClose: () => void }> = (props) => {
  const docker = () => props.incident.resource.docker;
  return (
    <div data-testid="docker-alert-detail" class="space-y-3">
      <div class="flex min-w-0 items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="text-sm font-semibold text-base-content">Docker alert detail</div>
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
        <DetailField label="Resource" value={props.incident.resourceName} />
        <DetailField label="Type" value={formatResourceType(props.incident.resourceType)} />
        <DetailField label="Host" value={docker()?.hostname} />
        <DetailField label="Runtime" value={docker()?.runtime} />
        <DetailField label="Swarm cluster" value={docker()?.swarm?.clusterName} />
        <DetailField label="Started" value={detailDateTime(props.incident.startedAt)} />
        <DetailField label="Source" value={props.incident.source} />
        <DetailField label="Action" value={props.incident.action} />
      </dl>
    </div>
  );
};

export const DockerAlertsTable: Component<{
  incidents: DockerIncidentRow[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.incidents,
    initialStatus: 'all' as DockerIncidentSeverityFilter,
    filter: filterDockerIncidents,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-alert-drawer' });
  const filteredEmptyState = () => getAlertFilteredEmptyState('Docker alerts', 'severity');

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
            searchPlaceholder="Search Docker alerts"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={DOCKER_INCIDENT_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="alerts"
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
          <PlatformTableShell
            title="Active Alerts"
            tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
            header={
              <>
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[22%]`}>
                  Resource
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[10%]`}>
                  Severity
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[34%]`}>
                  Alert
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                >
                  Host
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[10%]`}
                >
                  Started
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden xl:table-cell md:w-[10%]`}
                >
                  Action
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
                  {(incident) => {
                    const docker = () => incident.resource.docker;
                    const detailRowId = () => drawer.detailRowId(incident);
                    const isExpanded = () => drawer.isExpanded(incident);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-docker-alert-row={incident.id}
                          onClick={() => drawer.toggle(incident)}
                          onKeyDown={drawer.handleActivationKey(incident)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={incident.resourceName}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(incident)}
                              />
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
                              title={docker()?.hostname || docker()?.swarm?.clusterName}
                            >
                              {docker()?.hostname || docker()?.swarm?.clusterName || '-'}
                            </span>
                            <span class="block truncate text-[10px] text-muted">
                              {docker()?.runtime || formatCode(incident.code)}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                          >
                            {formatStartedAt(incident.startedAt)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content xl:table-cell`}
                          >
                            <span class="block truncate" title={incident.action}>
                              {incident.action}
                            </span>
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={6}
                            data-inline-detail-for={incident.id}
                            data-docker-alert-detail-row={incident.id}
                          >
                            <AlertDetail
                              incident={incident}
                              onClose={() => drawer.close(incident)}
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

export default DockerAlertsTable;
