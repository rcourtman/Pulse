import type { Component, JSX } from 'solid-js';
import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import { TableCell, TableHead } from '@/components/shared/Table';
import {
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import { PlatformAlertsTable } from '@/features/platformPage/PlatformAlertsTable';
import { getPlatformAlertSeverityFilterOptions } from '@/features/platformPage/platformAlertSeverityFilterOptions';
import {
  formatPlatformAlertCode,
  formatPlatformAlertDetailDateTime,
  formatPlatformAlertResourceType,
  formatPlatformAlertStartedAt,
} from '@/utils/alertDetailPresentation';
import {
  formatAlertSeverityLabel,
  getAlertSeverityDetailTone,
} from '@/utils/alertSeverityPresentation';
import {
  filterKubernetesIncidents,
  type KubernetesIncidentRow,
  type KubernetesIncidentSeverityFilter,
} from './kubernetesPageModel';

const KUBERNETES_INCIDENT_STATUS_OPTIONS =
  getPlatformAlertSeverityFilterOptions<KubernetesIncidentSeverityFilter>();

const buildAlertDetailSections = (incident: KubernetesIncidentRow): DetailSection[] => {
  const k = incident.resource.kubernetes;
  const owner = k?.ownerKind && k?.ownerName ? `${k.ownerKind}/${k.ownerName}` : undefined;
  return compactDetailSections([
    {
      label: 'Alert',
      rows: compactDetailRows([
        makeDetailRow('Severity', formatAlertSeverityLabel(incident.severity), {
          tone: getAlertSeverityDetailTone(incident.severityBucket),
        }),
        makeDetailRow('Signal', incident.label),
        makeDetailRow('Code', formatPlatformAlertCode(incident.code, 'kubernetes'), {
          title: incident.code,
        }),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        makeDetailRow('Name', incident.resourceName),
        makeDetailRow('Type', formatPlatformAlertResourceType(incident.resourceType, 'kubernetes')),
        makeDetailRow('Cluster', k?.clusterName || k?.clusterId),
        makeDetailRow('Namespace', k?.namespace),
        makeDetailRow('Node', k?.nodeName),
        makeDetailRow('Owner', owner),
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

export const KubernetesAlertsTable: Component<{
  incidents: KubernetesIncidentRow[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => (
  <PlatformAlertsTable
    platform="kubernetes"
    attrPrefix="kubernetes"
    incidents={props.incidents}
    emptyIcon={props.emptyIcon}
    emptyTitle={props.emptyTitle}
    emptyDescription={props.emptyDescription}
    showToolbar={props.showToolbar}
    searchPlaceholder="Search Kubernetes alerts"
    rowNoun="alerts"
    filteredEmptyNoun="Kubernetes alerts"
    shellTitle="Active Alerts"
    initialStatus={'all' as KubernetesIncidentSeverityFilter}
    filter={filterKubernetesIncidents}
    statusOptions={(countForStatus) =>
      withPlatformStatusCounts(KUBERNETES_INCIDENT_STATUS_OPTIONS, countForStatus)
    }
    resourceTitleMeta={(incident) => {
      const k = incident.resource.kubernetes;
      return k?.ownerKind && k?.ownerName ? `${k.ownerKind}/${k.ownerName}` : '';
    }}
    colspan={6}
    buildDetailSections={buildAlertDetailSections}
    trailingHeaders={
      <>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 md:w-[14%]`}
        >
          Scope
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[10%]`}
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
    renderTrailingCells={(incident, now) => {
      const k = () => incident.resource.kubernetes;
      const scopeText = () => {
        const cluster = k()?.clusterName || k()?.clusterId;
        const namespace = k()?.namespace;
        if (cluster && namespace) return `${cluster}/${namespace}`;
        return cluster || namespace || '-';
      };
      return (
        <>
          <TableCell class={getPlatformTableCellClassForKind('text')}>
            <span
              class="block truncate text-base-content"
              title={[
                scopeText(),
                k()?.nodeName || formatPlatformAlertCode(incident.code, 'kubernetes'),
              ]
                .filter(Boolean)
                .join(' · ')}
            >
              {scopeText()}
            </span>
          </TableCell>
          <TableCell
            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
          >
            <span title={formatPlatformAlertDetailDateTime(incident.startedAt)}>
              {formatPlatformAlertStartedAt(incident.startedAt, now())}
            </span>
          </TableCell>
          <TableCell
            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content xl:table-cell`}
          >
            <span class="block truncate" title={incident.action}>
              {incident.action}
            </span>
          </TableCell>
        </>
      );
    }}
  />
);

export default KubernetesAlertsTable;
