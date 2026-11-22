import { Show, type JSX } from 'solid-js';
import { InlineDetailPanel, type DetailSection } from '@/components/shared/DetailSectionTable';
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
  type PlatformTableFilterOption,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import { RecordOwnerLink } from '@/features/platformPage/RecordOwnerLink';
import {
  formatPlatformAlertCode,
  formatPlatformAlertResourceType,
} from '@/utils/alertDetailPresentation';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { getAlertFilteredEmptyState } from '@/utils/alertOverviewPresentation';
import { getAlertSeverityIndicator } from '@/utils/alertSeverityPresentation';
import type { Resource, ResourceType } from '@/types/resource';

export type PlatformAlertTablePlatform = 'docker' | 'kubernetes' | 'truenas' | 'vmware';
export type PlatformAlertTableAttrPrefix = 'docker' | 'kubernetes' | 'truenas' | 'vsphere';
export type PlatformAlertSeverityBucket = 'critical' | 'warning' | 'info';

export interface PlatformAlertIncidentBase {
  id: string;
  resource: Resource;
  resourceId: string;
  resourceName: string;
  resourceType: ResourceType;
  severity: string;
  severityBucket: PlatformAlertSeverityBucket;
  code: string;
  source: string;
  summary: string;
  label: string;
  startedAt?: string;
  action: string;
}

export interface PlatformAlertsTableProps<T extends PlatformAlertIncidentBase, S extends string> {
  platform: PlatformAlertTablePlatform;
  attrPrefix: PlatformAlertTableAttrPrefix;
  incidents: T[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
  searchPlaceholder: string;
  rowNoun: string;
  filteredEmptyNoun: string;
  shellTitle: string;
  tableClass?: string;
  initialStatus: S;
  filter: (items: T[], search: string, status: S) => T[];
  statusOptions: (countForStatus: (status: S) => number) => PlatformTableFilterOption<S>[];
  resourceColClass?: string;
  severityHeader?: JSX.Element;
  summaryHeader?: string;
  summaryColClass?: string;
  resourceTitleMeta?: (incident: T) => string | undefined;
  trailingHeaders: JSX.Element;
  renderTrailingCells: (incident: T, now: () => number) => JSX.Element;
  colspan: number;
  buildDetailSections: (incident: T) => DetailSection[];
}

export function PlatformAlertsTable<T extends PlatformAlertIncidentBase, S extends string>(
  props: PlatformAlertsTableProps<T, S>,
) {
  const now = useRelativeTimeNow();
  const tableState = createPlatformTableFilterState({
    resources: () => props.incidents,
    initialStatus: props.initialStatus,
    filter: props.filter,
  });
  const drawer = createPlatformResourceDetailState({
    idPrefix: `${props.attrPrefix}-alert-drawer`,
  });
  const filteredEmptyState = () => getAlertFilteredEmptyState(props.filteredEmptyNoun, 'severity');

  const rowAttrs = (id: string) => ({
    [`data-${props.attrPrefix}-alert-row`]: id,
  });

  const detailRowAttrs = (id: string) => ({
    'data-inline-detail-for': id,
    [`data-${props.attrPrefix}-alert-detail-row`]: id,
  });

  const detailPanelAttrs = (id: string) => ({
    [`data-${props.attrPrefix}-alert-detail-for`]: id,
  });

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
            searchPlaceholder={props.searchPlaceholder}
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={props.statusOptions(tableState.countForStatus)}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun={props.rowNoun}
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
            title={props.shellTitle}
            tableClass={props.tableClass ?? 'min-w-full table-fixed text-xs md:min-w-[960px]'}
            header={
              <>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('name')} ${props.resourceColClass ?? 'platform-table-mobile-w-30 md:w-[22%]'}`}
                >
                  Resource
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('badge')} platform-table-phone-hidden md:w-[10%]`}
                >
                  {props.severityHeader ?? 'Severity'}
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} ${props.summaryColClass ?? 'platform-table-mobile-w-25 md:w-[34%]'}`}
                >
                  {props.summaryHeader ?? 'Alert'}
                </TableHead>
                {props.trailingHeaders}
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={tableState.filtered} estimatedRowHeight={32}>
                  {(incident) => {
                    const detailRowId = () => drawer.detailRowId(incident);
                    const isExpanded = () => drawer.isExpanded(incident);
                    const titleMeta = () =>
                      props.resourceTitleMeta
                        ? props.resourceTitleMeta(incident)
                        : incident.resource.parentName
                          ? `on ${incident.resource.parentName}`
                          : '';
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          {...rowAttrs(incident.id)}
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
                                      props.platform,
                                    ),
                                    titleMeta(),
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
                          {props.renderTrailingCells(incident, now)}
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={props.colspan}
                            {...detailRowAttrs(incident.id)}
                          >
                            <InlineDetailPanel
                              testId={`${props.attrPrefix}-alert-detail`}
                              detailFor={incident.id}
                              title={
                                incident.resourceName ||
                                formatPlatformAlertCode(incident.code, props.platform)
                              }
                              status={getAlertSeverityIndicator(
                                incident.severity,
                                incident.severityBucket,
                              )}
                              summary={incident.summary || incident.label}
                              actions={
                                <RecordOwnerLink
                                  subject={{
                                    id: incident.resourceId,
                                    name: incident.resourceName,
                                    type: incident.resourceType,
                                    resource: incident.resource,
                                  }}
                                />
                              }
                              sections={props.buildDetailSections(incident)}
                              detailAttributes={detailPanelAttrs(incident.id)}
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
}
