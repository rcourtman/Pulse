import { For, Show, type Component, type JSX } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { PlatformAttentionSummary } from '@/features/platformPage/PlatformAttentionSummary';
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
  const posture = () => buildVmwareHealthPosture(props.incidents);
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
        <Show when={posture().attention > 0}>
          <PlatformAttentionSummary
            title="vSphere attention"
            headline={`${posture().attention} health signal${posture().attention === 1 ? '' : 's'} ${posture().attention === 1 ? 'needs' : 'need'} review`}
            description="Review affected resources before opening lower-priority informational signals. Provider identifiers remain available in each row's detail."
            tone={posture().critical > 0 ? 'danger' : 'warning'}
            metrics={[
              { label: 'critical', value: posture().critical },
              { label: 'warning', value: posture().warning },
              { label: 'resources', value: posture().affectedResources },
            ]}
            actions={
              <Button
                variant="secondary"
                size="sm"
                onClick={() =>
                  tableState.setStatus(tableState.status() === 'attention' ? 'all' : 'attention')
                }
              >
                {tableState.status() === 'attention' ? 'Show all signals' : 'Show attention'}
              </Button>
            }
          />
        </Show>
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
          <PlatformTableShell
            title="Health Signals"
            tableClass="min-w-full table-fixed text-xs md:min-w-[1040px]"
            header={
              <>
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
              </>
            }
            body={
              <>
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
                                  {formatPlatformAlertResourceType(incident.resourceType, 'vmware')}
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
                              {formatPlatformAlertEntityType(incident.entityType)}
                            </span>
                            <span class="block truncate text-[10px] text-muted">
                              {incident.managedObjectId}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content xl:table-cell`}
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
                </For>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default VsphereAlertsTable;
