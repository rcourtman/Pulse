import { Show } from 'solid-js';

import { formatDiscoveryAge } from '@/api/discovery';
import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
} from '@/components/shared/DetailSectionTable';
import { TechnicalDetailsSection } from '@/components/shared/TechnicalDetailsDisclosure';
import { DrawerAttentionSection } from '@/components/shared/DrawerAttentionSection';
import type { Alert } from '@/types/api';
import { AvailabilityProbeStatusCards } from '@/components/Infrastructure/AvailabilityProbeStatusCard';
import type { DiscoveryIdentifiedSummary } from '@/utils/discoveryPresentation';
import { formatBytes } from '@/utils/format';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';
import { getWorkloadsGuestProtectionPresentation } from '@/utils/workloadGuestPresentation';

import { AvailabilityProbeSuggestionCard } from './AvailabilityProbeSuggestionCard';
import { buildWorkloadsDiskPresentation } from './diskListModel';
import {
  getGuestDrawerAlertMessage,
  getGuestDrawerMemoryRows,
  isGuestDrawerVM,
} from './guestDrawerModel';
import type { NestedWorkloadContext } from './nestedWorkloadContext';
import { WORKLOAD_ACTION_AGENT_LABEL } from './workloadAgentReadiness';

import type { GuestDrawerProps } from './guestDrawerModel';

interface GuestDrawerOverviewProps {
  guest: GuestDrawerProps['guest'];
  guestOsSummary: string;
  agentHeading: string;
  agentLabel: string;
  agentTitle: string;
  hasAgentInfo: boolean;
  hasFilesystemDetails: boolean;
  hasNetworkInterfaces: boolean;
  hasOsInfo: boolean;
  hasWorkloadActionAgent: boolean;
  showInGuestAgentInstallCue: boolean;
  ipAddresses: string[];
  networkInterfaces: NonNullable<GuestDrawerProps['guest']['networkInterfaces']>;
  nestedWorkloadContext?: NestedWorkloadContext;
  normalizedTags: string[];
  backupPresentation: {
    ageClass: string;
    ageLabel: string;
    dateLabel: string;
  } | null;
  diskThresholds?: MetricDisplayThresholds | null;
  discoveryIdentifiedSummary?: DiscoveryIdentifiedSummary | null;
  workloadActionAgentTitle: string;
  parentMemoryTotal?: GuestDrawerProps['parentMemoryTotal'];
  memoryDisplayBasis?: GuestDrawerProps['memoryDisplayBasis'];
  alerts?: Alert[];
}

export function GuestDrawerOverview(props: GuestDrawerOverviewProps) {
  const protectionPresentation = () =>
    getWorkloadsGuestProtectionPresentation({
      backupInProgress: props.guest.backupInProgress,
      ageLabel: props.backupPresentation?.ageLabel,
      ageClass: props.backupPresentation?.ageClass,
    });
  const coverageLabel = () => {
    if (props.hasWorkloadActionAgent) return WORKLOAD_ACTION_AGENT_LABEL;
    if (props.hasAgentInfo) return `${props.agentHeading} connected`;
    if (props.showInGuestAgentInstallCue) return 'Agent recommended';
    return null;
  };
  const overviewSections = () =>
    compactDetailSections([
      {
        label: 'Operator context',
        rows: compactDetailRows([
          makeDetailRow('System', props.hasOsInfo ? props.guestOsSummary : null),
          makeDetailRow('Pulse coverage', coverageLabel(), {
            title: props.hasWorkloadActionAgent
              ? props.workloadActionAgentTitle
              : props.hasAgentInfo
                ? props.agentTitle
                : undefined,
          }),
          makeDetailRow('Primary IP', props.ipAddresses[0]),
          makeDetailRow('Protection', protectionPresentation().label, {
            tone: protectionPresentation().tone,
          }),
          makeDetailRow(
            'Identified service',
            props.discoveryIdentifiedSummary
              ? [
                  props.discoveryIdentifiedSummary.serviceName,
                  props.discoveryIdentifiedSummary.category,
                ]
                  .filter(Boolean)
                  .join(' · ')
              : null,
          ),
        ]),
      },
    ]);

  const technicalSections = () => {
    const discovery = props.discoveryIdentifiedSummary;
    const nested = props.nestedWorkloadContext;
    const nestedItems = nested?.items.slice(0, 4) ?? [];
    const nestedHiddenCount = nested ? Math.max(0, nested.count - nestedItems.length) : 0;
    const showVmware =
      (props.guest.platformScopes?.includes('vmware-vsphere') ?? false) &&
      Boolean(
        props.guest.vmware?.connectionName ||
        props.guest.vmware?.vcenterHost ||
        props.guest.vmware?.datacenterName ||
        props.guest.vmware?.clusterName,
      );
    const diskRows = (props.guest.disks ?? []).map((disk, index) =>
      buildWorkloadsDiskPresentation(disk, index, props.diskThresholds),
    );

    return compactDetailSections([
      discovery
        ? {
            label: 'Identified Service',
            rows: compactDetailRows([
              makeDetailRow('Version', discovery.serviceVersion),
              makeDetailRow('Endpoint', discovery.suggestedUrl, { wrap: true }),
              discovery.portCount > 0 ? makeDetailRow('Ports', `${discovery.portCount}`) : null,
              makeDetailRow('Source', discovery.sourceLabel),
              makeDetailRow(
                'Observed',
                discovery.observedAt ? formatDiscoveryAge(discovery.observedAt) : null,
              ),
            ]),
          }
        : null,
      {
        label: 'System',
        rows: compactDetailRows([
          props.guest.cpus ? makeDetailRow('CPUs', `${props.guest.cpus}`) : null,
          props.hasAgentInfo
            ? makeDetailRow(props.agentHeading, props.agentLabel, { title: props.agentTitle })
            : null,
        ]),
      },
      nested
        ? {
            label: nested.title,
            rows: compactDetailRows([
              makeDetailRow('Containers', `${nested.count}`),
              ...nestedItems.map((item) => makeDetailRow(item.name, item.status)),
              nestedHiddenCount > 0
                ? makeDetailRow('Remaining', `${nestedHiddenCount} more`, { tone: 'muted' })
                : null,
            ]),
          }
        : null,
      showVmware
        ? {
            label: 'vSphere',
            rows: compactDetailRows([
              makeDetailRow(
                'vCenter',
                props.guest.vmware?.connectionName || props.guest.vmware?.vcenterHost,
                { title: props.guest.vmware?.vcenterHost },
              ),
              makeDetailRow('Datacenter', props.guest.vmware?.datacenterName),
              makeDetailRow('Cluster', props.guest.vmware?.clusterName),
            ]),
          }
        : null,
      props.ipAddresses.length > 1
        ? {
            label: 'Network identity',
            rows: compactDetailRows([
              makeDetailRow('Other IPs', props.ipAddresses.slice(1).join(', '), { wrap: true }),
            ]),
          }
        : null,
      {
        label: 'Memory',
        rows: compactDetailRows(
          getGuestDrawerMemoryRows(props.guest)
            .filter((row) => row.label !== 'Usage')
            .map((row) => makeDetailRow(row.label, row.value)),
        ),
      },
      props.normalizedTags.length > 0
        ? {
            label: 'Tags',
            rows: compactDetailRows([
              makeDetailRow('Values', props.normalizedTags.join(', '), { wrap: true }),
            ]),
          }
        : null,
      props.hasFilesystemDetails && diskRows.length > 0
        ? {
            label: 'Filesystems',
            rows: compactDetailRows([
              ...diskRows.map((disk) =>
                makeDetailRow(
                  disk.label,
                  [disk.usagePercentLabel, disk.usageText, disk.typeLabel]
                    .filter(Boolean)
                    .join(' · '),
                  {
                    title: disk.labelTitle,
                    wrap: true,
                    progress:
                      disk.progressValue === null
                        ? undefined
                        : {
                            value: disk.progressValue,
                            fillClass: disk.progressClass,
                            ariaLabel: `Filesystem ${disk.label} utilization`,
                          },
                  },
                ),
              ),
              makeDetailRow(
                'Status',
                isGuestDrawerVM(props.guest) ? props.guest.diskStatusReason : null,
                { wrap: true },
              ),
            ]),
          }
        : null,
      props.hasNetworkInterfaces
        ? {
            label: 'Network',
            rows: compactDetailRows(
              props.networkInterfaces.slice(0, 4).map((iface, index) => {
                const addresses = iface.addresses ?? [];
                const hasTraffic = (iface.rxBytes ?? 0) > 0 || (iface.txBytes ?? 0) > 0;
                const detail = [
                  addresses.join(', '),
                  iface.mac ? `MAC ${iface.mac}` : '',
                  hasTraffic
                    ? `RX ${formatBytes(iface.rxBytes ?? 0)} / TX ${formatBytes(
                        iface.txBytes ?? 0,
                      )}`
                    : '',
                ]
                  .filter(Boolean)
                  .join(' · ');
                return makeDetailRow(
                  iface.name || `Interface ${index + 1}`,
                  detail || 'No details',
                  {
                    wrap: true,
                  },
                );
              }),
            ),
          }
        : null,
    ]);
  };

  return (
    <div class="space-y-3">
      <DrawerAttentionSection
        items={(props.alerts ?? []).map((alert) => ({
          id: alert.id,
          message: getGuestDrawerAlertMessage(alert, {
            guest: props.guest,
            memoryDisplayBasis: props.memoryDisplayBasis,
            parentMemoryTotal: props.parentMemoryTotal,
          }),
          severity: alert.level,
          acknowledged: alert.acknowledged,
        }))}
      />
      <TechnicalDetailsSection
        dataTestId="guest-technical-details"
        sections={[...overviewSections(), ...technicalSections()]}
      />

      <div class="space-y-3">
        <Show when={props.guest.availability || props.guest.availabilityChecks?.length}>
          <div class="mt-3 max-w-sm">
            <div class="space-y-3">
              <AvailabilityProbeStatusCards
                availability={props.guest.availability}
                checks={props.guest.availabilityChecks}
              />
            </div>
          </div>
        </Show>
        <Show
          when={
            props.discoveryIdentifiedSummary?.suggestedAvailabilityProbe &&
            !props.guest.availability &&
            !props.guest.availabilityChecks?.length
          }
        >
          <div class="mt-3 max-w-sm">
            <AvailabilityProbeSuggestionCard
              suggestion={props.discoveryIdentifiedSummary!.suggestedAvailabilityProbe!}
              linkedResourceId={props.guest.id}
            />
          </div>
        </Show>
      </div>
    </div>
  );
}
