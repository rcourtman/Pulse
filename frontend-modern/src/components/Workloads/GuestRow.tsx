import { createMemo, createSignal, Show, splitProps, type JSX } from 'solid-js';
import BoxIcon from 'lucide-solid/icons/box';
import type { VM } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { formatUptime, formatSpeed, getShortImageName } from '@/utils/format';
import { StackedDiskBar } from './StackedDiskBar';
import { StackedMemoryBar } from './StackedMemoryBar';
import { WorkloadReadingStatus } from './WorkloadReadingStatus';
import {
  AvailabilityProbeCell,
  BackupIndicator,
  BackupStatusCell,
  InfoTooltipCell,
  NetworkInfoCell,
  OSInfoCell,
} from './GuestRowCells';

import { StatusDot } from '@/components/shared/StatusDot';
import { recordWorkloadHistoryActivity } from '@/api/workloadHistoryActivity';
import { TagBadges } from '@/components/shared/TagBadges';
import { WorkloadTypeBadge } from '@/components/shared/WorkloadTypeBadge';
import { getWorkloadCPUPercent, resolveWorkloadType } from '@/utils/workloads';
import { EnhancedCPUBar } from '@/components/Workloads/EnhancedCPUBar';
import {
  MetricMiniSparkline,
  MetricMiniSparklineRatePair,
  type MetricMiniSparklineValueLabelContext,
} from '@/components/Workloads/MetricMiniSparkline';
import { UpdateButton } from '@/components/shared/ContainerUpdateBadge';
import {
  buildSummaryDisclosureControlsId,
  createSummaryInteractiveRowPreviewHandlers,
} from '@/components/shared/summaryInteractionA11y';
import { SummaryRowActionButton } from '@/components/shared/SummaryRowActionButton';
import { createRowTextSelectionGuard, nativeRowClickTarget } from '@/components/shared/Table';
import { getWorkloadGuestDiskStatusMessage } from '@/utils/workloadGuestPresentation';
import { getMetricSeverity } from '@/utils/metricThresholds';
import {
  getWorkloadMemoryGuestKind,
  getWorkloadMemoryObservationPresentation,
} from '@/utils/memoryObservation';
import { ResourceNameWithWebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import type { GuestRowProps } from './guestRowModel';
import { useGuestRowState } from './useGuestRowState';
import type {
  WorkloadMetricHistoryReader,
  WorkloadRateMetric,
  WorkloadTableMetric,
} from './workloadMetricHistoryModel';

type Guest = WorkloadGuest;

// Type guard for VM vs Container
const isVM = (guest: Guest): guest is VM => {
  return resolveWorkloadType(guest) === 'vm';
};

export { GUEST_COLUMNS, VIEW_MODE_COLUMNS } from './guestRowModel';
export type { GuestRowProps, WorkloadIOEmphasis } from './guestRowModel';
import { getGuestColumnStyle } from './guestRowModel';

export function GuestRow(props: GuestRowProps) {
  const [rowActionProps] = splitProps(props, ['onClick']);
  const selectionGuard = createRowTextSelectionGuard();
  const runRowAction = (event: MouseEvent) => {
    if (!selectionGuard.isSelectionClick(event)) rowActionProps.onClick?.();
  };
  const rowAction = {
    get onClick() {
      return rowActionProps.onClick ? runRowAction : undefined;
    },
  };
  const {
    agentVersion,
    appContainerRuntimeBadge,
    clusterName,
    contextLabel,
    cpuAnomaly,
    cpuThresholds,
    customUrl,
    diskAnomaly,
    diskIOEmphasis,
    diskPercent,
    diskRead,
    diskThresholds,
    diskUsageRead,
    diskWrite,
    displayId,
    dockerHostId,
    dockerImage,
    firstCellIndent,
    guestId,
    guestStatus,
    hasDiskUsage,
    hasNetworkInterfaces,
    hasOsInfo,
    infoTooltip,
    infoValue,
    ipAddresses,
    isColVisible,
    isMobile,
    isPveWorkload,
    isRunning,
    lockLabel,
    memoryAnomaly,
    memoryPercentOnly,
    memoryThresholds,
    memoryTooltip,
    metricsKey,
    namespace,
    networkEmphasis,
    networkIn,
    networkInterfaces,
    networkOut,
    ociImage,
    osName,
    osVersion,
    alertAccentTone,
    rowClass,
    supportsBackup,
    typeInfo,
    workloadType,
    availabilityPresentation,
  } = useGuestRowState(props);

  const cpuPercent = createMemo(() => getWorkloadCPUPercent(props.guest.cpu) ?? 0);
  const metricDisplayMode = createMemo(() => props.metricDisplayMode ?? 'bars');
  const isSparklineMode = createMemo(() => metricDisplayMode() === 'sparklines');
  const rowHistoryPreviewEnabled = createMemo(
    () => isSparklineMode() || (props.metricHoverMode ?? 'history') === 'history',
  );
  const [historyCursorRatio, setHistoryCursorRatio] = createSignal<number | null>(null);
  const setTrackedHistoryCursorRatio = (ratio: number | null): void => {
    setHistoryCursorRatio(ratio);
    if (ratio !== null) {
      recordWorkloadHistoryActivity('scrub');
    }
  };
  const [historyLensActive, setHistoryLensActive] = createSignal(false);
  const historyLensHasData = createMemo(() => {
    if (!historyLensActive() || !props.metricHistory) return false;
    if (props.metricHistory.hasGuestHistory) {
      return props.metricHistory.hasGuestHistory(props.guest);
    }
    return (['cpu', 'memory', 'disk', 'netIo', 'diskIo'] as const).some((metric) =>
      props
        .metricHistory!.getGuestMetricSeries(props.guest, metric)
        .some((series) => series.points.length > 0),
    );
  });
  const historyLensVisible = createMemo(
    () =>
      !isSparklineMode() &&
      rowHistoryPreviewEnabled() &&
      historyLensActive() &&
      historyLensHasData(),
  );
  const usesCompactTableLayout = createMemo(
    () =>
      props.workloadTableLayoutMode === 'narrow' ||
      props.workloadTableLayoutMode === 'phone' ||
      props.workloadTableLayoutMode === 'mobile',
  );
  const usesCondensedIdentity = createMemo(
    () => props.workloadTableLayoutMode === 'narrow' || props.workloadTableLayoutMode === 'phone',
  );
  const telemetryAvailable = (
    metric: keyof NonNullable<WorkloadGuest['telemetryAvailability']>,
  ): boolean => props.guest.telemetryAvailability?.[metric] ?? true;
  const isHostMemoryBasis = createMemo(() => props.memoryDisplayBasis === 'host');
  const hostMemoryTotal = createMemo(() => {
    const total = props.parentMemoryTotal;
    return typeof total === 'number' && Number.isFinite(total) && total > 0 ? total : 0;
  });
  const memoryDisplayTotal = createMemo(() =>
    isHostMemoryBasis() ? hostMemoryTotal() : (props.guest.memory?.total ?? 0),
  );
  const memoryReading = createMemo(() =>
    getWorkloadMemoryObservationPresentation(props.guest, { includeCurrent: false }),
  );
  const memoryDisplayUnavailable = createMemo(
    () =>
      !telemetryAvailable('memory') ||
      props.guest.memory?.usageUnavailable === true ||
      memoryReading()?.state === 'unavailable' ||
      (isHostMemoryBasis() && hostMemoryTotal() <= 0),
  );
  // The guest's own memory severity, even when the bar shows its host share:
  // the bar is coloured by the same figure.
  const guestMemoryAtWarning = createMemo(() => {
    const memory = props.guest.memory;
    const percent =
      typeof memory?.used === 'number' && typeof memory.total === 'number' && memory.total > 0
        ? (memory.used / memory.total) * 100
        : memory?.usage;
    if (typeof percent !== 'number' || !Number.isFinite(percent)) return false;
    return getMetricSeverity(percent, 'memory', memoryThresholds()) !== 'normal';
  });
  const memoryReadStatus = createMemo(() => {
    const reading = memoryReading();
    if (!reading) return undefined;
    // With no number shown for another reason (memory telemetry off, no host
    // total for a host share), there is no reading to qualify.
    if (memoryDisplayUnavailable() && reading.state !== 'unavailable') return undefined;
    // A current reading needs no qualification, except a VM's cache-inclusive
    // one: the guest agent can supply the VM's own figure. Containers have no
    // such step, so their caveat stays in the tooltip and drawer.
    const cacheReading =
      reading.state === 'current' &&
      reading.mayIncludeCache &&
      getWorkloadMemoryGuestKind(props.guest) === 'vm';
    if (reading.state === 'current' && !cacheReading) return undefined;
    return {
      message: reading.message,
      // Visible only where it changes what the user concludes: a cache
      // reading that looks like memory pressure, or a number that is not
      // current. An unavailable reading already shows no number, so a label
      // would only repeat it. Every reason stays in the title, assistive text
      // and drawer.
      label: cacheReading
        ? guestMemoryAtWarning()
          ? usesCompactTableLayout()
            ? 'Cache?'
            : 'May include cache'
          : null
        : reading.state === 'last-known'
          ? usesCompactTableLayout()
            ? 'Prior'
            : 'Last known'
          : reading.state === 'unavailable'
            ? null
            : usesCompactTableLayout()
              ? 'Unknown'
              : 'Freshness unknown',
      valueLabelContext: cacheReading
        ? ('current' as const)
        : reading.state === 'last-known'
          ? ('last known' as const)
          : reading.state === 'unavailable'
            ? ('unavailable' as const)
            : ('freshness unknown' as const),
    };
  });
  // The cell title carries a reading's reason; the dash explains only what
  // the cell does not.
  const memoryUnavailableTooltip = () =>
    memoryReadStatus()
      ? undefined
      : isHostMemoryBasis() && hostMemoryTotal() <= 0
        ? 'Host memory total unavailable'
        : 'Memory usage unavailable';
  const memoryValueLabelContext = (): MetricMiniSparklineValueLabelContext =>
    memoryDisplayUnavailable()
      ? 'unavailable'
      : (memoryReadStatus()?.valueLabelContext ?? 'current');
  const detailControlsId = createMemo(() => buildSummaryDisclosureControlsId(guestId()));
  const nestedWorkloadCueLabel = createMemo(() => {
    const context = props.nestedWorkloadContext;
    if (!context || context.count <= 0) return '';
    const noun = context.count === 1 ? 'container' : 'containers';
    return `${context.count} nested ${context.label} ${noun}. Open row for details.`;
  });
  const clampMetricPercent = (value: number | null | undefined): number => {
    if (typeof value !== 'number' || !Number.isFinite(value)) return 0;
    return Math.max(0, Math.min(100, value));
  };
  const formatMetricPercent = (value: number | null | undefined): string =>
    `${Math.round(clampMetricPercent(value))}%`;
  const usagePercent = (
    used?: number | null,
    total?: number | null,
    fallbackUsage?: number | null,
  ): number => {
    if (typeof used === 'number' && typeof total === 'number' && total > 0) {
      return Math.max(0, Math.min(100, (used / total) * 100));
    }
    return clampMetricPercent(fallbackUsage);
  };
  // Every input is an accessor so a live snapshot updates the mounted chart in
  // place. Reading signals while building the chart would remount it on each
  // update and drop an open scrub tooltip.
  const renderMetricSparkline = (
    metric: WorkloadTableMetric,
    options: {
      valueLabel: () => string;
      title: () => string;
      tooltip?: () => string | undefined;
      unit?: string;
      formatValue: (value: number) => string;
      seriesOptions?: () => Parameters<WorkloadMetricHistoryReader['getGuestMetricSeries']>[2];
      valueLabelContext?: () => MetricMiniSparklineValueLabelContext;
      valueContent?: () => JSX.Element;
    },
  ) => (
    <MetricMiniSparkline
      series={
        props.metricHistory?.getGuestMetricSeries(props.guest, metric, options.seriesOptions?.()) ??
        []
      }
      valueLabel={options.valueLabel()}
      valueContent={options.valueContent?.()}
      valueLabelContext={options.valueLabelContext?.()}
      valueLabelMode="inline"
      title={options.title()}
      tooltip={options.tooltip?.()}
      unit={options.unit ?? '%'}
      formatValue={options.formatValue}
      cursorRatio={historyCursorRatio()}
      onCursorRatioChange={setTrackedHistoryCursorRatio}
      showTooltip={true}
    />
  );
  const renderCpuSparkline = () =>
    renderMetricSparkline('cpu', {
      valueLabel: () => formatMetricPercent(cpuPercent()),
      title: () => `${props.guest.name} CPU history`,
      formatValue: formatMetricPercent,
    });
  const renderMemorySparkline = () =>
    renderMetricSparkline('memory', {
      valueLabel: () =>
        memoryDisplayUnavailable()
          ? 'N/A'
          : formatMetricPercent(
              usagePercent(
                props.guest.memory?.used,
                memoryDisplayTotal(),
                isHostMemoryBasis() ? undefined : props.guest.memory?.usage,
              ),
            ),
      title: () =>
        isHostMemoryBasis()
          ? `${props.guest.name} host memory share history`
          : `${props.guest.name} memory history`,
      // The chart covers the cell, so it carries the reading's reason on hover.
      tooltip: () =>
        memoryReadStatus()?.message ??
        (memoryDisplayUnavailable() ? memoryUnavailableTooltip() : undefined),
      formatValue: formatMetricPercent,
      seriesOptions: () => ({
        memoryDisplayBasis: props.memoryDisplayBasis,
        parentMemoryTotal: props.parentMemoryTotal,
      }),
      valueLabelContext: () => memoryValueLabelContext(),
    });
  const renderDiskSparkline = () =>
    renderMetricSparkline('disk', {
      valueLabel: () => (hasDiskUsage() ? formatMetricPercent(diskPercent()) : '—'),
      title: () => `${props.guest.name} disk usage history`,
      tooltip: () => diskReadStatus()?.message,
      formatValue: formatMetricPercent,
      valueLabelContext: () => diskValueLabelContext(),
    });
  const renderRateSparkline = (metric: WorkloadRateMetric) => {
    const isNetwork = metric === 'netIo';
    const live = () => isRunning() && telemetryAvailable(isNetwork ? 'networkIO' : 'diskIO');
    const values = (): [number, number] =>
      isNetwork ? [networkIn(), networkOut()] : [diskRead(), diskWrite()];
    return renderMetricSparkline(metric, {
      valueLabel: () =>
        live() ? `${formatSpeed(values()[0])} / ${formatSpeed(values()[1])}` : '—',
      title: () => `${props.guest.name} ${isNetwork ? 'network' : 'disk'} I/O history`,
      unit: 'B/s',
      formatValue: formatSpeed,
      valueContent: () =>
        live() ? <MetricMiniSparklineRatePair metric={metric} values={values()} /> : undefined,
    });
  };

  const getDiskStatusTooltip = () => {
    const message = diskUsageRead().message;
    if (message) return message;
    if (!isVM(props.guest) || props.guest.disksFromAgent) return 'Disk stats unavailable';

    const vm = props.guest as VM;
    return getWorkloadGuestDiskStatusMessage(vm.diskStatusReason);
  };
  // A native title or bar tooltip is not a visible freshness cue on touch,
  // and disappears from the chart's accessible value when the history lens opens.
  // A retained number and a broken read the user can fix get a visible label.
  // Other unavailable reads (VM stopped, no guest agent) already show a dash,
  // so their reason stays in the title, assistive text and drawer.
  const diskReadStatus = createMemo(() => {
    const read = diskUsageRead();
    if (read.state === 'current') return undefined;
    const retained = read.state === 'last-known';
    const reason = isVM(props.guest)
      ? (props.guest.diskStatusReason ?? '').replace(/^prev-/, '')
      : '';
    return {
      label: retained
        ? usesCompactTableLayout()
          ? 'Prior'
          : 'Last known'
        : !read.needsAction
          ? null
          : reason === 'permission-denied'
            ? usesCompactTableLayout()
              ? 'Denied'
              : 'No permission'
            : usesCompactTableLayout()
              ? 'Error'
              : 'Agent error',
      message: read.message,
      valueLabelContext: retained ? ('last known' as const) : ('current' as const),
      // Most reasons are a setup gap (no guest agent, VM stopped) rather than a
      // fault, so they stay muted; only a broken read the user can fix is amber.
      needsAction: read.needsAction,
    };
  });
  const diskValueLabelContext = (): MetricMiniSparklineValueLabelContext =>
    diskReadStatus()?.valueLabelContext ?? 'current';
  const interactiveRowHandlers = createSummaryInteractiveRowPreviewHandlers({
    onPreview: () => {
      if (!rowHistoryPreviewEnabled()) return;
      setHistoryLensActive(true);
      props.onHoverChange?.(guestId());
    },
    onPreviewClear: () => {
      setHistoryLensActive(false);
      setHistoryCursorRatio(null);
      if (!rowHistoryPreviewEnabled()) return;
      props.onHoverChange?.(null);
    },
  });

  return (
    <>
      <tr
        class={`${rowClass()} workload-row ${props.onClick ? 'cursor-pointer group' : ''}`.trim()}
        data-guest-id={guestId()}
        data-history-lens-active={historyLensActive() ? 'true' : undefined}
        data-history-lens-pending={
          historyLensActive() && !historyLensVisible() ? 'true' : undefined
        }
        data-workload-alert-accent={alertAccentTone()}
        data-summary-series-id={guestId()}
        data-summary-row-active={props.isSummaryHighlighted && !props.isExpanded ? 'true' : 'false'}
        on:click={rowActionProps.onClick ? nativeRowClickTarget : undefined}
        on:mousedown={rowActionProps.onClick ? selectionGuard.onMouseDown : undefined}
        {...rowAction}
        {...interactiveRowHandlers}
      >
        {/* Name - always visible */}
        <td
          class={`pr-1.5 sm:pr-2 py-0.5 align-middle whitespace-nowrap ${firstCellIndent()}`}
          data-workload-col="name"
          style={getGuestColumnStyle(
            'name',
            isMobile(),
            props.workloadTableLayoutMode,
            props.visibleColumnIds,
            props.columnWidths,
          )}
        >
          <div
            class={`flex items-center min-w-0 ${usesCompactTableLayout() ? 'gap-0.5' : 'gap-2'}`}
          >
            <Show when={props.onClick}>
              <SummaryRowActionButton
                kind="disclosure"
                subjectLabel={props.guest.name}
                expanded={props.isExpanded === true}
                controlsId={detailControlsId()}
                hideWhenRowTappableOnMobile={usesCompactTableLayout()}
                onAction={() => props.onClick?.()}
                onPreviewClear={() => props.onHoverChange?.(null)}
              />
            </Show>
            <div
              class={`min-w-0 ${usesCondensedIdentity() ? 'flex flex-1 flex-col items-start gap-0.5' : 'flex items-center gap-2'}`}
            >
              <div
                class={`flex items-center gap-1.5 min-w-0 ${usesCondensedIdentity() ? 'w-full' : ''}`}
              >
                <StatusDot
                  variant={guestStatus().variant}
                  title={guestStatus().label}
                  ariaLabel={guestStatus().label}
                  size="xs"
                  class={usesCondensedIdentity() ? 'self-start mt-1.5' : undefined}
                />
                <div
                  class={`min-w-0 group/name ${usesCondensedIdentity() ? 'flex flex-1 flex-col items-start gap-0.5' : 'flex items-center gap-1.5'}`}
                >
                  <ResourceNameWithWebInterfaceLink
                    name={props.guest.name}
                    url={customUrl()}
                    class={`min-w-0 ${usesCondensedIdentity() ? 'w-full [&>a]:hidden' : ''}`}
                    nameClass="truncate text-[11px] font-medium text-base-content select-none"
                  />
                  {/* Show backup indicator in name cell only if backup column is hidden */}
                  <Show when={!isColVisible('backup') && supportsBackup()}>
                    <BackupIndicator
                      guestName={props.guest.name}
                      lastBackup={props.guest.lastBackup}
                      isTemplate={props.guest.template}
                      backupRunning={props.guest.backupInProgress}
                      guestReadEvidence={props.guest}
                    />
                  </Show>
                  <Show when={nestedWorkloadCueLabel()}>
                    <span
                      class="inline-flex shrink-0 items-center gap-0.5 text-[10px] font-medium leading-none text-muted"
                      data-testid="nested-workload-cue"
                      title={nestedWorkloadCueLabel()}
                      aria-label={nestedWorkloadCueLabel()}
                    >
                      <BoxIcon class="h-3 w-3" aria-hidden="true" />
                      <span class="tabular-nums">{props.nestedWorkloadContext?.count}</span>
                    </span>
                  </Show>
                </div>
              </div>

              <Show when={lockLabel()}>
                <span
                  class={`max-w-full text-[10px] font-medium text-muted uppercase tracking-wide ${usesCondensedIdentity() ? 'whitespace-normal break-words' : 'whitespace-nowrap'}`}
                  title={`Guest is locked (${lockLabel()})`}
                >
                  Lock: {lockLabel()}
                </span>
              </Show>
            </div>
          </div>
        </td>

        {/* Availability */}
        <Show when={isColVisible('availability')}>
          <td
            class="px-1.5 sm:px-2 py-0.5 align-middle text-center"
            data-workload-col="availability"
          >
            <Show when={availabilityPresentation()}>
              {(ap) => <AvailabilityProbeCell presentation={ap()} />}
            </Show>
          </td>
        </Show>

        {/* Runtime */}
        <Show when={isColVisible('runtime')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle" data-workload-col="runtime">
            <div class="flex justify-center">
              <Show
                when={appContainerRuntimeBadge()}
                fallback={
                  <span class="text-xs text-slate-400" aria-hidden="true">
                    —
                  </span>
                }
              >
                {(badge) => (
                  <span class={badge().classes} title={badge().title}>
                    {badge().label}
                  </span>
                )}
              </Show>
            </div>
          </td>
        </Show>

        {/* Type */}
        <Show when={isColVisible('type')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <WorkloadTypeBadge
                type={workloadType()}
                label={typeInfo().label}
                title={typeInfo().title}
              />
            </div>
          </td>
        </Show>

        {/* Info - merged identifier (VMID / image / namespace) for mixed-type views */}
        <Show when={isColVisible('info')}>
          <td class="px-0.5 py-0.5 align-middle text-center sm:px-2" data-workload-col="info">
            <div class="text-xs text-muted whitespace-nowrap">
              <Show
                when={infoValue()}
                fallback={
                  <span class="" aria-hidden="true">
                    —
                  </span>
                }
              >
                <InfoTooltipCell
                  value={infoValue()}
                  tooltip={infoTooltip()}
                  type={workloadType()}
                />
              </Show>
            </div>
          </td>
        </Show>

        {/* VMID */}
        <Show when={isColVisible('vmid')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle text-center" data-workload-col="vmid">
            <div class="text-xs text-muted tabular-nums whitespace-nowrap">
              <Show
                when={displayId()}
                fallback={
                  <span class="" aria-hidden="true">
                    —
                  </span>
                }
              >
                {displayId()}
              </Show>
            </div>
          </td>
        </Show>

        {/* CPU */}
        <Show when={isColVisible('cpu')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle" data-workload-col="cpu">
            <Show
              when={telemetryAvailable('cpu')}
              fallback={<div class="text-center text-xs text-slate-400">—</div>}
            >
              <Show
                when={isSparklineMode()}
                fallback={
                  <div class="h-4">
                    <Show
                      when={historyLensVisible()}
                      fallback={
                        <EnhancedCPUBar
                          usage={cpuPercent()}
                          cores={isMobile() ? undefined : props.guest.cpus || undefined}
                          resourceId={metricsKey()}
                          anomaly={cpuAnomaly()}
                          thresholds={cpuThresholds()}
                        />
                      }
                    >
                      <div class="h-4 animate-in fade-in-0 duration-100 motion-reduce:animate-none">
                        {renderCpuSparkline()}
                      </div>
                    </Show>
                  </div>
                }
              >
                {renderCpuSparkline()}
              </Show>
            </Show>
          </td>
        </Show>

        {/* Memory */}
        <Show when={isColVisible('memory')}>
          <td
            class="px-1.5 sm:px-2 py-0.5 align-middle"
            data-workload-col="memory"
            title={memoryReadStatus()?.message}
          >
            <Show
              when={isSparklineMode()}
              fallback={
                <div class="h-4">
                  <Show
                    when={historyLensVisible()}
                    fallback={
                      <Show
                        when={!memoryDisplayUnavailable()}
                        fallback={
                          <div class="flex justify-center">
                            <span class="text-xs text-slate-400" title={memoryUnavailableTooltip()}>
                              -
                            </span>
                          </div>
                        }
                      >
                        <div
                          title={isHostMemoryBasis() ? undefined : (memoryTooltip() ?? undefined)}
                        >
                          <StackedMemoryBar
                            used={props.guest.memory?.used || 0}
                            total={memoryDisplayTotal()}
                            unavailable={memoryDisplayUnavailable()}
                            reading={memoryReading()}
                            percentOnly={isHostMemoryBasis() ? undefined : memoryPercentOnly()}
                            cache={isHostMemoryBasis() ? 0 : (props.guest.memory?.cache ?? 0)}
                            cacheInclusiveLabel={
                              isHostMemoryBasis() ? undefined : 'Shown in Proxmox'
                            }
                            balloon={isHostMemoryBasis() ? 0 : (props.guest.memory?.balloon ?? 0)}
                            swapUsed={isHostMemoryBasis() ? 0 : (props.guest.memory?.swapUsed ?? 0)}
                            swapTotal={
                              isHostMemoryBasis() ? 0 : (props.guest.memory?.swapTotal ?? 0)
                            }
                            resourceId={metricsKey()}
                            anomaly={isHostMemoryBasis() ? null : memoryAnomaly()}
                            thresholds={memoryThresholds()}
                            severityPercent={
                              isHostMemoryBasis()
                                ? usagePercent(
                                    props.guest.memory?.used,
                                    props.guest.memory?.total,
                                    props.guest.memory?.usage,
                                  )
                                : undefined
                            }
                            comparisonTotalLabel={isHostMemoryBasis() ? 'Host total' : undefined}
                            tooltipTitle={
                              isHostMemoryBasis()
                                ? `${props.parentNodeName?.trim() || 'Host'} memory share`
                                : undefined
                            }
                          />
                        </div>
                      </Show>
                    }
                  >
                    <div class="h-4 animate-in fade-in-0 duration-100 motion-reduce:animate-none">
                      {renderMemorySparkline()}
                    </div>
                  </Show>
                </div>
              }
            >
              {renderMemorySparkline()}
            </Show>
            <Show when={memoryReadStatus()}>
              {(status) => (
                <Show
                  when={status().label}
                  fallback={
                    <span data-workload-memory-read-status class="sr-only">
                      {status().message}
                    </span>
                  }
                >
                  {(label) => (
                    <WorkloadReadingStatus
                      metric="Memory"
                      guestId={guestId()}
                      guestName={props.guest.name}
                      label={label()}
                      message={status().message}
                    />
                  )}
                </Show>
              )}
            </Show>
          </td>
        </Show>

        {/* Disk */}
        <Show when={isColVisible('disk')}>
          <td
            class="px-1.5 sm:px-2 py-0.5 align-middle"
            data-workload-col="disk"
            title={diskReadStatus()?.message}
          >
            <Show when={isSparklineMode()}>{renderDiskSparkline()}</Show>
            <Show when={!isSparklineMode()}>
              <Show
                when={hasDiskUsage()}
                fallback={
                  <div class="flex justify-center">
                    <span class="text-xs text-slate-400" title={getDiskStatusTooltip()}>
                      -
                    </span>
                  </div>
                }
              >
                <div class="h-4">
                  <Show
                    when={historyLensVisible()}
                    fallback={
                      <StackedDiskBar
                        mode={usesCompactTableLayout() ? 'aggregate' : undefined}
                        disks={props.guest.disks}
                        aggregateDisk={props.guest.disk}
                        anomaly={diskAnomaly()}
                        thresholds={diskThresholds()}
                        reading={diskUsageRead()}
                      />
                    }
                  >
                    <div class="h-4 animate-in fade-in-0 duration-100 motion-reduce:animate-none">
                      {renderDiskSparkline()}
                    </div>
                  </Show>
                </div>
              </Show>
            </Show>
            <Show when={diskReadStatus()}>
              {(status) => (
                <Show
                  when={status().label}
                  fallback={
                    <span data-workload-disk-read-status class="sr-only">
                      {status().message}
                    </span>
                  }
                >
                  {(label) => (
                    <WorkloadReadingStatus
                      metric="Disk"
                      guestId={guestId()}
                      guestName={props.guest.name}
                      label={label()}
                      message={status().message}
                      needsAction={status().needsAction}
                    />
                  )}
                </Show>
              )}
            </Show>
          </td>
        </Show>

        {/* IP Address with Network Tooltip */}
        <Show when={isColVisible('ip')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={ipAddresses().length > 0 || hasNetworkInterfaces()}
                fallback={
                  <span class="text-xs text-slate-400" aria-hidden="true">
                    —
                  </span>
                }
              >
                <NetworkInfoCell
                  ipAddresses={ipAddresses()}
                  networkInterfaces={networkInterfaces()}
                />
              </Show>
            </div>
          </td>
        </Show>

        {/* Uptime */}
        <Show when={isColVisible('uptime')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={isRunning() && telemetryAvailable('uptime')}
                fallback={
                  <span class="text-xs text-slate-400" aria-hidden="true">
                    —
                  </span>
                }
              >
                <span
                  class={`text-xs whitespace-nowrap ${props.guest.uptime > 0 && props.guest.uptime < 3600 ? 'text-orange-500' : 'text-muted'}`}
                >
                  <Show
                    when={isMobile() || usesCompactTableLayout()}
                    fallback={formatUptime(props.guest.uptime)}
                  >
                    {formatUptime(props.guest.uptime, true)}
                  </Show>
                </span>
              </Show>
            </div>
          </td>
        </Show>

        {/* Node - NEW */}
        <Show when={isColVisible('node')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={props.guest.node}
                fallback={
                  <span class="text-xs text-slate-400" aria-hidden="true">
                    —
                  </span>
                }
              >
                <span
                  class="text-xs text-base-content truncate max-w-[80px]"
                  title={props.guest.node}
                >
                  {props.guest.node}
                </span>
              </Show>
            </div>
          </td>
        </Show>

        {/* Image */}
        <Show when={isColVisible('image')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex items-center justify-start gap-1.5">
              <Show
                when={workloadType() === 'app-container' && dockerImage()}
                fallback={
                  <span class="text-xs " aria-hidden="true">
                    —
                  </span>
                }
              >
                <span class="text-xs text-muted truncate max-w-[140px]" title={dockerImage()}>
                  {getShortImageName(dockerImage())}
                </span>
              </Show>
            </div>
          </td>
        </Show>

        {/* Namespace */}
        <Show when={isColVisible('namespace')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={workloadType() === 'pod' && namespace()}
                fallback={
                  <span class="text-xs " aria-hidden="true">
                    —
                  </span>
                }
              >
                <span class="text-xs text-muted truncate max-w-[120px]" title={namespace()}>
                  {namespace()}
                </span>
              </Show>
            </div>
          </td>
        </Show>

        {/* Context */}
        <Show when={isColVisible('context')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex items-center justify-center gap-1.5">
              <Show
                when={contextLabel()}
                fallback={
                  <span class="text-xs " aria-hidden="true">
                    —
                  </span>
                }
              >
                <Show
                  when={isPveWorkload() && clusterName()}
                  fallback={
                    <span class="text-xs text-muted truncate max-w-[140px]" title={contextLabel()}>
                      {contextLabel()}
                    </span>
                  }
                >
                  <span class="text-xs text-muted truncate max-w-[80px]" title={props.guest.node}>
                    {props.guest.node}
                  </span>
                  <span class="rounded-sm px-1.5 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900/25 dark:text-blue-300">
                    {clusterName()}
                  </span>
                </Show>
              </Show>
            </div>
          </td>
        </Show>

        {/* Backup Status */}
        <Show when={isColVisible('backup')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={supportsBackup()}
                fallback={
                  <span class="text-xs text-slate-400" aria-hidden="true">
                    —
                  </span>
                }
              >
                <Show when={!props.guest.template}>
                  <BackupStatusCell
                    guestName={props.guest.name}
                    lastBackup={props.guest.lastBackup}
                    backupRunning={props.guest.backupInProgress}
                    guestReadEvidence={props.guest}
                  />
                </Show>
                <Show when={props.guest.template}>
                  <span class="text-xs text-slate-400" aria-hidden="true">
                    —
                  </span>
                </Show>
              </Show>
            </div>
          </td>
        </Show>

        {/* Tags */}
        <Show when={isColVisible('tags')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center" onClick={(event) => event.stopPropagation()}>
              <TagBadges
                tags={Array.isArray(props.guest.tags) ? props.guest.tags : []}
                maxVisible={0}
                sourceInstance={props.guest.instance}
                onTagClick={props.onTagClick}
                activeSearch={props.activeSearch}
              />
            </div>
          </td>
        </Show>

        {/* OS / System */}
        <Show when={isColVisible('os')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex items-center justify-center gap-1">
              <Show
                when={hasOsInfo()}
                fallback={
                  <Show when={ociImage()}>
                    <span
                      class="text-xs text-cyan-600 dark:text-cyan-400 truncate max-w-[100px]"
                      title={`OCI Image: ${ociImage()}`}
                    >
                      {ociImage()}
                    </span>
                  </Show>
                }
              >
                <OSInfoCell
                  osName={osName()}
                  osVersion={osVersion()}
                  agentVersion={agentVersion()}
                />
              </Show>
            </div>
          </td>
        </Show>

        {/* Net I/O */}
        <Show when={isColVisible('netIo')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <Show when={isSparklineMode()}>{renderRateSparkline('netIo')}</Show>
            <Show when={!isSparklineMode()}>
              <Show
                when={isRunning() && telemetryAvailable('networkIO')}
                fallback={
                  <div class="text-center">
                    <span class="text-xs text-slate-400" aria-hidden="true">
                      —
                    </span>
                  </div>
                }
              >
                <Show
                  when={historyLensVisible()}
                  fallback={
                    <div class="grid w-full min-w-0 grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 overflow-hidden text-[11px] tabular-nums">
                      <span class="inline-flex w-3 justify-center text-emerald-500">↓</span>
                      <span
                        class={`block min-w-0 overflow-hidden text-ellipsis whitespace-nowrap ${networkEmphasis().className}`}
                        title={
                          networkEmphasis().showOutlierHint
                            ? `${formatSpeed(networkIn())} (Top outlier)`
                            : formatSpeed(networkIn())
                        }
                      >
                        {formatSpeed(networkIn())}
                      </span>
                      <span class="inline-flex w-3 justify-center text-orange-400">↑</span>
                      <span
                        class={`block min-w-0 overflow-hidden text-ellipsis whitespace-nowrap ${networkEmphasis().className}`}
                        title={
                          networkEmphasis().showOutlierHint
                            ? `${formatSpeed(networkOut())} (Top outlier)`
                            : formatSpeed(networkOut())
                        }
                      >
                        {formatSpeed(networkOut())}
                      </span>
                    </div>
                  }
                >
                  <div class="h-4 animate-in fade-in-0 duration-100 motion-reduce:animate-none">
                    {renderRateSparkline('netIo')}
                  </div>
                </Show>
              </Show>
            </Show>
          </td>
        </Show>

        {/* Disk I/O */}
        <Show when={isColVisible('diskIo')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <Show when={isSparklineMode()}>{renderRateSparkline('diskIo')}</Show>
            <Show when={!isSparklineMode()}>
              <Show
                when={isRunning() && telemetryAvailable('diskIO')}
                fallback={
                  <div class="text-center">
                    <span class="text-xs text-slate-400" aria-hidden="true">
                      —
                    </span>
                  </div>
                }
              >
                <Show
                  when={historyLensVisible()}
                  fallback={
                    <div class="grid w-full min-w-0 grid-cols-[0.75rem_minmax(0,1fr)_0.75rem_minmax(0,1fr)] items-center gap-x-1 overflow-hidden text-[11px] tabular-nums">
                      <span class="inline-flex w-3 justify-center font-mono text-blue-500">R</span>
                      <span
                        class={`block min-w-0 overflow-hidden text-ellipsis whitespace-nowrap ${diskIOEmphasis().className}`}
                        title={
                          diskIOEmphasis().showOutlierHint
                            ? `${formatSpeed(diskRead())} (Top outlier)`
                            : formatSpeed(diskRead())
                        }
                      >
                        {formatSpeed(diskRead())}
                      </span>
                      <span class="inline-flex w-3 justify-center font-mono text-amber-500">W</span>
                      <span
                        class={`block min-w-0 overflow-hidden text-ellipsis whitespace-nowrap ${diskIOEmphasis().className}`}
                        title={
                          diskIOEmphasis().showOutlierHint
                            ? `${formatSpeed(diskWrite())} (Top outlier)`
                            : formatSpeed(diskWrite())
                        }
                      >
                        {formatSpeed(diskWrite())}
                      </span>
                    </div>
                  }
                >
                  <div class="h-4 animate-in fade-in-0 duration-100 motion-reduce:animate-none">
                    {renderRateSparkline('diskIo')}
                  </div>
                </Show>
              </Show>
            </Show>
          </td>
        </Show>

        {/* Update (Docker only) */}
        <Show when={isColVisible('update')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show when={workloadType() === 'app-container' && dockerHostId()}>
                <UpdateButton
                  updateStatus={props.guest.updateStatus}
                  agentId={dockerHostId()}
                  containerId={props.guest.containerId ?? ''}
                  containerName={props.guest.name}
                  resourceId={props.guest.id}
                  actionReadiness={props.guest.actionReadiness}
                  compact={true}
                />
              </Show>
            </div>
          </td>
        </Show>
      </tr>
    </>
  );
}
