import { createMemo, Show } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { VM } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { formatUptime, formatSpeed, getShortImageName } from '@/utils/format';
import { StackedDiskBar } from './StackedDiskBar';
import { StackedMemoryBar } from './StackedMemoryBar';
import {
  BackupIndicator,
  BackupStatusCell,
  InfoTooltipCell,
  NetworkInfoCell,
  OSInfoCell,
} from './GuestRowCells';

import { StatusDot } from '@/components/shared/StatusDot';
import { TagBadges } from '@/components/shared/TagBadges';
import { resolveWorkloadType } from '@/utils/workloads';
import { EnhancedCPUBar } from '@/components/Dashboard/EnhancedCPUBar';
import { UpdateButton } from '@/components/shared/ContainerUpdateBadge';
import { getDashboardGuestDiskStatusMessage } from '@/utils/dashboardGuestPresentation';
import type { GuestRowProps } from './guestRowModel';
import { useGuestRowState } from './useGuestRowState';

type Guest = WorkloadGuest;

// Type guard for VM vs Container
const isVM = (guest: Guest): guest is VM => {
  return resolveWorkloadType(guest) === 'vm';
};

export { GUEST_COLUMNS, VIEW_MODE_COLUMNS } from './guestRowModel';
export type { GuestRowProps, WorkloadIOEmphasis } from './guestRowModel';

export function GuestRow(props: GuestRowProps) {
  const navigate = useNavigate();
  const {
    agentVersion,
    clusterName,
    contextLabel,
    cpuAnomaly,
    customUrl,
    diskAnomaly,
    diskIOEmphasis,
    diskRead,
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
    infrastructureHref,
    ipAddresses,
    isColVisible,
    isMobile,
    isPveWorkload,
    isRunning,
    lockLabel,
    memoryAnomaly,
    memoryPercentOnly,
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
    rowClass,
    rowStyle,
    supportsBackup,
    typeInfo,
    workloadType,
  } = useGuestRowState(props);

  const cpuPercent = createMemo(() => (props.guest.cpu || 0) * 100);

  const getDiskStatusTooltip = () => {
    if (!isVM(props.guest)) return 'Disk stats unavailable';

    const vm = props.guest as VM;
    return getDashboardGuestDiskStatusMessage(vm.diskStatusReason);
  };

  return (
    <>
      <tr
        class={`${rowClass()} ${props.onClick ? 'cursor-pointer group' : ''}`}
        style={rowStyle()}
        data-guest-id={guestId()}
        onClick={props.onClick}
        onMouseEnter={() => props.onHoverChange?.(guestId())}
        onMouseLeave={() => props.onHoverChange?.(null)}
      >
        {/* Name - always visible */}
        <td
          class={`pr-1.5 sm:pr-2 py-0.5 align-middle whitespace-nowrap ${firstCellIndent()}`}
        >
          <div class="flex items-center gap-2 min-w-0">
            <div class={`transition-transform duration-200 ${props.isExpanded ? 'rotate-90' : ''}`}>
              <svg
                class="w-3.5 h-3.5 text-muted group-hover:text-base-content"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </div>
            <div class="flex items-center gap-1.5 min-w-0">
              <StatusDot
                variant={guestStatus().variant}
                title={guestStatus().label}
                ariaLabel={guestStatus().label}
                size="xs"
              />
              <div class="flex items-center gap-1.5 min-w-0 group/name">
                <span
                  class="text-[11px] font-medium text-base-content select-none truncate"
                  title={props.guest.name}
                >
                  {props.guest.name}
                </span>
                {/* Show backup indicator in name cell only if backup column is hidden */}
                <Show when={!isColVisible('backup') && supportsBackup()}>
                  <BackupIndicator
                    lastBackup={props.guest.lastBackup}
                    isTemplate={props.guest.template}
                  />
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

        {/* Type */}
        <Show when={isColVisible('type')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <span
                class={`inline-flex items-center px-1 py-0.5 text-[10px] font-medium rounded whitespace-nowrap ${typeInfo().className}`}
                title={typeInfo().title}
              >
                {typeInfo().label}
              </span>
            </div>
          </td>
        </Show>

        {/* Info - merged identifier (VMID / image / namespace) for mixed-type views */}
        <Show when={isColVisible('info')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center text-xs text-muted whitespace-nowrap">
              <Show when={infoValue()} fallback={<span class="">—</span>}>
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
              <Show when={displayId()} fallback={<span class="">—</span>}>
                {displayId()}
              </Show>
            </div>
          </td>
        </Show>

        {/* CPU */}
        <Show when={isColVisible('cpu')}>
          <td
            class="px-1.5 sm:px-2 py-0.5 align-middle"
            style={
              isMobile()
                ? { 'min-width': '60px' }
                : { width: '140px', 'min-width': '140px', 'max-width': '140px' }
            }
          >
            <div class="h-4">
              <EnhancedCPUBar
                usage={cpuPercent()}
                cores={isMobile() ? undefined : props.guest.cpus}
                resourceId={metricsKey()}
                anomaly={cpuAnomaly()}
              />
            </div>
          </td>
        </Show>

        {/* Memory */}
        <Show when={isColVisible('memory')}>
          <td
            class="px-1.5 sm:px-2 py-0.5 align-middle"
            style={
              isMobile()
                ? { 'min-width': '60px' }
                : { width: '140px', 'min-width': '140px', 'max-width': '140px' }
            }
          >
            <div title={memoryTooltip() ?? undefined}>
              <StackedMemoryBar
                used={props.guest.memory?.used || 0}
                total={props.guest.memory?.total || 0}
                percentOnly={memoryPercentOnly()}
                balloon={props.guest.memory?.balloon || 0}
                swapUsed={props.guest.memory?.swapUsed || 0}
                swapTotal={props.guest.memory?.swapTotal || 0}
                resourceId={metricsKey()}
                anomaly={memoryAnomaly()}
              />
            </div>
          </td>
        </Show>

        {/* Disk */}
        <Show when={isColVisible('disk')}>
          <td
            class="px-1.5 sm:px-2 py-0.5 align-middle"
            style={
              isMobile()
                ? { 'min-width': '60px' }
                : { width: '140px', 'min-width': '140px', 'max-width': '140px' }
            }
          >
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
              />
            </Show>
          </td>
        </Show>

        {/* IP Address with Network Tooltip */}
        <Show when={isColVisible('ip')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={ipAddresses().length > 0 || hasNetworkInterfaces()}
                fallback={<span class="text-xs text-slate-400">—</span>}
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
              <Show when={isRunning()} fallback={<span class="text-xs text-slate-400">—</span>}>
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
                fallback={<span class="text-xs text-slate-400">—</span>}
              >
                <button
                  type="button"
                  class="text-xs text-blue-600 dark:text-blue-400 hover:underline truncate max-w-[80px]"
                  title={`${props.guest.node} (Open related infrastructure)`}
                  onClick={(event) => {
                    event.stopPropagation();
                    navigate(infrastructureHref());
                  }}
                >
                  {props.guest.node}
                </button>
              </Show>
            </div>
          </td>
        </Show>

        {/* Image */}
        <Show when={isColVisible('image')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={workloadType() === 'app-container' && dockerImage()}
                fallback={<span class="text-xs ">—</span>}
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
                fallback={<span class="text-xs ">—</span>}
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
              <Show when={contextLabel()} fallback={<span class="text-xs ">—</span>}>
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

        {/* Backup Status */}
        <Show when={isColVisible('backup')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={supportsBackup()}
                fallback={<span class="text-xs text-slate-400">—</span>}
              >
                <Show when={!props.guest.template}>
                  <BackupStatusCell lastBackup={props.guest.lastBackup} />
                </Show>
                <Show when={props.guest.template}>
                  <span class="text-xs text-slate-400">—</span>
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
                onTagClick={props.onTagClick}
                activeSearch={props.activeSearch}
              />
            </div>
          </td>
        </Show>

        {/* OS */}
        <Show when={isColVisible('os')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={hasOsInfo()}
                fallback={
                  <Show when={ociImage()} fallback={<span class="text-xs text-slate-400">—</span>}>
                    {/* For OCI containers without guest agent, show image name in OS column */}
                    <span
                      class="text-xs text-purple-600 dark:text-purple-400 truncate max-w-[100px]"
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
            <Show
              when={isRunning()}
              fallback={
                <div class="text-center">
                  <span class="text-xs text-slate-400">—</span>
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
          </td>
        </Show>

        {/* Disk I/O */}
        <Show when={isColVisible('diskIo')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <Show
              when={isRunning()}
              fallback={
                <div class="text-center">
                  <span class="text-xs text-slate-400">—</span>
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
          </td>
        </Show>

        {/* Update (Docker only) */}
        <Show when={isColVisible('update')}>
          <td class="px-1.5 sm:px-2 py-0.5 align-middle">
            <div class="flex justify-center">
              <Show
                when={workloadType() === 'app-container' && dockerHostId()}
              >
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

        {/* Link Column - at the end like InfrastructureSummaryTable */}
        <Show when={isColVisible('link')}>
          <td class="px-0 py-0.5 align-middle text-center">
            <Show
              when={customUrl() && customUrl() !== ''}
              fallback={
                <button
                  type="button"
                  class="inline-flex justify-center items-center text-muted hover:text-base-content transition-colors"
                  title="Open related infrastructure"
                  onClick={(event) => {
                    event.stopPropagation();
                    navigate(infrastructureHref());
                  }}
                >
                  <svg
                    class="w-3.5 h-3.5"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                  >
                    <rect x="2" y="2" width="20" height="8" rx="2" ry="2" />
                    <rect x="2" y="14" width="20" height="8" rx="2" ry="2" />
                    <line x1="6" y1="6" x2="6.01" y2="6" />
                    <line x1="6" y1="18" x2="6.01" y2="18" />
                  </svg>
                </button>
              }
            >
              <a
                href={customUrl()}
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex justify-center items-center text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors"
                title={`Open ${customUrl()}`}
                onClick={(event) => event.stopPropagation()}
              >
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                  />
                </svg>
              </a>
            </Show>
          </td>
        </Show>
      </tr>
    </>
  );
}
