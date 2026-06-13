import { For, Show, type Component, type JSX } from 'solid-js';
import { AlertSeverityBadge, AlertSeverityDot } from '@/components/shared/AlertSeverityBadge';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
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
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';
import type { Resource, ResourceType } from '@/types/resource';
import { getAlertFilteredEmptyState } from '@/utils/alertOverviewPresentation';
import { formatAlertSeverityLabel } from '@/utils/alertSeverityPresentation';

const TRUENAS_INCIDENT_STATUS_OPTIONS: PlatformTableFilterOption<TrueNASIncidentSeverityFilter>[] =
  [
    { value: 'all', label: 'All' },
    { value: 'critical', label: 'Critical', tone: 'danger' },
    { value: 'warning', label: 'Warning', tone: 'warning' },
    { value: 'info', label: 'Info', tone: 'success' },
  ];

const formatResourceType = (type: ResourceType): string => {
  switch (type) {
    case 'agent':
      return 'System';
    case 'storage':
    case 'pool':
      return 'Pool';
    case 'dataset':
      return 'Dataset';
    case 'physical_disk':
      return 'Disk';
    case 'network-share':
      return 'Share';
    case 'vm':
      return 'VM';
    case 'app-container':
      return 'App';
    default:
      return type;
  }
};

const formatCode = (code: string): string => {
  const normalized = code.trim().replace(/^truenas_/, '');
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
  return parsed.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

type AlertDetailTone = DetailValueTone;
type AlertDetailSection = DetailSection;

const detailDateTime = (value?: string): string | null => {
  if (!value) return null;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const detailRow = makeDetailRow;

const alertTone = (severity: TrueNASIncidentRow['severityBucket']): AlertDetailTone => {
  if (severity === 'critical') return 'danger';
  if (severity === 'warning') return 'warning';
  return 'muted';
};

const buildAlertDetailSections = (incident: TrueNASIncidentRow): AlertDetailSection[] => {
  const parentName = incident.resource.parentName?.trim();
  return compactDetailSections([
    {
      label: 'Alert',
      rows: compactDetailRows([
        detailRow('Severity', formatAlertSeverityLabel(incident.severity), {
          tone: alertTone(incident.severityBucket),
        }),
        detailRow('Summary', incident.summary),
        detailRow('Label', incident.label),
        detailRow('Code', formatCode(incident.code), { title: incident.code }),
        detailRow('Category', formatCode(incident.category)),
      ]),
    },
    {
      label: 'Source',
      rows: compactDetailRows([
        detailRow('Provider', incident.source),
        detailRow('Started', detailDateTime(incident.startedAt)),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        detailRow('Name', incident.resourceName),
        detailRow('Type', formatResourceType(incident.resourceType)),
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
    summary={`${formatAlertSeverityLabel(props.incident.severity)} · ${formatCode(
      props.incident.code,
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
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={TRUENAS_INCIDENT_STATUS_OPTIONS}
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
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[20%]`}>
                  Resource
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[10%]`}>
                  Severity
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[32%]`}>
                  Alert
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[13%]`}
                >
                  Source
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[10%]`}
                >
                  Started
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
                <For each={tableState.filtered()}>
                  {(incident) => {
                    const detailRowId = () => drawer.detailRowId(incident);
                    const isExpanded = () => drawer.isExpanded(incident);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-truenas-alert-row={incident.id}
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
                          <TableCell class={`${getPlatformTableCellClassForKind('text')}`}>
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
                            <span class="block truncate" title={incident.code}>
                              {formatCode(incident.code)}
                            </span>
                            <span class="block truncate text-[10px] text-muted">
                              {incident.source}
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
                </For>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default TrueNASAlertsTable;
