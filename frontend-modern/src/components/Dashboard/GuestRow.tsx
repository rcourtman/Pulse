import { createMemo, createSignal, createEffect, Show, For } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { VM, Container, GuestNetworkInterface } from '@/types/api';
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
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useMetricsViewMode } from '@/stores/metricsViewMode';
import { aiChatStore } from '@/stores/aiChat';

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

// Network info cell with rich tooltip showing interfaces, IPs, and MACs
function NetworkInfoCell(props: { ipAddresses: string[]; networkInterfaces: GuestNetworkInterface[] }) {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

  const hasInterfaces = () => props.networkInterfaces.length > 0;
  const primaryIp = () => props.ipAddresses[0] || props.networkInterfaces[0]?.addresses?.[0] || null;
  const totalIps = () => {
    if (props.ipAddresses.length > 0) return props.ipAddresses.length;
    return props.networkInterfaces.reduce((sum, iface) => sum + (iface.addresses?.length || 0), 0);
  };

  const handleMouseEnter = (e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
    setShowTooltip(true);
  };

  const handleMouseLeave = () => {
    setShowTooltip(false);
  };

  return (
    <>
      <span
        class="inline-flex items-center gap-1 text-xs text-gray-600 dark:text-gray-400 cursor-help"
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        <Show when={primaryIp()} fallback="-">
          {/* Network icon */}
          <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" />
          </svg>
          <span class="text-[10px] font-medium">{totalIps()}</span>
        </Show>
      </span>

      <Show when={showTooltip() && (hasInterfaces() || props.ipAddresses.length > 0)}>
        <Portal mount={document.body}>
          <div
            class="fixed z-[9999] pointer-events-none"
            style={{
              left: `${tooltipPos().x}px`,
              top: `${tooltipPos().y - 8}px`,
              transform: 'translate(-50%, -100%)',
            }}
          >
            <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2 py-1.5 min-w-[180px] max-w-[280px] border border-gray-700">
              <div class="font-medium mb-1 text-gray-300 border-b border-gray-700 pb-1">
                Network Interfaces
              </div>

              {/* Show detailed interface info if available */}
              <Show when={hasInterfaces()}>
                <For each={props.networkInterfaces}>
                  {(iface, idx) => (
                    <div class="py-1" classList={{ 'border-t border-gray-700/50': idx() > 0 }}>
                      <div class="flex items-center gap-2 text-blue-400 font-medium">
                        <span>{iface.name || 'eth' + idx()}</span>
                        <Show when={iface.mac}>
                          <span class="text-[9px] text-gray-500 font-normal">{iface.mac}</span>
                        </Show>
                      </div>
                      <Show when={iface.addresses && iface.addresses.length > 0}>
                        <div class="mt-0.5 flex flex-wrap gap-1">
                          <For each={iface.addresses}>
                            {(ip) => (
                              <span class="text-gray-300 font-mono">{ip}</span>
                            )}
                          </For>
                        </div>
                      </Show>
                      <Show when={!iface.addresses || iface.addresses.length === 0}>
                        <span class="text-gray-500 text-[9px]">No IP assigned</span>
                      </Show>
                      <Show when={(iface.rxBytes || 0) > 0 || (iface.txBytes || 0) > 0}>
                        <div class="mt-0.5 text-[9px] text-gray-500">
                          RX: {formatBytes(iface.rxBytes || 0)} / TX: {formatBytes(iface.txBytes || 0)}
                        </div>
                      </Show>
                    </div>
                  )}
                </For>
              </Show>

              {/* Fallback: just show IP list if no interface details */}
              <Show when={!hasInterfaces() && props.ipAddresses.length > 0}>
                <div class="flex flex-wrap gap-1 py-0.5">
                  <For each={props.ipAddresses}>
                    {(ip) => (
                      <span class="text-gray-300 font-mono">{ip}</span>
                    )}
                  </For>
                </div>
              </Show>
            </div>
          </div>
        </Portal>
      </Show>
    </>
  );
}

// OS detection helper - simplified to just Linux vs Windows
type OSType = 'windows' | 'linux' | 'unknown';

function detectOSType(osName: string): OSType {
  const lower = osName.toLowerCase();
  if (lower.includes('windows')) return 'windows';
  // All Linux distros, BSDs, and Unix-likes -> linux
  if (lower.includes('linux') || lower.includes('debian') || lower.includes('ubuntu') ||
    lower.includes('alpine') || lower.includes('centos') || lower.includes('fedora') ||
    lower.includes('arch') || lower.includes('nixos') || lower.includes('suse') ||
    lower.includes('gentoo') || lower.includes('rhel') || lower.includes('rocky') ||
    lower.includes('alma') || lower.includes('devuan') || lower.includes('gnu') ||
    lower.includes('freebsd') || lower.includes('openbsd') || lower.includes('netbsd')) {
    return 'linux';
  }
  return 'unknown';
}


// OS info cell with icon and Portal tooltip
function OSInfoCell(props: { osName: string; osVersion: string; agentVersion: string }) {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

  const osType = createMemo(() => detectOSType(props.osName));

  const handleMouseEnter = (e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
    setShowTooltip(true);
  };

  const handleMouseLeave = () => {
    setShowTooltip(false);
  };

  // OS icons - Windows logo and terminal prompt for Linux
  const OSIcon = () => {
    const type = osType();
    const iconClass = 'w-3.5 h-3.5 text-gray-500 dark:text-gray-400';

    switch (type) {
      case 'windows':
        // Windows logo - four tilted panes
        return (
          <svg class={iconClass} viewBox="0 0 24 24" fill="currentColor">
            <path d="M3 5.5l7.038-1v6.5H3v-5.5zm0 13l7.038 1V13H3v5.5zm8.038 1.118L21 21V13h-9.962v6.618zM11.038 4.382L21 3v8h-9.962V4.382z" />
          </svg>
        );
      case 'linux':
        // Terminal prompt icon
        return (
          <svg class={iconClass} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="4 17 10 11 4 5" />
            <line x1="12" y1="19" x2="20" y2="19" />
          </svg>
        );
      default:
        return <span class="text-gray-400">-</span>;
    }
  };

  return (
    <>
      <span
        class="inline-flex items-center gap-1 cursor-help"
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        <OSIcon />
      </span>

      <Show when={showTooltip()}>
        <Portal mount={document.body}>
          <div
            class="fixed z-[9999] pointer-events-none"
            style={{
              left: `${tooltipPos().x}px`,
              top: `${tooltipPos().y - 8}px`,
              transform: 'translate(-50%, -100%)',
            }}
          >
            <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2 py-1.5 min-w-[120px] max-w-[220px] border border-gray-700">
              <div class="font-medium mb-1 text-gray-300 border-b border-gray-700 pb-1">
                Operating System
              </div>
              <div class="py-0.5">
                <div class="text-gray-200 font-medium">{props.osName}</div>
                <Show when={props.osVersion}>
                  <div class="text-gray-400">Version: {props.osVersion}</div>
                </Show>
                <Show when={props.agentVersion}>
                  <div class="text-gray-500 text-[9px] mt-1 pt-1 border-t border-gray-700/50">
                    Agent: {props.agentVersion}
                  </div>
                </Show>
              </div>
            </div>
          </div>
        </Portal>
      </Show>
    </>
  );
}

// Backup status cell with Portal tooltip
function BackupStatusCell(props: { lastBackup: string | number | null | undefined }) {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

  const info = createMemo(() => getBackupInfo(props.lastBackup));
  const config = createMemo(() => BACKUP_STATUS_CONFIG[info().status]);

  const handleMouseEnter = (e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
    setShowTooltip(true);
  };

  const handleMouseLeave = () => {
    setShowTooltip(false);
  };

  return (
    <>
      <span
        class={`inline-flex items-center gap-1 px-1.5 py-0.5 text-[10px] font-medium rounded cursor-help ${config().bgColor} ${config().color}`}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        {/* Status icon */}
        <Show when={info().status === 'fresh'}>
          <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
        </Show>
        <Show when={info().status === 'stale'}>
          <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        </Show>
        <Show when={info().status === 'critical' || info().status === 'never'}>
          <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </Show>
        {/* Status text */}
        {info().status === 'fresh' && 'OK'}
        {info().status === 'stale' && info().ageFormatted}
        {info().status === 'critical' && info().ageFormatted}
        {info().status === 'never' && 'Never'}
      </span>

      <Show when={showTooltip()}>
        <Portal mount={document.body}>
          <div
            class="fixed z-[9999] pointer-events-none"
            style={{
              left: `${tooltipPos().x}px`,
              top: `${tooltipPos().y - 8}px`,
              transform: 'translate(-50%, -100%)',
            }}
          >
            <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2 py-1.5 min-w-[140px] border border-gray-700">
              <div class="font-medium mb-1 text-gray-300 border-b border-gray-700 pb-1">
                Backup Status
              </div>
              <Show when={info().status !== 'never'}>
                <div class="py-0.5">
                  <div class="text-gray-400">Last backup</div>
                  <div class="text-gray-200 font-medium">
                    {new Date(props.lastBackup!).toLocaleDateString(undefined, {
                      weekday: 'short',
                      year: 'numeric',
                      month: 'short',
                      day: 'numeric',
                    })}
                  </div>
                  <div class="text-gray-300">
                    {new Date(props.lastBackup!).toLocaleTimeString()}
                  </div>
                </div>
                <div class="pt-1 mt-1 border-t border-gray-700/50">
                  <span class={config().color}>{info().ageFormatted} ago</span>
                </div>
              </Show>
              <Show when={info().status === 'never'}>
                <div class="py-0.5 text-red-400">
                  No backup has ever been recorded for this guest.
                </div>
              </Show>
            </div>
          </div>
        </Portal>
      </Show>
    </>
  );
}

// Column configuration using the priority system
export interface GuestColumnDef {
  id: string;
  label: string;
  priority: ColumnPriority;
  toggleable?: boolean;
  width?: string;  // Fixed width for consistent column sizing
  minWidth?: string;
  maxWidth?: string;
  flex?: number;
  sortKey?: string;
}

export const GUEST_COLUMNS: GuestColumnDef[] = [
  // Essential - always visible
  { id: 'name', label: 'Name', priority: 'essential', sortKey: 'name' },
  { id: 'type', label: 'Type', priority: 'essential', width: '40px', sortKey: 'type' },
  { id: 'vmid', label: 'VMID', priority: 'essential', width: '45px', sortKey: 'vmid' },

  // Core metrics - percentage width for proportional sizing
  { id: 'cpu', label: 'CPU', priority: 'essential', width: '15%', sortKey: 'cpu' },
  { id: 'memory', label: 'Memory', priority: 'essential', width: '15%', sortKey: 'memory' },
  { id: 'disk', label: 'Disk', priority: 'essential', width: '15%', sortKey: 'disk' },

  // Secondary - visible on md+ (768px), user toggleable
  { id: 'ip', label: 'IP', priority: 'secondary', width: '90px', toggleable: true },
  { id: 'uptime', label: 'Uptime', priority: 'secondary', width: '65px', toggleable: true, sortKey: 'uptime' },
  { id: 'node', label: 'Node', priority: 'secondary', width: '60px', toggleable: true, sortKey: 'node' },

  // Supplementary - visible on lg+ (1024px), user toggleable
  { id: 'backup', label: 'Backup', priority: 'supplementary', width: '55px', toggleable: true },
  { id: 'tags', label: 'Tags', priority: 'supplementary', width: '80px', toggleable: true },

  // Detailed - visible on xl+ (1280px), user toggleable
  { id: 'os', label: 'OS', priority: 'detailed', width: '45px', toggleable: true },
  { id: 'diskRead', label: 'D Read', priority: 'detailed', width: '55px', toggleable: true, sortKey: 'diskRead' },
  { id: 'diskWrite', label: 'D Write', priority: 'detailed', width: '55px', toggleable: true, sortKey: 'diskWrite' },
  { id: 'netIn', label: 'Net In', priority: 'detailed', width: '55px', toggleable: true, sortKey: 'networkIn' },
  { id: 'netOut', label: 'Net Out', priority: 'detailed', width: '55px', toggleable: true, sortKey: 'networkOut' },
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
  /** IDs of columns that should be visible */
  visibleColumnIds?: string[];
  /** Guest ID of the row above (for checking AI context adjacency) */
  aboveGuestId?: string | null;
  /** Guest ID of the row below (for checking AI context adjacency) */
  belowGuestId?: string | null;
  /** Called when user clicks the row (for AI context selection) */
  onRowClick?: (guest: Guest) => void;
}

export function GuestRow(props: GuestRowProps) {
  const guestId = createMemo(() => buildGuestId(props.guest));
  const isEditingUrl = createMemo(() => currentlyEditingGuestId() === guestId());

  // Use breakpoint hook directly for responsive behavior
  const { isMobile } = useBreakpoint();

  // Get current metrics view mode (bars vs sparklines)
  const { viewMode } = useMetricsViewMode();

  // Helper to check if a column is visible
  // If visibleColumnIds is not provided, show all columns for backwards compatibility
  const isColVisible = (colId: string) => {
    if (!props.visibleColumnIds) return true;
    return props.visibleColumnIds.includes(colId);
  };

  // Create namespaced metrics key for sparklines
  const metricsKey = createMemo(() => {
    const kind = props.guest.type === 'qemu' ? 'vm' : 'container';
    return buildMetricKey(kind, guestId());
  });

  const [customUrl, setCustomUrl] = createSignal<string | undefined>(props.customUrl);
  const [shouldAnimateIcon, setShouldAnimateIcon] = createSignal(false);
  const editingUrlValue = createMemo(() => {
    editingValuesVersion(); // Subscribe to changes
    return editingValues.get(guestId()) || '';
  });
  let urlInputRef: HTMLInputElement | undefined;

  const ipAddresses = createMemo(() => props.guest.ipAddresses ?? []);
  const networkInterfaces = createMemo(() => props.guest.networkInterfaces ?? []);
  const hasNetworkInterfaces = createMemo(() => networkInterfaces().length > 0);
  const osName = createMemo(() => props.guest.osName?.trim() ?? '');
  const osVersion = createMemo(() => props.guest.osVersion?.trim() ?? '');
  const agentVersion = createMemo(() => props.guest.agentVersion?.trim() ?? '');
  const hasOsInfo = createMemo(() => osName().length > 0 || osVersion().length > 0);

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

  // Check AI context state reactively from the store
  const isInAIContext = createMemo(() => aiChatStore.isOpen && aiChatStore.hasContextItem(guestId()));
  const isAboveInAIContext = createMemo(() => {
    if (!props.aboveGuestId) return false;
    return aiChatStore.hasContextItem(props.aboveGuestId);
  });
  const isBelowInAIContext = createMemo(() => {
    if (!props.belowGuestId) return false;
    return aiChatStore.hasContextItem(props.belowGuestId);
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
    // Make row clickable if click handler provided
    const clickable = props.onRowClick ? 'cursor-pointer' : '';
    // AI context highlight with merged borders for adjacent rows
    let aiContext = '';
    if (isInAIContext()) {
      aiContext = 'ai-context-row';
      if (isAboveInAIContext()) aiContext += ' ai-context-no-top';
      if (isBelowInAIContext()) aiContext += ' ai-context-no-bottom';
    }
    return `${base} ${hover} ${defaultHover} ${alertBg} ${stoppedDimming} ${clickable} ${aiContext}`;
  });

  const rowStyle = createMemo(() => {
    const styles: Record<string, string> = {};

    // Alert styling (only if not in AI context - AI context uses CSS class)
    if (!isInAIContext() && showAlertHighlight()) {
      const color = alertAccentColor();
      if (color) {
        styles['box-shadow'] = `inset 4px 0 0 0 ${color}`;
      }
    }

    return styles;
  });

  const handleRowClick = (e: MouseEvent) => {
    // Don't trigger if clicking on interactive elements
    const target = e.target as HTMLElement;
    if (target.closest('[data-url-editor]') ||
      target.closest('[data-prevent-toggle]') ||
      target.closest('a') ||
      target.closest('button') ||
      target.closest('input')) {
      return;
    }
    props.onRowClick?.(props.guest);
  };

  return (
    <tr
      class={rowClass()}
      style={rowStyle()}
      onClick={handleRowClick}
      data-guest-id={guestId()}
    >
      {/* Name - always visible */}
      <td class={`pr-2 py-1 align-middle whitespace-nowrap ${props.isGroupedView ? GROUPED_FIRST_CELL_INDENT : DEFAULT_FIRST_CELL_INDENT}`}>
        <div class="flex items-center gap-2 min-w-0">
          <div class="flex items-center gap-1.5 min-w-0">
            <StatusDot
              variant={guestStatus().variant}
              title={guestStatus().label}
              ariaLabel={guestStatus().label}
              size="xs"
            />
            <Show
              when={isEditingUrl()}
              fallback={
                <div class="flex items-center gap-1.5 min-w-0">
                  <span
                    class="text-xs font-medium text-gray-900 dark:text-gray-100 cursor-text select-none whitespace-nowrap"
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
                  {/* Show backup indicator in name cell only if backup column is hidden */}
                  <Show when={!isColVisible('backup')}>
                    <BackupIndicator lastBackup={props.guest.lastBackup} isTemplate={props.guest.template} />
                  </Show>
                </div>
              }
            >
              <div class="flex items-center gap-1 min-w-0" data-url-editor>
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
                  class="min-w-0 px-2 py-0.5 text-sm border border-blue-500 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
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

          {/* Show tags inline only if tags column is hidden */}
          <Show when={!isEditingUrl() && !isColVisible('tags')}>
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
              class="text-[10px] font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide whitespace-nowrap"
              title={`Guest is locked (${lockLabel()})`}
            >
              Lock: {lockLabel()}
            </span>
          </Show>
        </div>
      </td>

      {/* Type */}
      <Show when={isColVisible('type')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <span
              class={`inline-block px-1 py-0.5 text-[10px] font-medium rounded whitespace-nowrap ${props.guest.type === 'qemu'
                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300'
                : 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300'
                }`}
              title={isVM(props.guest) ? 'Virtual Machine' : 'LXC Container'}
            >
              {isVM(props.guest) ? 'VM' : 'LXC'}
            </span>
          </div>
        </td>
      </Show>

      {/* VMID */}
      <Show when={isColVisible('vmid')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center text-xs text-gray-600 dark:text-gray-400 whitespace-nowrap">
            {props.guest.vmid}
          </div>
        </td>
      </Show>

      {/* CPU */}
      <Show when={isColVisible('cpu')}>
        <td class="px-2 py-1 align-middle" style={{ "min-width": "140px" }}>
          <Show when={isMobile()}>
            <div class="md:hidden flex justify-center">
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
                showMobile={true}
              />
            </div>
          </Show>
          <div class="hidden md:block">
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
              showMobile={false}
            />
          </div>
        </td>
      </Show>

      {/* Memory */}
      <Show when={isColVisible('memory')}>
        <td class="px-2 py-1 align-middle" style={{ "min-width": "140px" }}>
          <div title={memoryTooltip() ?? undefined}>
            <Show when={isMobile()}>
              <div class="md:hidden flex justify-center">
                <ResponsiveMetricCell
                  value={memPercent()}
                  type="memory"
                  resourceId={metricsKey()}
                  sublabel={memoryUsageLabel()}
                  isRunning={isRunning()}
                  showMobile={true}
                />
              </div>
            </Show>
            <div class="hidden md:block">
              <Show
                when={viewMode() === 'sparklines'}
                fallback={
                  <StackedMemoryBar
                    used={props.guest.memory?.used || 0}
                    total={props.guest.memory?.total || 0}
                    balloon={props.guest.memory?.balloon || 0}
                    swapUsed={props.guest.memory?.swapUsed || 0}
                    swapTotal={props.guest.memory?.swapTotal || 0}
                  />
                }
              >
                <ResponsiveMetricCell
                  value={memPercent()}
                  type="memory"
                  resourceId={metricsKey()}
                  sublabel={memoryUsageLabel()}
                  isRunning={isRunning()}
                  showMobile={false}
                />
              </Show>
            </div>
          </div>
        </td>
      </Show>

      {/* Disk */}
      <Show when={isColVisible('disk')}>
        <td class="px-2 py-1 align-middle" style={{ "min-width": "140px" }}>
          <Show
            when={hasDiskUsage()}
            fallback={
              <div class="flex justify-center">
                <span class="text-xs text-gray-400 cursor-help" title={getDiskStatusTooltip()}>
                  -
                </span>
              </div>
            }
          >
            <Show when={isMobile()}>
              <div class="md:hidden flex justify-center text-xs text-gray-600 dark:text-gray-400">
                {formatPercent(diskPercent())}
              </div>
            </Show>
            <div class={isMobile() ? 'hidden md:block' : ''}>
              <Show
                when={viewMode() === 'sparklines'}
                fallback={
                  <StackedDiskBar
                    disks={props.guest.disks}
                    aggregateDisk={props.guest.disk}
                  />
                }
              >
                <ResponsiveMetricCell
                  value={diskPercent()}
                  type="disk"
                  resourceId={metricsKey()}
                  isRunning={isRunning()}
                  showMobile={false}
                />
              </Show>
            </div>
          </Show>
        </td>
      </Show>

      {/* IP Address with Network Tooltip */}
      <Show when={isColVisible('ip')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <Show when={ipAddresses().length > 0 || hasNetworkInterfaces()} fallback={<span class="text-xs text-gray-400">-</span>}>
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
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <span class={`text-xs whitespace-nowrap ${props.guest.uptime < 3600 ? 'text-orange-500' : 'text-gray-600 dark:text-gray-400'}`}>
              <Show when={isRunning()} fallback="-">
                <Show when={isMobile()} fallback={formatUptime(props.guest.uptime)}>
                  {formatUptime(props.guest.uptime, true)}
                </Show>
              </Show>
            </span>
          </div>
        </td>
      </Show>

      {/* Node - NEW */}
      <Show when={isColVisible('node')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <span class="text-xs text-gray-600 dark:text-gray-400 truncate max-w-[80px]" title={props.guest.node}>
              {props.guest.node}
            </span>
          </div>
        </td>
      </Show>

      {/* Backup Status */}
      <Show when={isColVisible('backup')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <Show when={!props.guest.template}>
              <BackupStatusCell lastBackup={props.guest.lastBackup} />
            </Show>
            <Show when={props.guest.template}>
              <span class="text-xs text-gray-400">-</span>
            </Show>
          </div>
        </td>
      </Show>

      {/* Tags */}
      <Show when={isColVisible('tags')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center" onClick={(event) => event.stopPropagation()}>
            <TagBadges
              tags={Array.isArray(props.guest.tags) ? props.guest.tags : []}
              maxVisible={2}
              onTagClick={props.onTagClick}
              activeSearch={props.activeSearch}
            />
          </div>
        </td>
      </Show>

      {/* OS */}
      <Show when={isColVisible('os')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <Show when={hasOsInfo()} fallback={<span class="text-xs text-gray-400">-</span>}>
              <OSInfoCell
                osName={osName()}
                osVersion={osVersion()}
                agentVersion={agentVersion()}
              />
            </Show>
          </div>
        </td>
      </Show>

      {/* Disk Read */}
      <Show when={isColVisible('diskRead')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center whitespace-nowrap">
            <Show when={isRunning()} fallback={<span class="text-xs text-gray-400">-</span>}>
              <span class={`text-xs ${getIOColorClass(diskRead())}`}>{formatSpeed(diskRead())}</span>
            </Show>
          </div>
        </td>
      </Show>

      {/* Disk Write */}
      <Show when={isColVisible('diskWrite')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center whitespace-nowrap">
            <Show when={isRunning()} fallback={<span class="text-xs text-gray-400">-</span>}>
              <span class={`text-xs ${getIOColorClass(diskWrite())}`}>{formatSpeed(diskWrite())}</span>
            </Show>
          </div>
        </td>
      </Show>

      {/* Net In */}
      <Show when={isColVisible('netIn')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center whitespace-nowrap">
            <Show when={isRunning()} fallback={<span class="text-xs text-gray-400">-</span>}>
              <span class={`text-xs ${getIOColorClass(networkIn())}`}>{formatSpeed(networkIn())}</span>
            </Show>
          </div>
        </td>
      </Show>

      {/* Net Out */}
      <Show when={isColVisible('netOut')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center whitespace-nowrap">
            <Show when={isRunning()} fallback={<span class="text-xs text-gray-400">-</span>}>
              <span class={`text-xs ${getIOColorClass(networkOut())}`}>{formatSpeed(networkOut())}</span>
            </Show>
          </div>
        </td>
      </Show>
    </tr>
  );
}
