import { Show, type Component, type JSX } from 'solid-js';
import { AlertSeverityBadge, AlertSeverityDot } from '@/components/shared/AlertSeverityBadge';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  PlatformWindowedRows,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  PlatformResponsiveTableLabel,
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
  filterTrueNASIncidents,
  type TrueNASIncidentRow,
  type TrueNASIncidentSeverityFilter,
} from './truenasPageModel';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import type { Resource } from '@/types/resource';
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

const TRUENAS_INCIDENT_STATUS_OPTIONS =
  getPlatformAlertSeverityFilterOptions<TrueNASIncidentSeverityFilter>();

type AlertDetailSection = DetailSection;

const detailRow = makeDetailRow;

const buildAlertDetailSections = (incident: TrueNASIncidentRow): AlertDetailSection[] => {
  const parentName = incident.resource.parentName?.trim();
  return compactDetailSections([
    {
      label: 'Alert',
      rows: compactDetailRows([
        detailRow('Severity', formatAlertSeverityLabel(incident.severity), {
          tone: getAlertSeverityDetailTone(incident.severityBucket),
        }),
        detailRow('Summary', incident.summary),
        detailRow('Label', incident.label),
        detailRow('Code', formatPlatformAlertCode(incident.code, 'truenas'), {
          title: incident.code,
        }),
        detailRow('Category', formatPlatformAlertCode(incident.category, 'truenas')),
      ]),
    },
    {
      label: 'Source',
      rows: compactDetailRows([
        detailRow('Provider', incident.source),
        detailRow('Started', formatPlatformAlertDetailDateTime(incident.startedAt)),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        detailRow('Name', incident.resourceName),
        detailRow('Type', formatPlatformAlertResourceType(incident.resourceType, 'truenas')),
        detailRow('Parent', parentName),
        detailRow('Resource ID', incident.resourceId),
      ]),
    },
    {
      label: 'Action',
      rows: compactDetailRows([detailRow('Recommended', incident.action)]),
    },
  ]);
};

const AlertDetailTable: Component<{ incident: TrueNASIncidentRow; onClose: () => void }> = (
  props,
) => (
  <InlineDetailPanel
    testId="truenas-alert-detail"
    detailFor={props.incident.id}
    title="Alert detail"
    summary={`${formatAlertSeverityLabel(props.incident.severity)} · ${formatPlatformAlertCode(
      props.incident.code,
      'truenas',
    )}`}
    sections={buildAlertDetailSections(props.incident)}
    detailAttributes={{ 'data-truenas-alert-detail-for': props.incident.id }}
    onClose={props.onClose}
  />
);

export const TrueNASAlertsTable: Component<{
  incidents: TrueNASIncidentRow[];
  scope: readonly Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.incidents,
    initialStatus: 'all' as TrueNASIncidentSeverityFilter,
    filter: filterTrueNASIncidents,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'truenas-alert-drawer' });
  const filteredEmptyState = () => getAlertFilteredEmptyState('TrueNAS alerts', 'severity');

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
            searchPlaceholder="Search TrueNAS alerts"
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              TRUENAS_INCIDENT_STATUS_OPTIONS,
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
            title="Health Alerts"
            tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
            header={
              <>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-30 md:w-[20%]`}
                >
                  Resource
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('badge')} platform-table-phone-hidden md:w-[10%]`}
                >
                  <PlatformResponsiveTableLabel compact="Sev" full="Severity" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-25 md:w-[32%]`}
                >
                  Alert
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 md:w-[13%]`}
                >
                  <PlatformResponsiveTableLabel compact="Src" full="Source" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[10%]`}
                >
                  <PlatformResponsiveTableLabel compact="Age" full="Started" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden xl:table-cell md:w-[15%]`}
                >
                  Action
                </TableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={tableState.filtered} estimatedRowHeight={32}>
                  {(incident) => {
                    const detailRowId = () => drawer.detailRowId(incident);
                    const isExpanded = () => drawer.isExpanded(incident);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-truenas-alert-row={incident.id}
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
                                      'truenas',
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
                          <TableCell class={`${getPlatformTableCellClassForKind('text')}`}>
                            <span
                              class="block truncate text-base-content"
                              title={[incident.summary, incident.label].filter(Boolean).join(' · ')}
                            >
                              {incident.summary}
                            </span>
                          </TableCell>
                          <TableCell class={`${getPlatformTableCellClassForKind('text')}`}>
                            <span
                              class="block truncate"
                              title={[
                                formatPlatformAlertCode(incident.code, 'truenas'),
                                incident.source,
                              ]
                                .filter(Boolean)
                                .join(' · ')}
                            >
                              {formatPlatformAlertCode(incident.code, 'truenas')}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {formatPlatformAlertStartedAt(incident.startedAt)}
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
                            data-truenas-alert-detail-row={incident.id}
                          >
                            <AlertDetailTable
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

export default TrueNASAlertsTable;
