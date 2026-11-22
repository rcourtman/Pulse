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
  filterDockerIncidents,
  type DockerIncidentRow,
  type DockerIncidentSeverityFilter,
} from './dockerPageModel';
import { useDockerHostIdentities, type DockerHostIdentities } from './dockerHostIdentity';

const DOCKER_INCIDENT_STATUS_OPTIONS =
  getPlatformAlertSeverityFilterOptions<DockerIncidentSeverityFilter>();

const buildAlertDetailSections = (
  incident: DockerIncidentRow,
  hostIdentities: DockerHostIdentities,
): DetailSection[] => {
  const docker = incident.resource.docker;
  return compactDetailSections([
    {
      label: 'Alert',
      rows: compactDetailRows([
        makeDetailRow('Severity', formatAlertSeverityLabel(incident.severity), {
          tone: getAlertSeverityDetailTone(incident.severityBucket),
        }),
        makeDetailRow('Signal', incident.label),
        makeDetailRow('Code', formatPlatformAlertCode(incident.code, 'docker'), {
          title: incident.code,
        }),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        makeDetailRow('Name', incident.resourceName),
        makeDetailRow('Type', formatPlatformAlertResourceType(incident.resourceType, 'docker')),
        makeDetailRow('Host', docker?.hostname && hostIdentities.labelFor(incident.resource)),
        makeDetailRow('Runtime', docker?.runtime),
        makeDetailRow('Swarm cluster', docker?.swarm?.clusterName),
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

export const DockerAlertsTable: Component<{
  incidents: DockerIncidentRow[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const hostIdentities = useDockerHostIdentities();

  return (
    <PlatformAlertsTable
      platform="docker"
      attrPrefix="docker"
      incidents={props.incidents}
      emptyIcon={props.emptyIcon}
      emptyTitle={props.emptyTitle}
      emptyDescription={props.emptyDescription}
      showToolbar={props.showToolbar}
      searchPlaceholder="Search Docker alerts"
      rowNoun="alerts"
      filteredEmptyNoun="Docker alerts"
      shellTitle="Active Alerts"
      initialStatus={'all' as DockerIncidentSeverityFilter}
      filter={filterDockerIncidents}
      statusOptions={(countForStatus) =>
        withPlatformStatusCounts(DOCKER_INCIDENT_STATUS_OPTIONS, countForStatus)
      }
      resourceColClass="platform-table-mobile-w-30 w-[28%] md:w-[22%]"
      summaryColClass="platform-table-mobile-w-20 w-[25%] md:w-[34%]"
      colspan={6}
      buildDetailSections={(incident) => buildAlertDetailSections(incident, hostIdentities())}
      trailingHeaders={
        <>
          <TableHead
            class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-15 w-[15%] md:w-[14%]`}
          >
            Host
          </TableHead>
          <TableHead
            class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-10 w-[15%] md:w-[10%]`}
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
      renderTrailingCells={(incident, now) => {
        const docker = () => incident.resource.docker;
        const hostLabel = () =>
          docker()?.hostname
            ? hostIdentities().labelFor(incident.resource)
            : docker()?.swarm?.clusterName;
        return (
          <>
            <TableCell class={getPlatformTableCellClassForKind('text')}>
              <span
                class="block truncate text-base-content"
                title={[
                  hostLabel(),
                  docker()?.runtime || formatPlatformAlertCode(incident.code, 'docker'),
                ]
                  .filter(Boolean)
                  .join(' · ')}
              >
                {hostLabel() || '-'}
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
              class={`${getPlatformTableCellClassForKind('text')} platform-table-narrow-hidden hidden text-base-content xl:table-cell`}
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
};

export default DockerAlertsTable;
