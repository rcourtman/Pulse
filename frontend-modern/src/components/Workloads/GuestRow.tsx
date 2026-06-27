import { createMemo, Show } from 'solid-js';
import BoxIcon from 'lucide-solid/icons/box';
import type { VM } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { formatUptime, formatSpeed, getShortImageName } from '@/utils/format';
import { StackedDiskBar } from './StackedDiskBar';
import { StackedMemoryBar } from './StackedMemoryBar';
import {
  AvailabilityProbeCell,
  BackupIndicator,
  BackupStatusCell,
  InfoTooltipCell,
  NetworkInfoCell,
  OSInfoCell,
} from './GuestRowCells';

import { StatusDot } from '@/components/shared/StatusDot';
import { TagBadges } from '@/components/shared/TagBadges';
import { WorkloadTypeBadge } from '@/components/shared/WorkloadTypeBadge';
import { resolveWorkloadType } from '@/utils/workloads';
import { EnhancedCPUBar } from '@/components/Workloads/EnhancedCPUBar';
import { MetricMiniSparkline } from '@/components/Workloads/MetricMiniSparkline';
import { UpdateButton } from '@/components/shared/ContainerUpdateBadge';
import {
  buildSummaryDisclosureControlsId,
  createSummaryInteractiveRowPreviewHandlers,
} from '@/components/shared/summaryInteractionA11y';
import { SummaryRowActionButton } from '@/components/shared/SummaryRowActionButton';
import { DiscoveryReadinessBadge } from '@/components/shared/DiscoveryReadinessBadge';
import { getWorkloadGuestDiskStatusMessage } from '@/utils/workloadGuestPresentation';
import { WebInterfaceNameLink } from '@/components/shared/WebInterfaceNameLink';
import type { GuestRowProps } from './guestRowModel';
import { useGuestRowState } from './useGuestRowState';
import type { WorkloadTableMetric } from './workloadMetricHistoryModel';

type Guest = WorkloadGuest;

// Type guard for VM vs Container
const isVM = (guest: Guest): guest is VM => {
  return resolveWorkloadType(guest) === 'vm';
};

export { GUEST_COLUMNS, VIEW_MODE_COLUMNS } from './guestRowModel';
export type { GuestRowProps, WorkloadIOEmphasis } from './guestRowModel';
import { getGuestColumnStyle } from './guestRowModel';

export function GuestRow(props: GuestRowProps) {
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
    discoveryReadinessPresentation,
    diskRead,
    diskThresholds,
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

  const cpuPercent = createMemo(() => (props.guest.cpu || 0) * 100);
  const metricDisplayMode = createMemo(() => props.metricDisplayMode ?? 'bars');
  const isSparklineMode = createMemo(() => metricDisplayMode() === 'sparklines');
  const detailControlsId = createMemo(() => buildSummaryDisclosureControlsId(guestId()));
  const nestedWorkloadCueLabel = createMemo(() => {
    const context = props.nestedWorkloadContext;
    if (!context || context.count <= 0) return '';
    const noun = context.count === 1 ? 'container' : 'containers';
    return `${context.count} nested ${context.label} ${noun}. Open row for details.`;
  });
  const normalizeMetricPercent = (value: number | null | undefined): number => {
    if (typeof value !== 'number' || !Number.isFinite(value)) return 0;
    return value <= 1 ? Math.max(0, value * 100) : Math.max(0, value);
  };
  const formatMetricPercent = (value: number | null | undefined): string =>
    `${Math.round(normalizeMetricPercent(value))}%`;
  const usagePercent = (
    used?: number | null,
    total?: number | null,
    fallbackUsage?: number | null,
  ): number => {
    if (typeof used === 'number' && typeof total === 'number' && total > 0) {
      return normalizeMetricPercent((used / total) * 100);
    }
    return normalizeMetricPercent(fallbackUsage);
  };
  const renderMetricSparkline = (
    metric: WorkloadTableMetric,
    valueLabel: string,
    title: string,
    unit = '%',
    valueLabelMode: 'inline' | 'tooltip' | 'hidden' = 'inline',
    formatValue?: (value: number) => string,
  ) => (
    <MetricMiniSparkline
      series={props.metricHistory?.getGuestMetricSeries(props.guest, metric) ?? []}
      valueLabel={valueLabel}
      valueLabelMode={valueLabelMode}
      title={title}
      unit={unit}
      formatValue={formatValue}
    />
  );

  const getDiskStatusTooltip = () => {
    if (!isVM(props.guest)) return 'Disk stats unavailable';

    const vm = props.guest as VM;
    return getWorkloadGuestDiskStatusMessage(vm.diskStatusReason);
  };
  const interactiveRowHandlers =
    props.onClick || props.onHoverChange
      ? createSummaryInteractiveRowPreviewHandlers({
          onPreview: () => props.onHoverChange?.(guestId()),
          onPreviewClear: () => props.onHoverChange?.(null),
        })
      : {};

  return (
    <>
      <tr
        class={`${rowClass()} workload-row ${props.onClick ? 'cursor-pointer group' : ''}`.trim()}
        data-guest-id={guestId()}
        data-workload-alert-accent={alertAccentTone()}
        data-summary-series-id={guestId()}
        data-summary-group-member-active={
          props.summaryGroupMemberState && props.summaryGroupMemberState !== 'default'
            ? props.summaryGroupMemberState
            : undefined
        }
        data-summary-row-active={props.isSummaryHighlighted && !props.isExpanded ? 'true' : 'false'}
        onClick={props.onClick}
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
          )}
        >
          <div class="flex items-center gap-2 min-w-0">
            <Show when={props.onClick}>
              <SummaryRowActionButton
                kind="disclosure"
                subjectLabel={props.guest.name}
                expanded={props.isExpanded === true}
                controlsId={detailControlsId()}
                onAction={() => props.onClick?.()}
                onPreviewClear={() => props.onHoverChange?.(null)}
              />
            </Show>
            <div class="flex items-center gap-1.5 min-w-0">
              <StatusDot
                variant={guestStatus().variant}
                title={guestStatus().label}
                ariaLabel={guestStatus().label}
                size="xs"
              />
              <div class="flex items-center gap-1.5 min-w-0 group/name">
                <WebInterfaceNameLink
                  name={props.guest.name}
                  url={customUrl()}
                  class="text-[11px] font-medium text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline select-none truncate"
                  fallbackClass="text-[11px] font-medium text-base-content select-none truncate"
                />
                {/* Show backup indicator in name cell only if backup column is hidden */}
                <Show when={!isColVisible('backup') && supportsBackup()}>
                  <BackupIndicator
                    lastBackup={props.guest.lastBackup}
                    isTemplate={props.guest.template}
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
                class="text-[10px] font-medium text-muted uppercase tracking-wide whitespace-nowrap"
                title={`Guest is locked (${lockLabel()})`}
              >
                Lock: {lockLabel()}
              </span>
            </Show>
          </div>
        </td>

        {/* Availability */}
        <Show when={isColVisible('availability')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle text-center" data-workload-col="availability">
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
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center text-xs text-muted whitespace-nowrap">
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
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center text-xs text-muted whitespace-nowrap">
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
              when={isSparklineMode()}
              fallback={
                <div class="h-4">
                  <EnhancedCPUBar
                    usage={cpuPercent()}
                    cores={isMobile() ? undefined : props.guest.cpus}
                    resourceId={metricsKey()}
                    anomaly={cpuAnomaly()}
                    thresholds={cpuThresholds()}
                  />
                </div>
              }
            >
              {renderMetricSparkline(
                'cpu',
                formatMetricPercent(cpuPercent()),
                `${props.guest.name} CPU history`,
                '%',
                'inline',
                formatMetricPercent,
              )}
            </Show>
          </td>
        </Show>

        {/* Memory */}
        <Show when={isColVisible('memory')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle" data-workload-col="memory">
            <Show
              when={isSparklineMode()}
              fallback={
                <div title={memoryTooltip() ?? undefined}>
                  <StackedMemoryBar
                    used={props.guest.memory?.used || 0}
                    total={props.guest.memory?.total || 0}
                    percentOnly={memoryPercentOnly()}
                    cache={props.guest.memory?.cache || 0}
                    cacheInclusiveLabel="Shown in Proxmox"
                    balloon={props.guest.memory?.balloon || 0}
                    swapUsed={props.guest.memory?.swapUsed || 0}
                    swapTotal={props.guest.memory?.swapTotal || 0}
                    resourceId={metricsKey()}
                    anomaly={memoryAnomaly()}
                    thresholds={memoryThresholds()}
                  />
                </div>
              }
            >
              {renderMetricSparkline(
                'memory',
                formatMetricPercent(
                  usagePercent(
                    props.guest.memory?.used,
                    props.guest.memory?.total,
                    props.guest.memory?.usage,
                  ),
                ),
                `${props.guest.name} memory history`,
                '%',
                'inline',
                formatMetricPercent,
              )}
            </Show>
          </td>
        </Show>

        {/* Disk */}
        <Show when={isColVisible('disk')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle" data-workload-col="disk">
            <Show when={isSparklineMode()}>
              {renderMetricSparkline(
                'disk',
                hasDiskUsage() ? formatMetricPercent(props.guest.disk?.usage) : '—',
                `${props.guest.name} disk usage history`,
                '%',
                'inline',
                formatMetricPercent,
              )}
            </Show>
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
                <StackedDiskBar
                  disks={props.guest.disks}
                  aggregateDisk={props.guest.disk}
                  anomaly={diskAnomaly()}
                  thresholds={diskThresholds()}
                />
              </Show>
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
                when={isRunning()}
                fallback={
                  <span class="text-xs text-slate-400" aria-hidden="true">
                    —
                  </span>
                }
              >
                <span
                  class={`text-xs whitespace-nowrap ${props.guest.uptime > 0 && props.guest.uptime < 3600 ? 'text-orange-500' : 'text-muted'}`}
                >
                  <Show when={isMobile()} fallback={formatUptime(props.guest.uptime)}>
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
                  <span class="rounded px-1.5 py-0.5 text-[10px] font-medium whitespace-nowrap bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                    {clusterName()}
                  </span>
                </Show>
              </Show>
            </div>
          </td>
        </Show>

        {/* AI context readiness */}
        <Show when={isColVisible('aiContext')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={discoveryReadinessPresentation()}
                fallback={
                  <span class="text-xs text-slate-400" aria-hidden="true">
                    —
                  </span>
                }
              >
                {(presentation) => (
                  <DiscoveryReadinessBadge presentation={presentation()} compact />
                )}
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
                  <BackupStatusCell lastBackup={props.guest.lastBackup} />
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
            <Show when={isSparklineMode()}>
              {renderMetricSparkline(
                'netIo',
                isRunning() ? `${formatSpeed(networkIn())} / ${formatSpeed(networkOut())}` : '—',
                `${props.guest.name} network I/O history`,
                'B/s',
                'tooltip',
                formatSpeed,
              )}
            </Show>
            <Show when={!isSparklineMode()}>
              <Show
                when={isRunning()}
                fallback={
                  <div class="text-center">
                    <span class="text-xs text-slate-400" aria-hidden="true">
                      —
                    </span>
                  </div>
                }
              >
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
              </Show>
            </Show>
          </td>
        </Show>

        {/* Disk I/O */}
        <Show when={isColVisible('diskIo')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <Show when={isSparklineMode()}>
              {renderMetricSparkline(
                'diskIo',
                isRunning() ? `${formatSpeed(diskRead())} / ${formatSpeed(diskWrite())}` : '—',
                `${props.guest.name} disk I/O history`,
                'B/s',
                'tooltip',
                formatSpeed,
              )}
            </Show>
            <Show when={!isSparklineMode()}>
              <Show
                when={isRunning()}
                fallback={
                  <div class="text-center">
                    <span class="text-xs text-slate-400" aria-hidden="true">
                      —
                    </span>
                  </div>
                }
              >
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
