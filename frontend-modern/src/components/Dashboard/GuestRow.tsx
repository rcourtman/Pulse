import { createMemo, createSignal, createEffect, on, Show, For } from 'solid-js';
import type { VM, Container } from '@/types/api';
import { formatBytes, formatUptime } from '@/utils/format';
import { MetricBar } from './MetricBar';
import { IOMetric } from './IOMetric';
import { TagBadges } from './TagBadges';
import { DiskList } from './DiskList';
import { isGuestRunning, shouldDisplayGuestMetrics } from '@/utils/status';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import { showSuccess, showError } from '@/utils/toast';

type Guest = VM | Container;

const drawerState = new Map<string, boolean>();
// Global editing state - use a signal so all components react
const [currentlyEditingGuestId, setCurrentlyEditingGuestId] = createSignal<string | null>(null);
// Store the editing value globally so it survives re-renders
const editingValues = new Map<string, string>();
// Signal to trigger reactivity when editing values change
const [editingValuesVersion, setEditingValuesVersion] = createSignal(0);

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
    hasPoweredOffAlert?: boolean;
    hasNonPoweredOffAlert?: boolean;
    hasUnacknowledgedAlert?: boolean;
    unacknowledgedCount?: number;
    acknowledgedCount?: number;
    hasAcknowledgedOnlyAlert?: boolean;
  };
  customUrl?: string;
  onTagClick?: (tag: string) => void;
  activeSearch?: string;
  parentNodeOnline?: boolean;
  onCustomUrlUpdate?: (guestId: string, url: string) => void;
}

export function GuestRow(props: GuestRowProps) {
  const initialGuestId = buildGuestId(props.guest);
  const guestId = createMemo(() => buildGuestId(props.guest));
  const isEditingUrl = createMemo(() => currentlyEditingGuestId() === guestId());

  const [customUrl, setCustomUrl] = createSignal<string | undefined>(props.customUrl);
  const [drawerOpen, setDrawerOpen] = createSignal(drawerState.get(initialGuestId) ?? false);
  const editingUrlValue = createMemo(() => {
    editingValuesVersion(); // Subscribe to changes
    return editingValues.get(guestId()) || '';
  });
  let urlInputRef: HTMLInputElement | undefined;

  const hasFilesystemDetails = createMemo(() => (props.guest.disks?.length ?? 0) > 0);
  const ipAddresses = createMemo(() => props.guest.ipAddresses ?? []);
  const networkInterfaces = createMemo(() => props.guest.networkInterfaces ?? []);
  const hasNetworkInterfaces = createMemo(() => networkInterfaces().length > 0);
  const osName = createMemo(() => props.guest.osName?.trim() ?? '');
  const osVersion = createMemo(() => props.guest.osVersion?.trim() ?? '');
  const agentVersion = createMemo(() => props.guest.agentVersion?.trim() ?? '');
  const hasOsInfo = createMemo(() => osName().length > 0 || osVersion().length > 0);
  const hasAgentInfo = createMemo(() => agentVersion().length > 0);

  // Update custom URL when prop changes, but only if we're not currently editing
  createEffect(() => {
    // Don't update customUrl from props if this guest is currently being edited
    if (currentlyEditingGuestId() !== guestId()) {
      setCustomUrl(props.customUrl);
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
  const memoryTooltip = createMemo(() => memoryExtraLines()?.join('\n') ?? undefined);
  const hasDrawerContent = createMemo(
    () =>
      hasOsInfo() ||
      hasAgentInfo() ||
      ipAddresses().length > 0 ||
      (memoryExtraLines()?.length ?? 0) > 0 ||
      hasFilesystemDetails() ||
      hasNetworkInterfaces(),
  );
  const hasFallbackContent = createMemo(
    () => !hasDrawerContent() && (props.guest.type === 'qemu' || props.guest.type === 'lxc'),
  );
  const canShowDrawer = createMemo(() => hasDrawerContent() || hasFallbackContent());

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
    if (target.closest('a, button, input, [data-prevent-toggle]')) {
      return;
    }
    setDrawerOpen((prev) => !prev);
  };

  const startEditingUrl = (event: MouseEvent) => {
    event.stopPropagation();

    // If another guest is being edited, save it first
    const currentEditing = currentlyEditingGuestId();
    if (currentEditing !== null && currentEditing !== guestId()) {
      // Find the input for the currently editing guest and blur it
      const currentInput = document.querySelector(`input[data-guest-id="${currentEditing}"]`) as HTMLInputElement;
      if (currentInput) {
        currentInput.blur();
      }
    }

    editingValues.set(guestId(), customUrl() || '');
    setEditingValuesVersion(v => v + 1);
    setCurrentlyEditingGuestId(guestId());

    // Focus the input after it renders
    queueMicrotask(() => {
      if (urlInputRef) {
        urlInputRef.focus();
        urlInputRef.select();
      }
    });
  };

  const saveUrl = async () => {
    // Only save if this guest is the one being edited
    if (currentlyEditingGuestId() !== guestId()) return;

    const newUrl = (editingValues.get(guestId()) || '').trim();

    // Clear global editing state
    editingValues.delete(guestId());
    setEditingValuesVersion(v => v + 1);
    setCurrentlyEditingGuestId(null);

    // If URL hasn't changed, don't save
    if (newUrl === (customUrl() || '')) return;

    try {
      await GuestMetadataAPI.updateMetadata(guestId(), { customUrl: newUrl });
      setCustomUrl(newUrl || undefined);

      // Notify parent to update metadata
      if (props.onCustomUrlUpdate) {
        props.onCustomUrlUpdate(guestId(), newUrl);
      }

      if (newUrl) {
        showSuccess('Guest URL saved');
      } else {
        showSuccess('Guest URL cleared');
      }
    } catch (err: any) {
      console.error('Failed to save guest URL:', err);
      showError(err.message || 'Failed to save guest URL');
    }
  };

  const cancelEditingUrl = () => {
    // Only cancel if this guest is the one being edited
    if (currentlyEditingGuestId() !== guestId()) return;

    editingValues.delete(guestId());
    setEditingValuesVersion(v => v + 1);
    setCurrentlyEditingGuestId(null);
  };
  const diskPercent = createMemo(() => {
    if (!props.guest.disk || props.guest.disk.total === 0) return 0;
    // Check if usage is -1 (unknown/no guest agent)
    if (props.guest.disk.usage === -1) return -1;
    return (props.guest.disk.used / props.guest.disk.total) * 100;
  });
  const hasDiskUsage = createMemo(() => {
    if (!props.guest.disk) return false;
    if (props.guest.disk.total <= 0) return false;
    return diskPercent() !== -1;
  });

  const parentOnline = createMemo(() => props.parentNodeOnline !== false);
  const isRunning = createMemo(() => isGuestRunning(props.guest, parentOnline()));
  const showGuestMetrics = createMemo(() => shouldDisplayGuestMetrics(props.guest, parentOnline()));
  const lockLabel = createMemo(() => (props.guest.lock || '').trim());

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

  const hasUnacknowledgedAlert = createMemo(() => !!props.alertStyles?.hasUnacknowledgedAlert);
  const hasAcknowledgedOnlyAlert = createMemo(() => !!props.alertStyles?.hasAcknowledgedOnlyAlert);
  const showAlertHighlight = createMemo(
    () => hasUnacknowledgedAlert() || hasAcknowledgedOnlyAlert(),
  );

  const alertAccentColor = createMemo(() => {
    if (!showAlertHighlight()) return undefined;
    if (hasUnacknowledgedAlert()) {
      return props.alertStyles?.severity === 'critical' ? '#ef4444' : '#eab308';
    }
    return '#9ca3af';
  });

  const drawerDisabled = createMemo(() => !isRunning());

  // Get row styling - include alert styles if present
  const rowClass = createMemo(() => {
    const base = 'transition-all duration-200 relative';
    const hover = 'hover:shadow-sm';
    const alertBg = hasUnacknowledgedAlert()
      ? props.alertStyles?.severity === 'critical'
        ? 'bg-red-50 dark:bg-red-950/30'
        : 'bg-yellow-50 dark:bg-yellow-950/20'
      : '';
    const defaultHover = hasUnacknowledgedAlert()
      ? ''
      : 'hover:bg-gray-50 dark:hover:bg-gray-700/30';
    const stoppedDimming = !isRunning() ? 'opacity-60' : '';
    const clickable = canShowDrawer() ? 'cursor-pointer' : '';
    const expanded = drawerOpen() && !hasUnacknowledgedAlert()
      ? 'bg-gray-50 dark:bg-gray-800/40'
      : '';
    return `${base} ${hover} ${defaultHover} ${alertBg} ${stoppedDimming} ${clickable} ${expanded}`;
  });

  // Get first cell styling
  const firstCellClass = createMemo(() => {
    const base =
      'py-0.5 pr-2 whitespace-nowrap relative w-[130px] sm:w-[150px] lg:w-[180px] xl:w-[200px] 2xl:w-[240px]';
    const indent = 'pl-4';
    return `${base} ${indent}`;
  });

  // Get row styles including box-shadow for alert border
  const rowStyle = createMemo(() => {
    if (!showAlertHighlight()) return {};
    const color = alertAccentColor();
    if (!color) return {};
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
          {/* Name - show input when editing, otherwise show name with optional link */}
          <Show
            when={isEditingUrl()}
            fallback={
              <div class="flex items-center gap-1.5 flex-1 min-w-0">
                <span
                  class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate cursor-text select-none"
                  style="cursor: text;"
                  title={`${props.guest.name}${customUrl() ? ' - Click to edit URL' : ' - Click to add URL'}`}
                  onClick={startEditingUrl}
                >
                  {props.guest.name}
                </span>
                <Show when={customUrl()}>
                  <a
                    href={customUrl()}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="flex-shrink-0 text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors"
                    title="Open in new tab"
                    onClick={(event) => event.stopPropagation()}
                  >
                    <svg
                      class="w-3.5 h-3.5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                      />
                    </svg>
                  </a>
                </Show>
              </div>
            }
          >
            <div class="flex-1 flex items-center gap-1">
              <input
                ref={urlInputRef}
                type="text"
                value={editingUrlValue()}
                data-guest-id={guestId()}
                onInput={(e) => {
                  editingValues.set(guestId(), e.currentTarget.value);
                  setEditingValuesVersion(v => v + 1);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    saveUrl();
                  } else if (e.key === 'Escape') {
                    e.preventDefault();
                    cancelEditingUrl();
                  }
                }}
                onClick={(e) => e.stopPropagation()}
                placeholder="https://192.168.1.100:8006"
                class="flex-1 min-w-0 px-2 py-0.5 text-sm border border-blue-500 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  saveUrl();
                }}
                class="px-2 py-0.5 text-xs bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
                title="Save (or press Enter)"
              >
                ✓
              </button>
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  cancelEditingUrl();
                }}
                class="px-2 py-0.5 text-xs bg-gray-500 text-white rounded hover:bg-gray-600 transition-colors"
                title="Cancel (or press Escape)"
              >
                ✕
              </button>
            </div>
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

          <Show when={lockLabel()}>
            <span
              class="text-[10px] font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide"
              title={`Guest is locked (${lockLabel()})`}
            >
              Lock: {lockLabel()}
            </span>
          </Show>
        </div>
      </td>

      {/* Type */}
      <td class="py-0.5 px-2 whitespace-nowrap w-[48px] sm:w-[56px] lg:w-[60px] xl:w-[64px] 2xl:w-[72px]">
        <div class="flex h-[24px] items-center">
          <span
            class={`inline-block px-1.5 py-0.5 text-xs font-medium rounded ${
              props.guest.type === 'qemu'
                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300'
                : 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300'
            }`}
          >
            {isVM(props.guest) ? 'VM' : 'LXC'}
          </span>
        </div>
      </td>

      {/* VMID */}
      <td class="py-0.5 px-2 whitespace-nowrap w-[56px] sm:w-[64px] lg:w-[72px] xl:w-[80px] 2xl:w-[90px] text-sm text-gray-600 dark:text-gray-400 align-middle">
        {props.guest.vmid}
      </td>

      {/* Uptime */}
      <td
        class={`py-0.5 px-2 w-[76px] sm:w-[86px] lg:w-[96px] xl:w-[108px] 2xl:w-[128px] text-sm whitespace-nowrap align-middle ${
          props.guest.uptime < 3600 ? 'text-orange-500' : 'text-gray-600 dark:text-gray-400'
        }`}
      >
        <Show when={isRunning()} fallback="-">
          {formatUptime(props.guest.uptime)}
        </Show>
      </td>

      {/* CPU */}
      <td class="py-0.5 px-2 w-[100px] sm:w-[110px] lg:w-[130px] xl:w-[150px] 2xl:w-[180px]">
        <Show when={showGuestMetrics()} fallback={<span class="text-sm text-gray-400">-</span>}>
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
      <td class="py-0.5 px-2 w-[100px] sm:w-[110px] lg:w-[130px] xl:w-[150px] 2xl:w-[180px]">
        <div title={memoryTooltip() ?? undefined}>
          <Show when={showGuestMetrics()} fallback={<span class="text-sm text-gray-400">-</span>}>
            <MetricBar
              value={memPercent()}
              label={`${memPercent().toFixed(0)}%`}
              sublabel={memoryUsageLabel()}
              type="memory"
            />
          </Show>
        </div>
      </td>

      {/* Disk – surface usage even if guest is currently stopped so users can see last reported values */}
      <td class="py-0.5 px-2 w-[100px] sm:w-[110px] lg:w-[130px] xl:w-[150px] 2xl:w-[180px]">
        <Show
          when={hasDiskUsage()}
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
      <td class="py-0.5 px-2 w-[56px] sm:w-[62px] lg:w-[70px] xl:w-[78px] 2xl:w-[96px]">
        <div class="flex h-[24px] items-center">
          <IOMetric value={props.guest.diskRead} disabled={!isRunning()} />
        </div>
      </td>
      <td class="py-0.5 px-2 w-[56px] sm:w-[62px] lg:w-[70px] xl:w-[78px] 2xl:w-[96px]">
        <div class="flex h-[24px] items-center">
          <IOMetric value={props.guest.diskWrite} disabled={!isRunning()} />
        </div>
      </td>

      {/* Network I/O */}
      <td class="py-0.5 px-2 w-[56px] sm:w-[62px] lg:w-[70px] xl:w-[78px] 2xl:w-[96px]">
        <div class="flex h-[24px] items-center">
          <IOMetric value={props.guest.networkIn} disabled={!isRunning()} />
        </div>
      </td>
      <td class="py-0.5 px-2 w-[56px] sm:w-[62px] lg:w-[70px] xl:w-[78px] 2xl:w-[96px]">
        <div class="flex h-[24px] items-center">
          <IOMetric value={props.guest.networkOut} disabled={!isRunning()} />
        </div>
      </td>
      </tr>
      <Show when={drawerOpen() && canShowDrawer()}>
        <tr
          class={`text-[11px] ${
            isRunning() && props.parentNodeOnline !== false
              ? 'bg-gray-50/60 text-gray-600 dark:bg-gray-800/40 dark:text-gray-300'
              : 'bg-gray-100/70 text-gray-400 dark:bg-gray-900/30 dark:text-gray-500'
          }`}
          aria-hidden={!isRunning() || props.parentNodeOnline === false}
        >
          <td class="px-4 py-2" colSpan={11}>
            <div
              class={`flex flex-wrap gap-3 justify-start ${
                drawerDisabled() ? 'opacity-50 saturate-75 pointer-events-none' : ''
              }`}
            >
              <Show
                when={hasDrawerContent()}
                fallback={
                  <div class="min-w-[220px] rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                    <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">
                      Guest details unavailable
                    </div>
                    <div class="mt-1 space-y-1 text-gray-600 dark:text-gray-300">
                      <Show
                        when={isVM(props.guest)}
                        fallback={
                          <p>
                            Start this container and ensure the Pulse user has sufficient Proxmox
                            permissions to collect guest metrics.
                          </p>
                        }
                      >
                        <p>{getDiskStatusTooltip()}</p>
                        <p>
                          Install and run the qemu-guest-agent inside this VM so Pulse can surface
                          OS, network, and filesystem details.
                        </p>
                      </Show>
                    </div>
                  </div>
                }
              >
                <>
                  <Show when={hasOsInfo() || hasAgentInfo() || ipAddresses().length > 0}>
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
                        <Show when={hasAgentInfo()}>
                          <div class="flex flex-wrap items-center gap-1 text-[11px] text-gray-500 dark:text-gray-400">
                            <span class="uppercase tracking-wide text-[10px] text-gray-400 dark:text-gray-500">
                              Agent
                            </span>
                            <span title={`QEMU guest agent ${agentVersion()}`}>
                              QEMU guest agent {agentVersion()}
                            </span>
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

                  <Show when={hasFilesystemDetails() && props.guest.disks && props.guest.disks.length > 0}>
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
                </>
              </Show>
            </div>
          </td>
        </tr>
      </Show>
    </>
  );
}
