import type { Component, JSX } from 'solid-js';
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
import type { Resource } from '@/types/resource';
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
  filterTrueNASIncidents,
  type TrueNASIncidentRow,
  type TrueNASIncidentSeverityFilter,
} from './truenasPageModel';

const TRUENAS_INCIDENT_STATUS_OPTIONS =
  getPlatformAlertSeverityFilterOptions<TrueNASIncidentSeverityFilter>();

const buildAlertDetailSections = (incident: TrueNASIncidentRow): DetailSection[] => {
  const parentName = incident.resource.parentName?.trim();
  return compactDetailSections([
    {
      label: 'Alert',
      rows: compactDetailRows([
        makeDetailRow('Severity', formatAlertSeverityLabel(incident.severity), {
          tone: getAlertSeverityDetailTone(incident.severityBucket),
        }),
        makeDetailRow('Label', incident.label),
        makeDetailRow('Code', formatPlatformAlertCode(incident.code, 'truenas'), {
          title: incident.code,
        }),
        makeDetailRow('Category', formatPlatformAlertCode(incident.category, 'truenas')),
      ]),
    },
    {
      label: 'Source',
      rows: compactDetailRows([
        makeDetailRow('Provider', incident.source),
        makeDetailRow('Started', formatPlatformAlertDetailDateTime(incident.startedAt)),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        makeDetailRow('Name', incident.resourceName),
        makeDetailRow('Type', formatPlatformAlertResourceType(incident.resourceType, 'truenas')),
        makeDetailRow('Parent', parentName),
        makeDetailRow('Resource ID', incident.resourceId),
      ]),
    },
    {
      label: 'Action',
      rows: compactDetailRows([makeDetailRow('Recommended', incident.action)]),
    },
  ]);
};

export const TrueNASAlertsTable: Component<{
  incidents: TrueNASIncidentRow[];
  scope: readonly Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => (
  <PlatformAlertsTable
    platform="truenas"
    attrPrefix="truenas"
    incidents={props.incidents}
    emptyIcon={props.emptyIcon}
    emptyTitle={props.emptyTitle}
    emptyDescription={props.emptyDescription}
    showToolbar={props.showToolbar}
    searchPlaceholder="Search TrueNAS alerts"
    rowNoun="alerts"
    filteredEmptyNoun="TrueNAS alerts"
    shellTitle="Health Alerts"
    initialStatus={'all' as TrueNASIncidentSeverityFilter}
    filter={filterTrueNASIncidents}
    statusOptions={(countForStatus) =>
      withPlatformStatusCounts(TRUENAS_INCIDENT_STATUS_OPTIONS, countForStatus)
    }
    resourceColClass="platform-table-mobile-w-30 md:w-[20%]"
    severityHeader={<PlatformResponsiveTableLabel compact="Sev" full="Severity" />}
    summaryColClass="platform-table-mobile-w-25 md:w-[32%]"
    colspan={6}
    buildDetailSections={buildAlertDetailSections}
    trailingHeaders={
      <>
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
    renderTrailingCells={(incident, now) => (
      <>
        <TableCell class={getPlatformTableCellClassForKind('text')}>
          <span
            class="block truncate"
            title={[formatPlatformAlertCode(incident.code, 'truenas'), incident.source]
              .filter(Boolean)
              .join(' · ')}
          >
            {formatPlatformAlertCode(incident.code, 'truenas')}
          </span>
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
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
    )}
  />
);

export default TrueNASAlertsTable;
