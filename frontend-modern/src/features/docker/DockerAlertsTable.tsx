import { For, Show, type Component, type JSX } from 'solid-js';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import { AlertSeverityBadge, AlertSeverityDot } from '@/components/shared/AlertSeverityBadge';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import { getPlatformAlertSeverityFilterOptions } from '@/features/platformPage/platformAlertSeverityFilterOptions';
import type { ResourceType } from '@/types/resource';
import { getAlertFilteredEmptyState } from '@/utils/alertOverviewPresentation';
import {
  formatAlertSeverityLabel,
  getAlertSeverityDetailTone,
} from '@/utils/alertSeverityPresentation';
import {
  filterDockerIncidents,
  type DockerIncidentRow,
  type DockerIncidentSeverityFilter,
} from './dockerPageModel';

const DOCKER_INCIDENT_STATUS_OPTIONS =
  getPlatformAlertSeverityFilterOptions<DockerIncidentSeverityFilter>();

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

type AlertDetailSection = DetailSection;

const detailRow = makeDetailRow;

const buildAlertDetailSections = (incident: DockerIncidentRow): AlertDetailSection[] => {
  const docker = incident.resource.docker;
  return compactDetailSections([
    {
      label: 'Alert',
      rows: compactDetailRows([
        detailRow('Severity', formatAlertSeverityLabel(incident.severity), {
          tone: getAlertSeverityDetailTone(incident.severityBucket),
        }),
        detailRow('Summary', incident.summary),
        detailRow('Signal', incident.label),
        detailRow('Code', formatCode(incident.code), { title: incident.code }),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        detailRow('Name', incident.resourceName),
        detailRow('Type', formatResourceType(incident.resourceType)),
        detailRow('Host', docker?.hostname),
        detailRow('Runtime', docker?.runtime),
        detailRow('Swarm cluster', docker?.swarm?.clusterName),
        detailRow('Resource ID', incident.resourceId),
      ]),
    },
    {
      label: 'Source',
      rows: compactDetailRows([
        detailRow('Started', detailDateTime(incident.startedAt)),
        detailRow('Provider', incident.source),
      ]),
    },
    {
      label: 'Action',
      rows: compactDetailRows([detailRow('Recommended', incident.action)]),
    },
  ]);
};

const AlertDetail: Component<{ incident: DockerIncidentRow; onClose: () => void }> = (props) => (
  <InlineDetailPanel
    testId="docker-alert-detail"
    detailFor={props.incident.id}
    title="Docker alert detail"
    summary={`${formatAlertSeverityLabel(props.incident.severity)} · ${formatCode(
      props.incident.code,
    )}`}
    sections={buildAlertDetailSections(props.incident)}
    detailAttributes={{ 'data-docker-alert-detail-for': props.incident.id }}
    onClose={props.onClose}
  />
);

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
                              <AlertSeverityDot
                                size="sm"
                                severity={incident.severity}
                                bucket={incident.severityBucket}
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
                            <AlertSeverityBadge
                              severity={incident.severity}
                              bucket={incident.severityBucket}
                            />
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
