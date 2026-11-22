import { Show, createMemo, type Component, type JSX } from 'solid-js';
import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import { TableCell, TableHead } from '@/components/shared/Table';
import {
  PlatformResponsiveTableLabel,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import { PlatformAlertsTable } from '@/features/platformPage/PlatformAlertsTable';
import { getPlatformAlertSeverityFilterOptions } from '@/features/platformPage/platformAlertSeverityFilterOptions';
import {
  formatPlatformAlertCode,
  formatPlatformAlertDetailDateTime,
  formatPlatformAlertEntityType,
  formatPlatformAlertResourceType,
  formatPlatformAlertStartedAt,
} from '@/utils/alertDetailPresentation';
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

const buildAlertDetailSections = (incident: VmwareIncidentRow): DetailSection[] => {
  const meta = incident.resource.vmware;
  return compactDetailSections([
    {
      label: 'Signal',
      rows: compactDetailRows([
        makeDetailRow('Severity', formatAlertSeverityLabel(incident.severity), {
          tone: getAlertSeverityDetailTone(incident.severityBucket),
        }),
        makeDetailRow('Signal', incident.label),
        makeDetailRow('Code', formatPlatformAlertCode(incident.code, 'vmware'), {
          title: incident.code,
        }),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        makeDetailRow('Resource', incident.resourceName),
        makeDetailRow('Type', formatPlatformAlertResourceType(incident.resourceType, 'vmware')),
        makeDetailRow('Entity', formatPlatformAlertEntityType(incident.entityType)),
        makeDetailRow('Managed object', incident.managedObjectId),
        makeDetailRow('vCenter', meta?.connectionName || meta?.vcenterHost),
        makeDetailRow('Datacenter', meta?.datacenterName),
        makeDetailRow('Cluster', meta?.clusterName || meta?.computeResourceName),
        makeDetailRow('Resource ID', incident.resourceId),
      ]),
    },
    {
      label: 'Source',
      rows: compactDetailRows([
        makeDetailRow('Started', formatPlatformAlertDetailDateTime(incident.startedAt)),
        makeDetailRow('Provider', incident.source),
      ]),
    },
    {
      label: 'Action',
      rows: compactDetailRows([makeDetailRow('Recommended', incident.action)]),
    },
  ]);
};

export const VsphereAlertsTable: Component<{
  incidents: VmwareIncidentRow[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
  // One vCenter connection: its name would repeat on every row, so the
  // column hides (the row drawer still names it).
  singleVcenter?: boolean;
}> = (props) => {
  const posture = createMemo(() => buildVmwareHealthPosture(props.incidents));

  return (
    <PlatformAlertsTable
      platform="vmware"
      attrPrefix="vsphere"
      incidents={props.incidents}
      emptyIcon={props.emptyIcon}
      emptyTitle={props.emptyTitle}
      emptyDescription={props.emptyDescription}
      showToolbar={props.showToolbar}
      searchPlaceholder="Search vSphere health"
      rowNoun="signals"
      filteredEmptyNoun="vSphere health signals"
      shellTitle="Health Signals"
      tableClass="min-w-full table-fixed text-xs md:min-w-[1040px]"
      initialStatus={'all' as VmwareIncidentSeverityFilter}
      filter={filterVmwareIncidents}
      statusOptions={(countForStatus) =>
        withPlatformStatusCounts(VSPHERE_INCIDENT_STATUS_OPTIONS, countForStatus, {
          value: 'attention',
          tone: posture().critical > 0 ? 'danger' : 'warning',
        })
      }
      resourceColClass="platform-table-mobile-w-30 md:w-[20%]"
      severityHeader={<PlatformResponsiveTableLabel compact="Sev" full="Severity" />}
      summaryHeader="Signal"
      summaryColClass="platform-table-mobile-w-25 md:w-[34%]"
      colspan={props.singleVcenter ? 5 : 6}
      buildDetailSections={buildAlertDetailSections}
      trailingHeaders={
        <>
          <Show when={!props.singleVcenter}>
            <TableHead
              class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 md:w-[14%]`}
            >
              <PlatformResponsiveTableLabel compact="VC" full="vCenter" />
            </TableHead>
          </Show>
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
      renderTrailingCells={(incident, now) => {
        const meta = () => incident.resource.vmware;
        return (
          <>
            <Show when={!props.singleVcenter}>
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
            </Show>
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
              <span title={formatPlatformAlertDetailDateTime(incident.startedAt)}>
                {formatPlatformAlertStartedAt(incident.startedAt, now())}
              </span>
            </TableCell>
          </>
        );
      }}
    />
  );
};

export default VsphereAlertsTable;
