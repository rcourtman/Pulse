import { ALERTS_EMPTY_STATE } from '@/utils/alertOverviewPresentation';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { For, Show, Suspense, createMemo, lazy } from 'solid-js';

import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
} from '@/components/shared/DetailSectionTable';
import {
  TechnicalDetailsDisclosure,
  TechnicalDetailsSection,
} from '@/components/shared/TechnicalDetailsDisclosure';
import { DrawerAttentionSection } from '@/components/shared/DrawerAttentionSection';
import { DrawerExceptionList } from './DrawerExceptionList';
import type { Alert } from '@/types/api';
import { AvailabilityProbeStatusCards } from '@/components/Infrastructure/AvailabilityProbeStatusCard';
import { HOST_AGENT_STOPPED_REPORTING_REASON } from '@/components/Infrastructure/resourceDetailMappers';
import { formatBytes } from '@/utils/format';
import { getShippedDocUrl } from '@/utils/docsLinks';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';
import {
  getWorkloadGuestDiskRead,
  getWorkloadsGuestProtectionPresentation,
} from '@/utils/workloadGuestPresentation';

import { buildWorkloadsDiskPresentation } from './diskListModel';
import type { HistoryTimeRange } from '@/api/charts';
import {
  GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  GUEST_DRAWER_HISTORY_GROUPS,
  getGuestDrawerAlertAttention,
  getGuestDrawerMemoryRows,
  getGuestDrawerMemoryReading,
  getGuestDrawerGuestReadPresentation,
  isGuestDrawerVM,
  type GuestDrawerBackupPresentation,
  type GuestDrawerHistoryDeferredMetric,
  type GuestDrawerHistoryTarget,
} from './guestDrawerModel';
import {
  collectDrawerExceptionRows,
  getDrawerChartAlerts,
  isDrawerAvailabilityHealthy,
  withoutDrawerExceptionRows,
} from './drawerOverviewModel';
import type { NestedWorkloadContext } from './nestedWorkloadContext';
import { WORKLOAD_ACTION_AGENT_LABEL } from './workloadAgentReadiness';
import { getWorkloadAlertResourceIdCandidates } from './workloadTopology';
import { RaidCard } from '@/components/shared/cards/RaidCard';

import type { GuestDrawerProps } from './guestDrawerModel';

// Linked physical disks render only inside an open agent guest's drawer. Load
// them on demand so the disk presentation module stays out of the
// WorkloadsSurface table chunk.
const GuestPhysicalDisks = lazy(() =>
  import('./GuestPhysicalDisks').then(({ GuestPhysicalDisks }) => ({
    default: GuestPhysicalDisks,
  })),
);

// The charts read metrics history only once a drawer is open; keep their code
// out of the WorkloadsSurface table chunk the same way.
const GuestDrawerHistory = lazy(() =>
  import('./GuestDrawerHistory').then(({ GuestDrawerHistory }) => ({
    default: GuestDrawerHistory,
  })),
);

const GUEST_DRAWER_CHARTED_METRICS = GUEST_DRAWER_HISTORY_GROUPS.flatMap((group) =>
  group.series.map((series) => series.metric),
);

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
  backupPresentation: GuestDrawerBackupPresentation | null;
  diskThresholds?: MetricDisplayThresholds | null;
  workloadActionAgentTitle: string;
  parentMemoryTotal?: GuestDrawerProps['parentMemoryTotal'];
  memoryDisplayBasis?: GuestDrawerProps['memoryDisplayBasis'];
  alerts?: Alert[];
  historyTarget?: GuestDrawerHistoryTarget | null;
  historyRange?: HistoryTimeRange;
  onHistoryRangeChange?: (range: HistoryTimeRange) => void;
  /** True while Overview is the visible tab; hidden charts do not poll. */
  historyEnabled?: boolean;
  currentMetrics?: Record<string, number | undefined>;
  deferredMetrics?: Record<string, GuestDrawerHistoryDeferredMetric | undefined>;
}

export function GuestDrawerOverview(props: GuestDrawerOverviewProps) {
  const now = useRelativeTimeNow();
  const protectionPresentation = () =>
    getWorkloadsGuestProtectionPresentation({
      ageLabel: props.backupPresentation?.ageLabel,
      ageClass: props.backupPresentation?.ageClass,
    });
  const guestReadPresentation = () => getGuestDrawerGuestReadPresentation(props.guest);
  const guestReadRow = () => {
    const read = guestReadPresentation();
    if (!read) return null;
    return makeDetailRow('Guest-agent reads', read.label, {
      title: read.detail,
      tone: read.tone,
      wrap: true,
      // A title alone is not available to touch users. Deferred reads already
      // have a visible safety notice above the drawer; do not repeat it here.
      layout: read.precaution ? undefined : 'stacked',
      valueContent: (
        <>
          <span class="block whitespace-normal wrap-break-word text-left leading-snug">
            {read.label}
          </span>
          <Show when={!read.precaution}>
            <span class="mt-1 block whitespace-normal text-left font-normal leading-snug text-muted wrap-break-word">
              {read.detail}
            </span>
          </Show>
        </>
      ),
    });
  };
  const memoryReading = () => getGuestDrawerMemoryReading(props.guest);
  // A linked Pulse agent past its reporting lease leaves its last report on
  // the guest, so its RAID, network and filesystem rows read as last known.
  const agentLastKnownReason = () =>
    props.guest.agentStale ? HOST_AGENT_STOPPED_REPORTING_REASON : undefined;
  const networkLastKnownReason = () =>
    props.guest.networkInterfacesFromAgent === false ? undefined : agentLastKnownReason();
  const coverageLabel = () => {
    if (props.hasWorkloadActionAgent) return WORKLOAD_ACTION_AGENT_LABEL;
    if (props.hasAgentInfo) return `${props.agentHeading} observed`;
    if (props.showInGuestAgentInstallCue) return 'Agent recommended';
    return null;
  };
  const overviewSections = () =>
    compactDetailSections([
      {
        label: 'Summary',
        rows: compactDetailRows([
          // OS and primary IP lead the facts strip; they are not repeated here.
          makeDetailRow('Pulse coverage', coverageLabel(), {
            title: props.hasWorkloadActionAgent
              ? props.workloadActionAgentTitle
              : props.hasAgentInfo
                ? props.agentTitle
                : undefined,
            wrap: true,
          }),
          guestReadRow(),
          makeDetailRow('Memory reading', memoryReading()?.summary, {
            wrap: true,
          }),
          // Visible text, not a tooltip: the caveat and what to do about it
          // must reach touch and keyboard users too.
          makeDetailRow('About this reading', memoryReading()?.cacheNote, {
            layout: 'stacked',
            wrap: true,
          }),
          // Starting or ending an operation is not evidence of a completed
          // backup. Keep its activity separate from the last completed age.
          makeDetailRow('Last completed backup', protectionPresentation().label, {
            tone: protectionPresentation().tone,
            layout: 'stacked',
            wrap: true,
            valueContent: (
              <>
                <span class="block whitespace-normal wrap-break-word text-left leading-snug">
                  {protectionPresentation().label}
                </span>
                <Show when={props.backupPresentation?.completedAt}>
                  <time
                    class="mt-1 block whitespace-normal wrap-break-word text-left font-normal leading-snug text-muted"
                    dateTime={props.backupPresentation?.completedAt}
                  >
                    {props.backupPresentation?.dateLabel}
                  </time>
                </Show>
              </>
            ),
          }),
          makeDetailRow(
            'Backup activity',
            props.guest.backupInProgress ? 'Running · not completed yet' : null,
            { tone: 'warning', wrap: true },
          ),
        ]),
      },
    ]);

  const technicalSections = () => {
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
    // Rows the linked Pulse agent supplied follow that agent's freshness, the
    // same rule the workload table's disk cell and History read.
    const diskRead = getWorkloadGuestDiskRead(props.guest, isGuestDrawerVM(props.guest));
    const diskReadState =
      !props.guest.disksFromAgent && props.guest.telemetryAvailability?.disk === false
        ? 'unavailable'
        : diskRead.state;
    const diskRows = (props.guest.disks ?? []).map((disk, index) =>
      buildWorkloadsDiskPresentation(disk, index, props.diskThresholds, diskReadState),
    );
    const diskStatusMessage =
      diskRead.message ??
      (diskReadState === 'unavailable' ? 'Filesystem usage is unavailable.' : null);

    return compactDetailSections([
      {
        label: 'System',
        rows: compactDetailRows([
          props.guest.cpus ? makeDetailRow('CPUs', `${props.guest.cpus}`) : null,
          props.hasAgentInfo
            ? agentLastKnownReason()
              ? makeDetailRow(props.agentHeading, `${props.agentLabel} · stopped reporting`, {
                  title: `${props.agentTitle} has stopped reporting. Its readings here are its last report.`,
                  tone: 'warning',
                  wrap: true,
                })
              : makeDetailRow(props.agentHeading, props.agentLabel, { title: props.agentTitle })
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
      (props.hasFilesystemDetails && diskRows.length > 0) || diskStatusMessage
        ? {
            label: 'Filesystems',
            footerContent:
              diskStatusMessage && isGuestDrawerVM(props.guest) ? (
                <a
                  class="inline-flex min-h-11 items-center rounded-sm text-xs font-semibold text-base-content underline underline-offset-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-base-content sm:min-h-0"
                  href={`${getShippedDocUrl('VM_DISK_MONITORING.md')}#a-missing-reading-is-not-an-installation-diagnosis`}
                >
                  Filesystem reading guidance
                </a>
              ) : undefined,
            rows: compactDetailRows([
              makeDetailRow('Status', diskStatusMessage, { layout: 'stacked', wrap: true }),
              ...diskRows.map((disk) =>
                makeDetailRow(
                  disk.label,
                  [disk.usagePercentLabel, disk.usageText, disk.typeLabel]
                    .filter(Boolean)
                    .join(' · '),
                  {
                    title: disk.labelTitle,
                    layout: 'stacked',
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
            ]),
          }
        : null,
      props.hasNetworkInterfaces
        ? {
            label: 'Network',
            rows: compactDetailRows([
              // Agent freshness applies only to the selected agent list,
              // not interfaces supplied independently by Proxmox.
              makeDetailRow(
                'Status',
                networkLastKnownReason()
                  ? 'Using last known interface details. The Pulse Agent in this guest stopped reporting.'
                  : null,
                { layout: 'stacked', wrap: true },
              ),
              ...props.networkInterfaces.slice(0, 4).map((iface, index) => {
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
                    tone: networkLastKnownReason() ? 'muted' : undefined,
                    wrap: true,
                  },
                );
              }),
            ]),
          }
        : null,
    ]);
  };

  const sectionsByLabel = () => {
    const technical = technicalSections();
    const filesystems = technical.filter((section) => section.label === 'Filesystems');
    const rest = technical.filter((section) => section.label !== 'Filesystems');
    return {
      filesystems,
      details: withoutDrawerExceptionRows([...overviewSections(), ...rest]),
    };
  };
  const exceptionRows = createMemo(() =>
    collectDrawerExceptionRows([...overviewSections(), ...technicalSections()]),
  );

  const availabilityChecks = () => [
    ...(props.guest.availability ? [props.guest.availability] : []),
    ...(props.guest.availabilityChecks ?? []),
  ];
  const hasAvailability = () => availabilityChecks().length > 0;
  const availabilityHealthy = () =>
    hasAvailability() &&
    availabilityChecks().every((check) => isDrawerAvailabilityHealthy(check, now()));

  // The amounts the charts' percentages are of. A withdrawn memory reading
  // shows no bytes either, the same decision as its percentage.
  const memoryFact = () => {
    const memory = props.guest.memory;
    const hasReading =
      typeof props.currentMetrics?.memory === 'number' ||
      typeof props.deferredMetrics?.memory?.lastKnownValue === 'number';
    if (!memory || !hasReading || memory.usageUnavailable || !((memory.total ?? 0) > 0)) {
      return '';
    }
    // A retained or undated reading is qualified in the chart; here it states
    // the size only, so no used amount reads as current.
    if (props.deferredMetrics?.memory) return `${formatBytes(memory.total ?? 0)} RAM`;
    // The short qualifier sits on the amount it qualifies; the full note (why,
    // and what to do) is the Details row "About this reading".
    const size = `${formatBytes(memory.used ?? 0)} of ${formatBytes(memory.total ?? 0)} RAM`;
    return memoryReading()?.mayIncludeCache ? `${size} (may include cache)` : size;
  };
  const diskFact = () => {
    const disk = props.guest.disk;
    if (
      !disk ||
      !((disk.total ?? 0) > 0) ||
      typeof disk.used !== 'number' ||
      !Number.isFinite(disk.used) ||
      disk.used < 0 ||
      typeof props.currentMetrics?.disk !== 'number'
    ) {
      return '';
    }
    return `${formatBytes(disk.used ?? 0)} of ${formatBytes(disk.total ?? 0)} disk`;
  };
  const chartAlerts = createMemo(() =>
    getDrawerChartAlerts(props.alerts, GUEST_DRAWER_CHARTED_METRICS),
  );

  const facts = () =>
    [
      props.hasOsInfo ? props.guestOsSummary : '',
      props.guest.cpus ? `${props.guest.cpus} vCPU` : '',
      memoryFact(),
      diskFact(),
      props.ipAddresses[0] ?? '',
      // A late or missing backup is listed under Worth checking instead.
      protectionPresentation().tone === 'success'
        ? `Last completed backup ${protectionPresentation().label}`
        : '',
    ].filter((fact) => fact.trim().length > 0);

  // A tag that repeats a fact already on the strip (often the IP) adds nothing.
  const tagChips = () => props.normalizedTags.filter((tag) => !facts().includes(tag));

  return (
    <div class="space-y-3" data-testid="guest-drawer-overview">
      <DrawerAttentionSection
        items={(props.alerts ?? []).map((alert) => ({
          id: alert.id,
          ...getGuestDrawerAlertAttention(
            alert,
            {
              guest: props.guest,
              memoryDisplayBasis: props.memoryDisplayBasis,
              parentMemoryTotal: props.parentMemoryTotal,
            },
            now,
          ),
          severity: alert.level,
          acknowledged: alert.acknowledged,
        }))}
      />
      <Show when={(props.alerts ?? []).length === 0}>
        <p data-testid="drawer-no-attention" class="text-xs text-muted">
          <span class="font-medium text-base-content">{ALERTS_EMPTY_STATE}</span>
          <Show when={availabilityHealthy()}>
            <span> · availability checks passing</span>
          </Show>
        </p>
      </Show>

      <DrawerExceptionList testId="guest-drawer-exceptions" rows={exceptionRows()} />

      <Show when={props.historyTarget}>
        <Suspense fallback={<div class="h-48" aria-hidden="true" />}>
          <GuestDrawerHistory
            target={props.historyTarget ?? null}
            range={props.historyRange ?? GUEST_DRAWER_HISTORY_DEFAULT_RANGE}
            onRangeChange={props.onHistoryRangeChange}
            enabled={props.historyEnabled ?? true}
            currentMetrics={props.currentMetrics}
            deferredMetrics={props.deferredMetrics}
            alerts={chartAlerts()}
          />
        </Suspense>
      </Show>

      <Show when={sectionsByLabel().filesystems.length > 0}>
        <TechnicalDetailsSection
          dataTestId="guest-drawer-filesystems"
          sections={sectionsByLabel().filesystems}
        />
      </Show>
      <Show
        when={
          props.guest.agentKind === 'pulse' ||
          props.guest.agentVersion ||
          props.guest.agentRaid?.length
        }
      >
        <Suspense fallback={null}>
          <GuestPhysicalDisks
            parentId={props.guest.canonicalResourceId ?? props.guest.id}
            alertResourceIds={[
              ...(props.guest.agentId ? [props.guest.agentId] : []),
              ...(props.guest.canonicalResourceId ? [props.guest.canonicalResourceId] : []),
              ...getWorkloadAlertResourceIdCandidates(props.guest),
            ]}
            historyEnabled={props.historyEnabled ?? true}
          />
        </Suspense>
      </Show>
      <Show when={props.guest.agentRaid?.length}>
        <RaidCard
          arrays={props.guest.agentRaid}
          title="Guest RAID"
          lastKnownReason={agentLastKnownReason()}
        />
      </Show>

      {/* A failing or unknown check is something to act on, so it stays in
          front. A passing one is summarised above and kept under Details. */}
      <Show when={hasAvailability() && !availabilityHealthy()}>
        <div class="max-w-sm space-y-3">
          <AvailabilityProbeStatusCards
            availability={props.guest.availability}
            checks={props.guest.availabilityChecks}
          />
        </div>
      </Show>

      <Show when={facts().length > 0 || tagChips().length > 0}>
        <div
          data-testid="guest-drawer-facts"
          class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted"
        >
          <For each={facts()}>
            {(fact) => <span class="max-w-full whitespace-normal wrap-anywhere">{fact}</span>}
          </For>
          <For each={tagChips()}>
            {(tag) => (
              <span class="max-w-full rounded-full border border-border px-2 py-0.5 text-[11px] break-all">
                {tag}
              </span>
            )}
          </For>
        </div>
      </Show>

      <TechnicalDetailsDisclosure
        dataTestId="guest-technical-details"
        title="Details"
        keepMounted
        subtitle="system, coverage, memory, network and availability checks"
        class="border-t border-border pt-2"
        contentClass="mt-2 space-y-3"
      >
        <TechnicalDetailsSection
          dataTestId="guest-technical-details-sections"
          sections={sectionsByLabel().details}
        />
        <Show when={availabilityHealthy()}>
          <div class="max-w-sm space-y-3">
            <AvailabilityProbeStatusCards
              availability={props.guest.availability}
              checks={props.guest.availabilityChecks}
            />
          </div>
        </Show>
      </TechnicalDetailsDisclosure>
    </div>
  );
}
