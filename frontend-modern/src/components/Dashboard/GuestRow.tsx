import { For, Show, createMemo, createSignal, createEffect, onMount, on } from 'solid-js';
import type { VM, Container } from '@/types/api';
import { formatBytes, formatUptime } from '@/utils/format';
import { MetricBar } from './MetricBar';
import { IOMetric } from './IOMetric';
import { TagBadges } from './TagBadges';
import { DiskList } from './DiskList';
import { GuestMetadataAPI } from '@/api/guestMetadata';

type Guest = VM | Container;

const drawerState = new Map<string, boolean>();

const buildGuestId = (guest: Guest) => {
  if (guest.id) return guest.id;
  if (guest.instance === guest.node) {
    return `${guest.node}-${guest.vmid}`;
  }
  return `${guest.instance}-${guest.node}-${guest.vmid}`;
};

// Type guard for VM vs Container
const isVM = (guest: Guest): guest is VM => {
  return guest.type === 'qemu';
};

interface GuestRowProps {
  guest: Guest;
  alertStyles?: {
    rowClass: string;
    indicatorClass: string;
    badgeClass: string;
    hasAlert: boolean;
    alertCount: number;
    severity: 'critical' | 'warning' | null;
  };
  customUrl?: string;
  onTagClick?: (tag: string) => void;
  activeSearch?: string;
  parentNodeOnline?: boolean;
}

export function GuestRow(props: GuestRowProps) {
  const [customUrl, setCustomUrl] = createSignal<string | undefined>(props.customUrl);
  const initialGuestId = buildGuestId(props.guest);
  const [drawerOpen, setDrawerOpen] = createSignal(drawerState.get(initialGuestId) ?? false);
  const guestId = createMemo(() => buildGuestId(props.guest));

  const hasMultipleDisks = createMemo(() => (props.guest.disks?.length ?? 0) > 1);
  const ipAddresses = createMemo(() => props.guest.ipAddresses ?? []);
  const networkInterfaces = createMemo(() => props.guest.networkInterfaces ?? []);
  const hasNetworkInterfaces = createMemo(() => networkInterfaces().length > 0);
  const osName = createMemo(() => props.guest.osName?.trim() ?? '');
  const osVersion = createMemo(() => props.guest.osVersion?.trim() ?? '');
  const hasOsInfo = createMemo(() => osName().length > 0 || osVersion().length > 0);

  // Update custom URL when prop changes
  createEffect(() => {
    setCustomUrl(props.customUrl);
  });


  // Load custom URL from backend if not provided via props
  onMount(async () => {
    if (!props.customUrl) {
      const startTime = performance.now();
      try {
        const metadata = await GuestMetadataAPI.getMetadata(guestId());
        const endTime = performance.now();
        console.log(`[PERF] Individual metadata call for ${guestId()} took ${(endTime - startTime).toFixed(2)}ms`);
        if (metadata && metadata.customUrl) {
          setCustomUrl(metadata.customUrl);
        }
      } catch (err) {
        // Silently fail - not critical for display
        console.debug('Failed to load guest metadata:', err);
      }
    }
  });
  const cpuPercent = createMemo(() => (props.guest.cpu || 0) * 100);
  const memPercent = createMemo(() => {
    if (!props.guest.memory) return 0;
    // Use the pre-calculated usage percentage from the backend
    return props.guest.memory.usage || 0;
  });
  const memoryUsageLabel = createMemo(() => {
    if (!props.guest.memory) return undefined;
    const used = props.guest.memory.used ?? 0;
    const total = props.guest.memory.total ?? 0;
    return `${formatBytes(used)}/${formatBytes(total)}`;
  });
  const memoryExtraLines = createMemo(() => {
    if (!props.guest.memory) return undefined;
    const lines: string[] = [];
    const total = props.guest.memory.total ?? 0;
    if (
      props.guest.memory.balloon &&
      props.guest.memory.balloon > 0 &&
      props.guest.memory.balloon !== total
    ) {
      lines.push(`Balloon: ${formatBytes(props.guest.memory.balloon)}`);
    }
    if (props.guest.memory.swapTotal && props.guest.memory.swapTotal > 0) {
      const swapUsed = props.guest.memory.swapUsed ?? 0;
      lines.push(`Swap: ${formatBytes(swapUsed)} / ${formatBytes(props.guest.memory.swapTotal)}`);
    }
    return lines.length > 0 ? lines : undefined;
  });
  const memoryTooltip = createMemo(() =>
    memoryExtraLines()?.join('\n') ?? undefined,
  );
  const canShowDrawer = createMemo(() =>
    hasOsInfo() ||
    ipAddresses().length > 0 ||
    (memoryExtraLines() && memoryExtraLines()!.length > 0) ||
    hasMultipleDisks() ||
    hasNetworkInterfaces(),
  );

  createEffect(on(guestId, (id) => {
    const stored = drawerState.get(id);
    if (stored !== undefined) {
      setDrawerOpen(stored);
    } else {
      setDrawerOpen(false);
    }
  }));

  createEffect(() => {
    drawerState.set(guestId(), drawerOpen());
  });

  createEffect(() => {
    if (!canShowDrawer() && drawerOpen()) {
      setDrawerOpen(false);
      drawerState.set(guestId(), false);
    }
  });

  const toggleDrawer = (event: MouseEvent) => {
    if (!canShowDrawer()) return;
    const target = event.target as HTMLElement;
    if (target.closest('a, button, [data-prevent-toggle]')) {
      return;
    }
    setDrawerOpen((prev) => !prev);
  };
  const diskPercent = createMemo(() => {
    if (!props.guest.disk || props.guest.disk.total === 0) return 0;
    // Check if usage is -1 (unknown/no guest agent)
    if (props.guest.disk.usage === -1) return -1;
    return (props.guest.disk.used / props.guest.disk.total) * 100;
  });

  const isRunning = createMemo(() => {
    if (props.parentNodeOnline === false) {
      return false;
    }
    return props.guest.status === 'running';
  });

  // Get helpful tooltip for disk status
  const getDiskStatusTooltip = () => {
    if (!isVM(props.guest)) return 'Disk stats unavailable';

    const vm = props.guest as VM;
    const reason = vm.diskStatusReason;

    switch (reason) {
      case 'agent-not-running':
        return 'Guest agent not running. Install and start qemu-guest-agent in the VM.';
      case 'agent-timeout':
        return 'Guest agent timeout. Agent may need to be restarted.';
      case 'permission-denied':
        return 'Permission denied. Check that your Pulse user/token has VM.Monitor permission (PVE 8) or VM.GuestAgent.Audit permission (PVE 9).';
      case 'agent-disabled':
        return 'Guest agent is disabled in VM configuration. Enable it in VM Options.';
      case 'no-filesystems':
        return 'No filesystems found. VM may be booting or using a Live ISO.';
      case 'special-filesystems-only':
        return 'Only special filesystems detected (ISO/squashfs). This is normal for Live systems.';
      case 'agent-error':
        return 'Error communicating with guest agent.';
      case 'no-data':
        return 'No disk data available from Proxmox API.';
      default:
        return 'Disk stats unavailable. Guest agent may not be installed.';
    }
  };

  const showAlertHighlight = createMemo(() => {
    if (!props.alertStyles?.hasAlert) return false;
    if (!isRunning() && props.alertStyles.severity === 'warning') {
      return false;
    }
    return true;
  });

  // Get row styling - include alert styles if present
  const rowClass = createMemo(() => {
    const base = 'transition-all duration-200 relative';
    const hover = 'hover:shadow-sm';
    const alertBg = showAlertHighlight()
      ? props.alertStyles?.severity === 'critical'
        ? 'bg-red-50 dark:bg-red-950/30'
        : 'bg-yellow-50 dark:bg-yellow-950/20'
      : '';
    const defaultHover = showAlertHighlight() ? '' : 'hover:bg-gray-50 dark:hover:bg-gray-700/30';
    const stoppedDimming = !isRunning() ? 'opacity-60' : '';
    const clickable = canShowDrawer() ? 'cursor-pointer' : '';
    const expanded = drawerOpen() && !showAlertHighlight() ? 'bg-gray-50 dark:bg-gray-800/40' : '';
    return `${base} ${hover} ${defaultHover} ${alertBg} ${stoppedDimming} ${clickable} ${expanded}`;
  });

  // Get first cell styling
  const firstCellClass = createMemo(() => {
    const base = 'py-0.5 pr-2 whitespace-nowrap relative';
    const indent = showAlertHighlight() ? 'pl-6' : 'pl-5';
    return `${base} ${indent}`;
  });

  // Get row styles including box-shadow for alert border
  const rowStyle = createMemo(() => {
    if (!showAlertHighlight()) return {};
    const color = props.alertStyles?.severity === 'critical' ? '#ef4444' : '#eab308';
    return {
      'box-shadow': `inset 4px 0 0 0 ${color}`,
    };
  });

  return (
    <>
      <tr class={rowClass()} style={rowStyle()} onClick={toggleDrawer} aria-expanded={drawerOpen()}>
      {/* Name - Sticky column */}
      <td class={firstCellClass()}>
        <div class="flex items-center gap-2">
          {/* Name - clickable if custom URL is set */}
          <Show
            when={customUrl()}
            fallback={
              <span
                class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate"
                title={props.guest.name}
              >
                {props.guest.name}
              </span>
            }
          >
            <a
              href={customUrl()}
              target="_blank"
              rel="noopener noreferrer"
              class="text-sm font-medium text-gray-900 dark:text-gray-100 hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-150 cursor-pointer truncate"
              title={`${props.guest.name} - Click to open custom URL`}
              onClick={(event) => event.stopPropagation()}
            >
              {props.guest.name}
            </a>
          </Show>

          {/* Tag badges */}
          <div class="flex" data-prevent-toggle onClick={(event) => event.stopPropagation()}>
            <TagBadges
              tags={Array.isArray(props.guest.tags) ? props.guest.tags : []}
              maxVisible={3}
              onTagClick={props.onTagClick}
              activeSearch={props.activeSearch}
            />
          </div>
        </div>
      </td>

      {/* Type */}
      <td class="py-0.5 px-2 whitespace-nowrap">
        <span
          class={`inline-block px-1.5 py-0.5 text-xs font-medium rounded ${
            props.guest.type === 'qemu'
              ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300'
              : 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300'
          }`}
        >
          {isVM(props.guest) ? 'VM' : 'LXC'}
        </span>
      </td>

      {/* VMID */}
      <td class="py-0.5 px-2 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400">
        {props.guest.vmid}
      </td>

      {/* Uptime */}
      <td
        class={`py-0.5 px-2 text-sm whitespace-nowrap ${
          props.guest.uptime < 3600 ? 'text-orange-500' : 'text-gray-600 dark:text-gray-400'
        }`}
      >
        <Show when={isRunning()} fallback="-">
          {formatUptime(props.guest.uptime)}
        </Show>
      </td>

      {/* CPU */}
      <td class="py-0.5 px-2 w-[140px]">
        <Show
          when={isRunning() && props.parentNodeOnline !== false}
          fallback={<span class="text-sm text-gray-400">-</span>}
        >
          <MetricBar
            value={cpuPercent()}
            label={`${cpuPercent().toFixed(0)}%`}
            sublabel={
              props.guest.cpus
                ? `${((props.guest.cpu || 0) * props.guest.cpus).toFixed(1)}/${props.guest.cpus} cores`
                : undefined
            }
            type="cpu"
          />
        </Show>
      </td>

      {/* Memory */}
      <td class="py-0.5 px-2 w-[140px]">
        <div title={memoryTooltip() ?? undefined}>
          <Show
            when={isRunning() && props.parentNodeOnline !== false}
            fallback={<span class="text-sm text-gray-400">-</span>}
          >
            <MetricBar
              value={memPercent()}
              label={`${memPercent().toFixed(0)}%`}
              sublabel={memoryUsageLabel()}
              type="memory"
            />
          </Show>
        </div>
      </td>

      {/* Disk */}
      <td class="py-0.5 px-2 w-[140px]">
        <Show
          when={
            isRunning() &&
            props.parentNodeOnline !== false &&
            props.guest.disk &&
            props.guest.disk.total > 0 &&
            diskPercent() !== -1
          }
          fallback={
            <span class="text-gray-400 text-sm cursor-help" title={getDiskStatusTooltip()}>
              -
            </span>
          }
        >
          <MetricBar
            value={diskPercent()}
            label={`${diskPercent().toFixed(0)}%`}
            sublabel={
              props.guest.disk
                ? `${formatBytes(props.guest.disk.used)}/${formatBytes(props.guest.disk.total)}`
                : undefined
            }
            type="disk"
          />
        </Show>
      </td>

      {/* Disk I/O */}
      <td class="py-0.5 px-2">
        <IOMetric value={props.guest.diskRead} disabled={!isRunning()} />
      </td>
      <td class="py-0.5 px-2">
        <IOMetric value={props.guest.diskWrite} disabled={!isRunning()} />
      </td>

      {/* Network I/O */}
      <td class="py-0.5 px-2">
        <IOMetric value={props.guest.networkIn} disabled={!isRunning()} />
      </td>
      <td class="py-0.5 px-2">
        <IOMetric value={props.guest.networkOut} disabled={!isRunning()} />
      </td>
      </tr>
      <Show when={drawerOpen() && canShowDrawer()}>
        <tr class="bg-gray-50/60 dark:bg-gray-800/40 text-[11px] text-gray-600 dark:text-gray-300">
          <td class="px-4 py-2" colSpan={11}>
            <div class="flex flex-wrap gap-3 justify-start">
              <Show when={hasOsInfo() || ipAddresses().length > 0}>
                <div class="min-w-[220px] rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                  <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Guest Overview</div>
                  <div class="mt-1 space-y-1">
                    <Show when={hasOsInfo()}>
                      <div class="flex flex-wrap items-center gap-1 text-gray-600 dark:text-gray-300">
                        <Show when={osName().length > 0}>
                          <span class="font-medium" title={osName()}>{osName()}</span>
                        </Show>
                        <Show when={osName().length > 0 && osVersion().length > 0}>
                          <span class="text-gray-400 dark:text-gray-500">•</span>
                        </Show>
                        <Show when={osVersion().length > 0}>
                          <span title={osVersion()}>{osVersion()}</span>
                        </Show>
                      </div>
                    </Show>
                    <Show when={ipAddresses().length > 0}>
                      <div class="flex flex-wrap gap-1">
                        <For each={ipAddresses()}>
                          {(ip) => (
                            <span class="rounded bg-blue-100 px-1.5 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                              {ip}
                            </span>
                          )}
                        </For>
                      </div>
                    </Show>
                  </div>
                </div>
              </Show>

              <Show when={memoryExtraLines() && memoryExtraLines()!.length > 0}>
                <div class="min-w-[220px] rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                  <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Memory</div>
                  <div class="mt-1 space-y-1 text-gray-600 dark:text-gray-300">
                    <For each={memoryExtraLines()!}>{(line) => <div>{line}</div>}</For>
                  </div>
                </div>
              </Show>

              <Show when={hasMultipleDisks() && props.guest.disks && props.guest.disks.length > 0}>
                <div class="min-w-[220px] rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                  <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Filesystems</div>
                  <div class="mt-1 text-gray-600 dark:text-gray-300">
                    <DiskList
                      disks={props.guest.disks || []}
                      diskStatusReason={isVM(props.guest) ? props.guest.diskStatusReason : undefined}
                    />
                  </div>
                </div>
              </Show>

              <Show when={hasNetworkInterfaces()}>
                <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                  <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Network Interfaces</div>
                  <div class="mt-1 text-[10px] text-gray-400 dark:text-gray-500">Row charts show current rate; totals below are cumulative since boot.</div>
                  <div class="mt-1 space-y-1 text-gray-600 dark:text-gray-300">
                    <For each={networkInterfaces()}>
                      {(iface) => {
                        const addresses = iface.addresses ?? [];
                        const hasTraffic = (iface.rxBytes ?? 0) > 0 || (iface.txBytes ?? 0) > 0;
                        return (
                          <div class="space-y-1 rounded border border-dashed border-gray-200 p-2 last:mb-0 dark:border-gray-700">
                            <div class="flex items-center gap-2 font-medium text-gray-700 dark:text-gray-200">
                              <span class="truncate" title={iface.name}>{iface.name || 'interface'}</span>
                              <Show when={iface.mac}>
                                <span class="text-[10px] text-gray-400 dark:text-gray-500" title={iface.mac}>
                                  {iface.mac}
                                </span>
                              </Show>
                            </div>
                            <Show when={addresses.length > 0}>
                              <div class="flex flex-wrap gap-1">
                                <For each={addresses}>
                                  {(ip) => (
                                    <span class="rounded bg-blue-100 px-1.5 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                                      {ip}
                                    </span>
                                  )}
                                </For>
                              </div>
                            </Show>
                            <Show when={hasTraffic}>
                              <div class="flex items-center gap-3 text-[10px] text-gray-500 dark:text-gray-400">
                                <span>Total RX {formatBytes(iface.rxBytes ?? 0)}</span>
                                <span>Total TX {formatBytes(iface.txBytes ?? 0)}</span>
                              </div>
                            </Show>
                          </div>
                        );
                      }}
                    </For>
                  </div>
                </div>
              </Show>
            </div>
          </td>
        </tr>
      </Show>
    </>
  );
}
