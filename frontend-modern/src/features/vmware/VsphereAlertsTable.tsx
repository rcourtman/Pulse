import { Show, createMemo, type Component, type JSX } from 'solid-js';
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
  PlatformResponsiveTableLabel,
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
  formatPlatformAlertEntityType,
  formatPlatformAlertResourceType,
  formatPlatformAlertStartedAt,
} from '@/utils/alertDetailPresentation';
import { getAlertFilteredEmptyState } from '@/utils/alertOverviewPresentation';
import {
  formatAlertSeverityLabel,
  getAlertSeverityDetailTone,
} from '@/utils/alertSeverityPresentation';
import {
  filterVmwareIncidents,
  buildVmwareHealthPosture,
  type VmwareIncidentRow,
  type VmwareIncidentSeverityFilter,
} from './vmwarePageModel';

const VSPHERE_INCIDENT_STATUS_OPTIONS =
  getPlatformAlertSeverityFilterOptions<VmwareIncidentSeverityFilter>({ includeAttention: true });

type AlertDetailSection = DetailSection;

const detailRow = makeDetailRow;

const buildAlertDetailSections = (incident: VmwareIncidentRow): AlertDetailSection[] => {
  const meta = incident.resource.vmware;
  return compactDetailSections([
    {
      label: 'Signal',
      rows: compactDetailRows([
        detailRow('Severity', formatAlertSeverityLabel(incident.severity), {
          tone: getAlertSeverityDetailTone(incident.severityBucket),
        }),
        detailRow('Summary', incident.summary),
        detailRow('Signal', incident.label),
        detailRow('Code', formatPlatformAlertCode(incident.code, 'vmware'), {
          title: incident.code,
        }),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        detailRow('Resource', incident.resourceName),
        detailRow('Type', formatPlatformAlertResourceType(incident.resourceType, 'vmware')),
        detailRow('Entity', formatPlatformAlertEntityType(incident.entityType)),
        detailRow('Managed object', incident.managedObjectId),
        detailRow('vCenter', meta?.connectionName || meta?.vcenterHost),
        detailRow('Datacenter', meta?.datacenterName),
        detailRow('Cluster', meta?.clusterName || meta?.computeResourceName),
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

const AlertDetail: Component<{ incident: VmwareIncidentRow; onClose: () => void }> = (props) => (
  <InlineDetailPanel
    testId="vsphere-alert-detail"
    detailFor={props.incident.id}
    title="vSphere health detail"
    summary={`${formatAlertSeverityLabel(props.incident.severity)} · ${formatPlatformAlertCode(
      props.incident.code,
      'vmware',
    )}`}
    sections={buildAlertDetailSections(props.incident)}
    detailAttributes={{ 'data-vsphere-alert-detail-for': props.incident.id }}
    onClose={props.onClose}
  />
);

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
  const posture = createMemo(() => buildVmwareHealthPosture(props.incidents));
  const statusOptions = createMemo(() =>
    withPlatformStatusCounts(VSPHERE_INCIDENT_STATUS_OPTIONS, tableState.countForStatus, {
      value: 'attention',
      tone: posture().critical > 0 ? 'danger' : 'warning',
    }),
  );
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
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={statusOptions()}
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
          <PlatformTableShell
            title="Health Signals"
            tableClass="min-w-full table-fixed text-xs md:min-w-[1040px]"
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
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-25 md:w-[34%]`}
                >
                  Signal
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 md:w-[14%]`}
                >
                  <PlatformResponsiveTableLabel compact="VC" full="vCenter" />
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[12%]`}
                >
                  Entity
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[10%]`}
                >
                  <PlatformResponsiveTableLabel compact="Age" full="Started" />
                </TableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={tableState.filtered} estimatedRowHeight={32}>
                  {(incident) => {
                    const meta = () => incident.resource.vmware;
                    const detailRowId = () => drawer.detailRowId(incident);
                    const isExpanded = () => drawer.isExpanded(incident);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-vsphere-alert-row={incident.id}
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
                                      'vmware',
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
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
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
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                          >
                            <span
                              class="block truncate"
                              title={[
                                formatPlatformAlertEntityType(incident.entityType),
                                incident.managedObjectId,
                              ]
                                .filter(Boolean)
                                .join(' · ')}
                            >
                              {formatPlatformAlertEntityType(incident.entityType)}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {formatPlatformAlertStartedAt(incident.startedAt)}
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={6}
                            data-inline-detail-for={incident.id}
                            data-vsphere-alert-detail-row={incident.id}
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

export default VsphereAlertsTable;
