import { Show, type Component, type JSX } from 'solid-js';
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
  PlatformWindowedRows,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableShell,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import { getPlatformAlertSeverityFilterOptions } from '@/features/platformPage/platformAlertSeverityFilterOptions';
import {
  formatPlatformAlertCode,
  formatPlatformAlertDetailDateTime,
  formatPlatformAlertResourceType,
  formatPlatformAlertStartedAt,
} from '@/utils/alertDetailPresentation';
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
        detailRow('Code', formatPlatformAlertCode(incident.code, 'docker'), {
          title: incident.code,
        }),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        detailRow('Name', incident.resourceName),
        detailRow('Type', formatPlatformAlertResourceType(incident.resourceType, 'docker')),
        detailRow('Host', docker?.hostname),
        detailRow('Runtime', docker?.runtime),
        detailRow('Swarm cluster', docker?.swarm?.clusterName),
        detailRow('Resource ID', incident.resourceId),
      ]),
    },
    {
      label: 'Source',
      rows: compactDetailRows([
        detailRow('Started', formatPlatformAlertDetailDateTime(incident.startedAt)),
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
    summary={`${formatAlertSeverityLabel(props.incident.severity)} · ${formatPlatformAlertCode(
      props.incident.code,
      'docker',
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
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              DOCKER_INCIDENT_STATUS_OPTIONS,
              tableState.countForStatus,
            )}
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
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 w-[28%] md:w-[22%]`}
                >
                  Resource
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('badge')} platform-table-phone-hidden md:w-[10%]`}
                >
                  Severity
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-20 w-[25%] md:w-[34%]`}
                >
                  Alert
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 w-[15%] md:w-[14%]`}
                >
                  Host
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-10 w-[15%] md:w-[10%]`}
                >
                  Started
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-narrow-hidden hidden xl:table-cell md:w-[10%]`}
                >
                  Action
                </TableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={tableState.filtered} estimatedRowHeight={32}>
                  {(incident) => {
                    const docker = () => incident.resource.docker;
                    const detailRowId = () => drawer.detailRowId(incident);
                    const isExpanded = () => drawer.isExpanded(incident);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-docker-alert-row={incident.id}
                          onClick={() => drawer.toggle(incident)}
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
                                  title={[
                                    incident.resourceName,
                                    formatPlatformAlertResourceType(
                                      incident.resourceType,
                                      'docker',
                                    ),
                                    incident.resource.parentName
                                      ? `on ${incident.resource.parentName}`
                                      : '',
                                  ]
                                    .filter(Boolean)
                                    .join(' · ')}
                                >
                                  {incident.resourceName}
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('badge')} platform-table-phone-hidden`}
                          >
                            <AlertSeverityBadge
                              severity={incident.severity}
                              bucket={incident.severityBucket}
                            />
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span
                              class="block truncate text-base-content"
                              title={[incident.summary, incident.label].filter(Boolean).join(' · ')}
                            >
                              {incident.summary}
                            </span>
                          </TableCell>
                          <TableCell class={`${getPlatformTableCellClassForKind('text')}`}>
                            <span
                              class="block truncate text-base-content"
                              title={[
                                docker()?.hostname || docker()?.swarm?.clusterName,
                                docker()?.runtime ||
                                  formatPlatformAlertCode(incident.code, 'docker'),
                              ]
                                .filter(Boolean)
                                .join(' · ')}
                            >
                              {docker()?.hostname || docker()?.swarm?.clusterName || '-'}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {formatPlatformAlertStartedAt(incident.startedAt)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} platform-table-narrow-hidden hidden text-base-content xl:table-cell`}
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
                </PlatformWindowedRows>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default DockerAlertsTable;
