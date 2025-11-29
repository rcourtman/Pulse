import { GuestDrawer } from './GuestDrawer';
import { createMemo, createSignal, createEffect, Show, For } from 'solid-js';
import type { VM, Container } from '@/types/api';
import { formatBytes, formatUptime, formatSpeed, getBackupInfo, type BackupStatus, formatPercent } from '@/utils/format';
import { TagBadges } from './TagBadges';
import { StackedDiskBar } from './StackedDiskBar';
import { StackedMemoryBar } from './StackedMemoryBar';

import { StatusDot } from '@/components/shared/StatusDot';
import { getGuestPowerIndicator, isGuestRunning } from '@/utils/status';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import { showSuccess, showError } from '@/utils/toast';
import { logger } from '@/utils/logger';
import { buildMetricKey } from '@/utils/metricsKeys';
import { type ColumnPriority } from '@/hooks/useBreakpoint';
import { ResponsiveMetricCell, useGridTemplate } from '@/components/shared/responsive';

type Guest = VM | Container;

/**
 * Get color class for I/O values based on throughput (bytes/sec)
 * Uses color intensity to indicate activity level (green/yellow/red)
 */
function getIOColorClass(bytesPerSec: number): string {
  const mbps = bytesPerSec / (1024 * 1024);
  if (mbps < 1) return 'text-gray-300 dark:text-gray-400';
  if (mbps < 10) return 'text-green-600 dark:text-green-400';
  if (mbps < 50) return 'text-yellow-600 dark:text-yellow-400';
  return 'text-red-600 dark:text-red-400';
}

// Global state for currently expanded drawer (only one drawer open at a time)
const [currentlyExpandedGuestId, setCurrentlyExpandedGuestId] = createSignal<string | null>(null);
// Global editing state - use a signal so all components react
const [currentlyEditingGuestId, setCurrentlyEditingGuestId] = createSignal<string | null>(null);
// Store the editing value globally so it survives re-renders
const editingValues = new Map<string, string>();
// Signal to trigger reactivity when editing values change
const [editingValuesVersion, setEditingValuesVersion] = createSignal(0);

const GROUPED_FIRST_CELL_INDENT = 'pl-5 sm:pl-6 lg:pl-8';
const DEFAULT_FIRST_CELL_INDENT = 'pl-4';

const buildGuestId = (guest: Guest) => {
  if (guest.id) return guest.id;
  return `${guest.instance}-${guest.vmid}`;
};

// Type guard for VM vs Container
const isVM = (guest: Guest): guest is VM => {
  return guest.type === 'qemu';
};

// Backup status indicator colors and icons
const BACKUP_STATUS_CONFIG: Record<BackupStatus, { color: string; bgColor: string; icon: 'check' | 'warning' | 'x' }> = {
  fresh: { color: 'text-green-600 dark:text-green-400', bgColor: 'bg-green-100 dark:bg-green-900/40', icon: 'check' },
  stale: { color: 'text-yellow-600 dark:text-yellow-400', bgColor: 'bg-yellow-100 dark:bg-yellow-900/40', icon: 'warning' },
  critical: { color: 'text-red-600 dark:text-red-400', bgColor: 'bg-red-100 dark:bg-red-900/40', icon: 'x' },
  never: { color: 'text-gray-400 dark:text-gray-500', bgColor: 'bg-gray-100 dark:bg-gray-800', icon: 'x' },
};

function BackupIndicator(props: { lastBackup: string | number | null | undefined; isTemplate: boolean }) {
  // Don't show for templates
  if (props.isTemplate) return null;

  const backupInfo = createMemo(() => getBackupInfo(props.lastBackup));
  const config = createMemo(() => BACKUP_STATUS_CONFIG[backupInfo().status]);

  // Only show when there's a problem (stale, critical, or never)
  const shouldShow = createMemo(() => {
    const status = backupInfo().status;
    return status === 'stale' || status === 'critical' || status === 'never';
  });

  const tooltipText = createMemo(() => {
    const info = backupInfo();
    if (info.status === 'never') {
      return 'No backup found';
    }
    return `Last backup: ${info.ageFormatted}`;
  });

  return (
    <Show when={shouldShow()}>
      <span
        class={`flex-shrink-0 ${config().color}`}
        title={tooltipText()}
      >
        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
          {/* Shield shape */}
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
          {/* Inner icon based on status */}
          <Show when={config().icon === 'warning'}>
            <path d="M12 8v4M12 16h.01" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
          </Show>
          <Show when={config().icon === 'x'}>
            <path d="M10 10l4 4M14 10l-4 4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
          </Show>
        </svg>
      </span>
    </Show>
  );
}

// Column configuration using the priority system
interface ColumnDef {
  id: string;
  label: string;
  priority: ColumnPriority;
  minWidth?: string;
  maxWidth?: string;
  flex?: number;
}

export const GUEST_COLUMNS: ColumnDef[] = [
  { id: 'name', label: 'Name', priority: 'essential', minWidth: '100px', maxWidth: '300px' },
  { id: 'type', label: 'Type', priority: 'essential', minWidth: '24px', maxWidth: '50px' },
  { id: 'vmid', label: 'VMID', priority: 'essential', minWidth: '28px', maxWidth: '55px' },
  { id: 'uptime', label: 'Uptime', priority: 'essential', minWidth: '28px', maxWidth: '65px' },
  // Metric columns - fixed width to match progress bar max-width (140px + padding)
  { id: 'cpu', label: 'CPU', priority: 'essential', minWidth: '50px', maxWidth: '156px' },
  { id: 'memory', label: 'Memory', priority: 'essential', minWidth: '50px', maxWidth: '156px' },
  { id: 'disk', label: 'Disk', priority: 'essential', minWidth: '50px', maxWidth: '156px' },
  // I/O columns - fixed width
  { id: 'diskRead', label: 'Disk Read', priority: 'essential', minWidth: '56px', maxWidth: '90px' },
  { id: 'diskWrite', label: 'Disk Write', priority: 'essential', minWidth: '56px', maxWidth: '90px' },
  { id: 'netIn', label: 'Net In', priority: 'essential', minWidth: '56px', maxWidth: '70px' },
  { id: 'netOut', label: 'Net Out', priority: 'essential', minWidth: '56px', maxWidth: '70px' },
];

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
  isGroupedView?: boolean;
}

export function GuestRow(props: GuestRowProps) {
  const guestId = createMemo(() => buildGuestId(props.guest));
  const isEditingUrl = createMemo(() => currentlyEditingGuestId() === guestId());

  // Use the responsive grid template hook for dynamic column visibility
  const { gridTemplate, visibleColumns, isMobile } = useGridTemplate({ columns: GUEST_COLUMNS });

  // Create namespaced metrics key
  const metricsKey = createMemo(() => {
    const kind = props.guest.type === 'qemu' ? 'vm' : 'container';
    return buildMetricKey(kind, guestId());
  });

  const [customUrl, setCustomUrl] = createSignal<string | undefined>(props.customUrl);
  const [shouldAnimateIcon, setShouldAnimateIcon] = createSignal(false);
  const drawerOpen = createMemo(() => currentlyExpandedGuestId() === guestId());
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
      const prevUrl = customUrl();
      const newUrl = props.customUrl;

      // Only animate when URL transitions from empty to having a value
      if (!prevUrl && newUrl) {
        setShouldAnimateIcon(true);
        // Remove animation class after it completes
        setTimeout(() => setShouldAnimateIcon(false), 200);
      }

      setCustomUrl(newUrl);
    }
  });

  const cpuPercent = createMemo(() => (props.guest.cpu || 0) * 100);

  // I/O metrics - must use memos for reactivity with WebSocket updates
  const diskRead = createMemo(() => props.guest.diskRead || 0);
  const diskWrite = createMemo(() => props.guest.diskWrite || 0);
  const networkIn = createMemo(() => props.guest.networkIn || 0);
  const networkOut = createMemo(() => props.guest.networkOut || 0);

  const memPercent = createMemo(() => {
    if (!props.guest.memory) return 0;
    return props.guest.memory.usage || 0;
  });
  const memoryUsageLabel = createMemo(() => {
    if (!props.guest.memory) return undefined;
    const used = props.guest.memory.used ?? 0;
    const total = props.guest.memory.total ?? 0;
    return `${formatBytes(used, 0)}/${formatBytes(total, 0)}`;
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
      lines.push(`Balloon: ${formatBytes(props.guest.memory.balloon, 0)}`);
    }
    if (props.guest.memory.swapTotal && props.guest.memory.swapTotal > 0) {
      const swapUsed = props.guest.memory.swapUsed ?? 0;
      lines.push(`Swap: ${formatBytes(swapUsed, 0)} / ${formatBytes(props.guest.memory.swapTotal, 0)}`);
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

  createEffect(() => {
    if (!canShowDrawer() && drawerOpen()) {
      setCurrentlyExpandedGuestId(null);
    }
  });

  const toggleDrawer = (event: MouseEvent) => {
    if (!canShowDrawer()) return;
    const target = event.target as HTMLElement;
    if (target.closest('a, button, input, [data-prevent-toggle]')) {
      return;
    }
    setCurrentlyExpandedGuestId(prev => prev === guestId() ? null : guestId());
  };

  const startEditingUrl = (event: MouseEvent) => {
    event.stopPropagation();

    const currentEditing = currentlyEditingGuestId();
    if (currentEditing !== null && currentEditing !== guestId()) {
      const currentInput = document.querySelector(`input[data-guest-id="${currentEditing}"]`) as HTMLInputElement;
      if (currentInput) {
        currentInput.blur();
      }
    }

    editingValues.set(guestId(), customUrl() || '');
    setEditingValuesVersion(v => v + 1);
    setCurrentlyEditingGuestId(guestId());
  };

  createEffect(() => {
    if (isEditingUrl() && urlInputRef) {
      urlInputRef.focus();
      urlInputRef.select();
    }
  });

  let isCurrentlyMounted = true;

  createEffect(() => {
    if (isEditingUrl() && isCurrentlyMounted) {
      const handleGlobalClick = (e: MouseEvent) => {
        if (currentlyEditingGuestId() !== guestId()) return;

        const target = e.target as HTMLElement;
        const isClickingGuestName = target.closest('[data-guest-name-editable]');

        if (!target.closest('[data-url-editor]') && !isClickingGuestName) {
          e.preventDefault();
          e.stopPropagation();
          e.stopImmediatePropagation();
          cancelEditingUrl();
        }
      };

      const handleGlobalMouseDown = (e: MouseEvent) => {
        if (currentlyEditingGuestId() !== guestId()) return;

        const target = e.target as HTMLElement;
        const isClickingGuestName = target.closest('[data-guest-name-editable]');

        if (!target.closest('[data-url-editor]') && !isClickingGuestName) {
          e.preventDefault();
          e.stopPropagation();
          e.stopImmediatePropagation();
        }
      };

      document.addEventListener('mousedown', handleGlobalMouseDown, true);
      document.addEventListener('click', handleGlobalClick, true);
      return () => {
        document.removeEventListener('mousedown', handleGlobalMouseDown, true);
        document.removeEventListener('click', handleGlobalClick, true);
      };
    }
  });

  const saveUrl = async () => {
    if (currentlyEditingGuestId() !== guestId()) return;

    const newUrl = (editingValues.get(guestId()) || '').trim();

    editingValues.delete(guestId());
    setEditingValuesVersion(v => v + 1);
    setCurrentlyEditingGuestId(null);

    if (newUrl === (customUrl() || '')) return;

    try {
      await GuestMetadataAPI.updateMetadata(guestId(), { customUrl: newUrl });

      const hadUrl = !!customUrl();
      if (!hadUrl && newUrl) {
        setShouldAnimateIcon(true);
        setTimeout(() => setShouldAnimateIcon(false), 200);
      }

      setCustomUrl(newUrl || undefined);

      if (props.onCustomUrlUpdate) {
        props.onCustomUrlUpdate(guestId(), newUrl);
      }

      if (newUrl) {
        showSuccess('Guest URL saved');
      } else {
        showSuccess('Guest URL cleared');
      }
    } catch (err: any) {
      logger.error('Failed to save guest URL:', err);
      showError(err.message || 'Failed to save guest URL');
    }
  };

  const deleteUrl = async () => {
    if (currentlyEditingGuestId() !== guestId()) return;

    editingValues.delete(guestId());
    setEditingValuesVersion(v => v + 1);
    setCurrentlyEditingGuestId(null);

    if (customUrl()) {
      try {
        await GuestMetadataAPI.updateMetadata(guestId(), { customUrl: '' });
        setCustomUrl(undefined);

        if (props.onCustomUrlUpdate) {
          props.onCustomUrlUpdate(guestId(), '');
        }

        showSuccess('Guest URL removed');
      } catch (err: any) {
        logger.error('Failed to remove guest URL:', err);
        showError(err.message || 'Failed to remove guest URL');
      }
    }
  };

  const cancelEditingUrl = () => {
    if (currentlyEditingGuestId() !== guestId()) return;

    editingValues.delete(guestId());
    setEditingValuesVersion(v => v + 1);
    setCurrentlyEditingGuestId(null);
  };

  const diskPercent = createMemo(() => {
    if (!props.guest.disk || props.guest.disk.total === 0) return 0;
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
  const guestStatus = createMemo(() => getGuestPowerIndicator(props.guest, parentOnline()));
  const lockLabel = createMemo(() => (props.guest.lock || '').trim());

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

  const rowClass = createMemo(() => {
    const base = 'transition-all duration-200 relative';
    const hover = 'hover:shadow-sm';
    const alertBg = hasUnacknowledgedAlert()
      ? props.alertStyles?.severity === 'critical'
        ? 'bg-red-50 dark:bg-red-950/30'
        : 'bg-yellow-50 dark:bg-yellow-950/20'
      : 'bg-white dark:bg-gray-800';
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

  const rowStyle = createMemo(() => {
    if (!showAlertHighlight()) return {};
    const color = alertAccentColor();
    if (!color) return {};
    return {
      'box-shadow': `inset 4px 0 0 0 ${color}`,
    };
  });

  // Render cell content based on column type
  const renderCell = (column: ColumnDef) => {
    switch (column.id) {
      case 'name':
        return (
          <div class={`px-1 py-1 flex items-center min-w-0 ${props.isGroupedView ? GROUPED_FIRST_CELL_INDENT : DEFAULT_FIRST_CELL_INDENT}`}>
            <div class="flex items-center gap-2 min-w-0 w-full">
              <div class="flex items-center gap-1.5 flex-1 min-w-0">
                <StatusDot
                  variant={guestStatus().variant}
                  title={guestStatus().label}
                  ariaLabel={guestStatus().label}
                  size="xs"
                />
                <div class="flex-1 min-w-0">
                  <Show
                    when={isEditingUrl()}
                    fallback={
                      <div class="flex items-center gap-1.5 min-w-0">
                        <span
                          class="text-xs font-medium text-gray-900 dark:text-gray-100 cursor-text select-none overflow-hidden text-ellipsis whitespace-nowrap"
                          style="cursor: text;"
                          title={`${props.guest.name}${customUrl() ? ' - Click to edit URL' : ' - Click to add URL'}`}
                          onClick={startEditingUrl}
                          data-guest-name-editable
                        >
                          {props.guest.name}
                        </span>
                        <Show when={customUrl()}>
                          <a
                            href={customUrl()}
                            target="_blank"
                            rel="noopener noreferrer"
                            class={`flex-shrink-0 text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors ${shouldAnimateIcon() ? 'animate-fadeIn' : ''}`}
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
                        <BackupIndicator lastBackup={props.guest.lastBackup} isTemplate={props.guest.template} />
                      </div>
                    }
                  >
                    <div class="flex-1 flex items-center gap-1 min-w-0" data-url-editor>
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
                        data-url-editor-button
                        onClick={(e) => {
                          e.stopPropagation();
                          saveUrl();
                        }}
                        class="flex-shrink-0 w-6 h-6 flex items-center justify-center text-xs bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
                        title="Save (or press Enter)"
                      >
                        ✓
                      </button>
                      <button
                        type="button"
                        data-url-editor-button
                        onClick={(e) => {
                          e.stopPropagation();
                          deleteUrl();
                        }}
                        class="flex-shrink-0 w-6 h-6 flex items-center justify-center text-xs bg-red-600 text-white rounded hover:bg-red-700 transition-colors"
                        title="Delete URL"
                      >
                        ✕
                      </button>
                    </div>
                  </Show>
                </div>
              </div>

              <Show when={!isEditingUrl()}>
                <div class="hidden md:flex" data-prevent-toggle onClick={(event) => event.stopPropagation()}>
                  <TagBadges
                    tags={Array.isArray(props.guest.tags) ? props.guest.tags : []}
                    maxVisible={3}
                    onTagClick={props.onTagClick}
                    activeSearch={props.activeSearch}
                  />
                </div>
              </Show>

              <Show when={lockLabel()}>
                <span
                  class="text-[10px] font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide"
                  title={`Guest is locked (${lockLabel()})`}
                >
                  Lock: {lockLabel()}
                </span>
              </Show>
            </div>
          </div>
        );

      case 'type':
        return (
          <div class="px-0.5 py-1 flex justify-center items-center">
            <span
              class={`inline-block px-1 py-0.5 text-[10px] font-medium rounded ${props.guest.type === 'qemu'
                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300'
                : 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300'
                }`}
              title={isVM(props.guest) ? 'Virtual Machine' : 'LXC Container'}
            >
              {isVM(props.guest) ? 'VM' : 'LXC'}
            </span>
          </div>
        );

      case 'vmid':
        return (
          <div class="flex-1 px-0.5 py-1 md:px-1.5 md:py-0 w-auto min-w-[30px] md:w-full flex justify-center md:justify-start items-center text-xs text-gray-600 dark:text-gray-400">
            {props.guest.vmid}
          </div>
        );

      case 'uptime':
        return (
          <div class="flex-1 px-0.5 py-1 md:px-1.5 md:py-0 w-auto min-w-[40px] md:w-full flex justify-center md:justify-start items-center">
            <div class={`text-xs whitespace-nowrap ${props.guest.uptime < 3600 ? 'text-orange-500' : 'text-gray-600 dark:text-gray-400'}`}>
              <Show when={isRunning()} fallback="-">
                <Show when={isMobile()} fallback={formatUptime(props.guest.uptime)}>
                  {formatUptime(props.guest.uptime, true)}
                </Show>
              </Show>
            </div>
          </div>
        );

      case 'cpu':
        return (
          <div class="flex-1 px-0.5 py-1 md:px-2 md:py-0 w-auto min-w-[35px] md:w-full flex justify-center items-center">
            <ResponsiveMetricCell
              value={cpuPercent()}
              type="cpu"
              resourceId={metricsKey()}
              sublabel={
                props.guest.cpus
                  ? `${props.guest.cpus} ${props.guest.cpus === 1 ? 'core' : 'cores'}`
                  : undefined
              }
              isRunning={isRunning()}
              showMobile={isMobile()}
              class="w-full"
            />
          </div>
        );

      case 'memory':
        return (
          <div class="flex-1 px-0.5 py-1 md:px-2 md:py-0 w-auto min-w-[35px] md:w-full flex justify-center items-center">
            <div title={memoryTooltip() ?? undefined} class="w-full text-center xl:text-left">
              <Show when={isMobile()}>
                <div class="md:hidden">
                  <ResponsiveMetricCell
                    value={memPercent()}
                    type="memory"
                    resourceId={metricsKey()}
                    sublabel={memoryUsageLabel()}
                    isRunning={isRunning()}
                    showMobile={true}
                    class="w-full"
                  />
                </div>
              </Show>
              <div class="hidden md:block w-full">
                <StackedMemoryBar
                  used={props.guest.memory?.used || 0}
                  total={props.guest.memory?.total || 0}
                  balloon={props.guest.memory?.balloon || 0}
                  swapUsed={props.guest.memory?.swapUsed || 0}
                  swapTotal={props.guest.memory?.swapTotal || 0}
                />
              </div>
            </div>
          </div>
        );

      case 'disk':
        return (
          <div class="flex-1 px-0.5 py-1 md:px-2 md:py-0 w-auto min-w-[35px] md:w-full flex justify-center items-center">
            <Show
              when={hasDiskUsage()}
              fallback={
                <span class="text-xs text-gray-400 cursor-help" title={getDiskStatusTooltip()}>
                  -
                </span>
              }
            >
              {/* Mobile: simple percentage text */}
              <Show when={isMobile()}>
                <div class="md:hidden text-xs text-center text-gray-600 dark:text-gray-400">
                  {formatPercent(diskPercent())}
                </div>
              </Show>
              {/* Desktop: stacked disk bar for multiple disks */}
              <div class={isMobile() ? 'hidden md:block w-full' : 'w-full'}>
                <StackedDiskBar
                  disks={props.guest.disks}
                  aggregateDisk={props.guest.disk}
                />
              </div>
            </Show>
          </div>
        );

      case 'diskRead':
        return (
          <div class="py-1 flex justify-center items-center min-h-[24px]">
            <Show when={isRunning()} fallback={<span class="text-xs text-gray-400">-</span>}>
              <span class={`text-xs ${getIOColorClass(diskRead())}`}>{formatSpeed(diskRead())}</span>
            </Show>
          </div>
        );

      case 'diskWrite':
        return (
          <div class="py-1 flex justify-center items-center min-h-[24px]">
            <Show when={isRunning()} fallback={<span class="text-xs text-gray-400">-</span>}>
              <span class={`text-xs ${getIOColorClass(diskWrite())}`}>{formatSpeed(diskWrite())}</span>
            </Show>
          </div>
        );

      case 'netIn':
        return (
          <div class="py-1 flex justify-center items-center min-h-[24px]">
            <Show when={isRunning()} fallback={<span class="text-xs text-gray-400">-</span>}>
              <span class={`text-xs ${getIOColorClass(networkIn())}`}>{formatSpeed(networkIn())}</span>
            </Show>
          </div>
        );

      case 'netOut':
        return (
          <div class="py-1 flex justify-center items-center min-h-[24px]">
            <Show when={isRunning()} fallback={<span class="text-xs text-gray-400">-</span>}>
              <span class={`text-xs ${getIOColorClass(networkOut())}`}>{formatSpeed(networkOut())}</span>
            </Show>
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <>
      <div
        class={`${rowClass()} grid items-center`}
        style={{ ...rowStyle(), 'grid-template-columns': gridTemplate() }}
        onClick={toggleDrawer}
        aria-expanded={drawerOpen()}
      >
        <For each={visibleColumns()}>
          {(column) => renderCell(column)}
        </For>
      </div>

      <Show when={drawerOpen() && canShowDrawer()}>
        <div class="bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700">
          <div class="p-4">
            <GuestDrawer
              guest={props.guest}
              metricsKey={metricsKey()}
              onClose={() => setCurrentlyExpandedGuestId(null)}
            />
          </div>
        </div>
      </Show>
    </>
  );
}
