import { createMemo, createSignal, createEffect, Show, For } from 'solid-js';
import type { JSX } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { VM, Container, GuestNetworkInterface } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { formatBytes, formatUptime, formatSpeed, getBackupInfo, getShortImageName, type BackupStatus } from '@/utils/format';
import { TagBadges } from './TagBadges';
import { StackedDiskBar } from './StackedDiskBar';
import { StackedMemoryBar } from './StackedMemoryBar';

import { StatusDot } from '@/components/shared/StatusDot';
import { getGuestPowerIndicator, isGuestRunning } from '@/utils/status';
import { buildMetricKey } from '@/utils/metricsKeys';
import { getWorkloadMetricsKind, resolveWorkloadType } from '@/utils/workloads';
import { type ColumnPriority } from '@/hooks/useBreakpoint';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { EnhancedCPUBar } from '@/components/Dashboard/EnhancedCPUBar';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useMetricsViewMode } from '@/stores/metricsViewMode';

import { useAlertsActivation } from '@/stores/alertsActivation';
import { useAnomalyForMetric } from '@/hooks/useAnomalies';


type Guest = WorkloadGuest;

/**
 * Get color class for I/O values based on throughput (bytes/sec)
 * Uses color intensity to indicate activity level (green/yellow/red)
 */
function getIOColorClass(bytesPerSec: number): string {
  const mbps = bytesPerSec / (1024 * 1024);
  if (mbps < 1) return 'text-gray-500 dark:text-gray-400';
  if (mbps < 10) return 'text-green-600 dark:text-green-400';
  if (mbps < 50) return 'text-yellow-600 dark:text-yellow-400';
  return 'text-red-600 dark:text-red-400';
}



const GROUPED_FIRST_CELL_INDENT = 'pl-5 sm:pl-6 lg:pl-8';
const DEFAULT_FIRST_CELL_INDENT = 'pl-4';

const buildGuestId = (guest: Guest) => {
  if (guest.id) return guest.id;
  // Canonical format: instance:node:vmid
  return `${guest.instance}:${guest.node}:${guest.vmid}`;
};

// Type guard for VM vs Container
const isVM = (guest: Guest): guest is VM => {
  return resolveWorkloadType(guest) === 'vm';
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

  const alertsActivation = useAlertsActivation();
  const backupInfo = createMemo(() => getBackupInfo(props.lastBackup, alertsActivation.getBackupThresholds()));
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
        class="inline-flex items-center gap-1 text-xs text-gray-600 dark:text-gray-400"
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
                <div class="py-1">
                  <div class="flex items-center gap-2 text-blue-400 font-medium">
                    <span>IP Addresses</span>
                    <span class="text-[9px] text-gray-500 font-normal">No agent data</span>
                  </div>
                  <div class="mt-0.5 flex flex-wrap gap-1">
                    <For each={props.ipAddresses}>
                      {(ip) => (
                        <span class="text-gray-300 font-mono">{ip}</span>
                      )}
                    </For>
                  </div>
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
        class="inline-flex items-center gap-1"
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

  const alertsActivation = useAlertsActivation();
  const info = createMemo(() => getBackupInfo(props.lastBackup, alertsActivation.getBackupThresholds()));
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
        class={`flex-shrink-0 cursor-help ${config().color}`}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        aria-label={`Backup status: ${info().status}`}
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          {/* Shield shape */}
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />

          {/* Inner icon based on status */}
          <Show when={config().icon === 'check'}>
            <path d="m9 12 2 2 4-4" />
          </Show>
          <Show when={config().icon === 'warning'}>
            <path d="M12 8v4M12 16h.01" />
          </Show>
          <Show when={config().icon === 'x'}>
            <path d="M10 10l4 4M14 10l-4 4" />
          </Show>
        </svg>
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
  icon?: JSX.Element;  // SVG icon for compact column headers
  priority: ColumnPriority;
  toggleable?: boolean;
  width?: string;  // Fixed width for consistent column sizing
  minWidth?: string;
  maxWidth?: string;
  flex?: number;
  sortKey?: string;
}

export const GUEST_COLUMNS: GuestColumnDef[] = [
  // Essential - always visible (fixed widths ensure no overlap)
  { id: 'name', label: 'Name', priority: 'essential', width: '200px', sortKey: 'name' },

  // Secondary - visible on md+ (Now essential for mobile scroll)
  { id: 'type', label: 'Type', priority: 'essential', width: '60px', sortKey: 'type' },
  { id: 'vmid', label: 'ID', priority: 'essential', width: '45px', sortKey: 'vmid' },

  // Core metrics - fixed minimum widths to prevent content overlap
  { id: 'cpu', label: 'CPU', priority: 'essential', width: '140px', sortKey: 'cpu' },
  { id: 'memory', label: 'Mem', priority: 'essential', width: '140px', sortKey: 'memory' },
  { id: 'disk', label: 'Disk', priority: 'essential', width: '140px', sortKey: 'disk' },

  // Secondary - visible on md+ (Now essential), user toggleable - use icons
  { id: 'ip', label: 'IP', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" /></svg>, priority: 'essential', width: '45px', toggleable: true },
  { id: 'uptime', label: 'Uptime', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>, priority: 'essential', width: '60px', toggleable: true, sortKey: 'uptime' },
  { id: 'node', label: 'Node', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" /></svg>, priority: 'essential', width: '70px', toggleable: true, sortKey: 'node' },

  { id: 'image', label: 'Image', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect x="3" y="6" width="18" height="12" rx="2" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h18M7 6v12M13 6v12" /></svg>, priority: 'secondary', width: '140px', minWidth: '120px', toggleable: true, sortKey: 'image' },
  { id: 'namespace', label: 'Namespace', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 2l7 4v8l-7 4-7-4V6l7-4z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v12" /></svg>, priority: 'secondary', width: '110px', minWidth: '90px', toggleable: true, sortKey: 'namespace' },

  // Supplementary - visible on lg+ (Now essential), user toggleable
  { id: 'backup', label: 'Backup', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" /></svg>, priority: 'essential', width: '50px', toggleable: true },
  { id: 'tags', label: 'Tags', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" /></svg>, priority: 'essential', width: '60px', toggleable: true },

  // Detailed - visible on xl+ (Now essential), user toggleable
  { id: 'os', label: 'OS', priority: 'essential', width: '45px', toggleable: true },
  { id: 'diskRead', label: 'D Read', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" /></svg>, priority: 'essential', width: '55px', toggleable: true, sortKey: 'diskRead' },
  { id: 'diskWrite', label: 'D Write', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" /></svg>, priority: 'essential', width: '55px', toggleable: true, sortKey: 'diskWrite' },
  { id: 'netIn', label: 'Net In', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16l-4-4m0 0l4-4m-4 4h18" /></svg>, priority: 'essential', width: '55px', toggleable: true, sortKey: 'networkIn' },
  { id: 'netOut', label: 'Net Out', icon: <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 8l4 4m0 0l-4 4m4-4H3" /></svg>, priority: 'essential', width: '55px', toggleable: true, sortKey: 'networkOut' },

  // Link column - at the end like NodeSummaryTable
  { id: 'link', label: '', priority: 'essential', width: '28px' },
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
  /** Click handler for the row */
  onClick?: () => void;
  /** Whether the row details are expanded */
  isExpanded?: boolean;
}

export function GuestRow(props: GuestRowProps) {
  const guestId = createMemo(() => buildGuestId(props.guest));

  // Use breakpoint hook directly for responsive behavior
  const { isMobile } = useBreakpoint();

  // Get current metrics view mode (bars vs sparklines)
  const { viewMode } = useMetricsViewMode();

  // PERFORMANCE: Use memoized Set for O(1) column visibility lookups instead of O(n) array.includes()
  const visibleColumnIdSet = createMemo(() =>
    props.visibleColumnIds ? new Set(props.visibleColumnIds) : null
  );

  // Helper to check if a column is visible
  // If visibleColumnIds is not provided, show all columns for backwards compatibility
  const isColVisible = (colId: string) => {
    const set = visibleColumnIdSet();
    if (!set) return true;
    return set.has(colId);
  };

  const workloadType = createMemo(() => resolveWorkloadType(props.guest));

  // Create namespaced metrics key for sparklines
  const metricsKey = createMemo(() => buildMetricKey(getWorkloadMetricsKind(props.guest), guestId()));

  // Get anomalies for this guest's metrics (deterministic, no LLM)
  const cpuAnomaly = useAnomalyForMetric(() => props.guest.id, () => 'cpu');
  const memoryAnomaly = useAnomalyForMetric(() => props.guest.id, () => 'memory');
  const diskAnomaly = useAnomalyForMetric(() => props.guest.id, () => 'disk');

  const [customUrl, setCustomUrl] = createSignal<string | undefined>(props.customUrl);

  const displayId = createMemo(() => {
    const provided = props.guest.displayId?.trim();
    if (provided) return provided;
    if (typeof props.guest.vmid === 'number' && props.guest.vmid > 0) {
      return String(props.guest.vmid);
    }
    return '';
  });

  const ipAddresses = createMemo(() => props.guest.ipAddresses ?? []);
  const networkInterfaces = createMemo(() => props.guest.networkInterfaces ?? []);
  const hasNetworkInterfaces = createMemo(() => networkInterfaces().length > 0);
  const osName = createMemo(() => props.guest.osName?.trim() ?? '');
  const osVersion = createMemo(() => props.guest.osVersion?.trim() ?? '');
  const agentVersion = createMemo(() => props.guest.agentVersion?.trim() ?? '');
  const hasOsInfo = createMemo(() => osName().length > 0 || osVersion().length > 0);
  const dockerImage = createMemo(() => props.guest.image?.trim() ?? '');
  const namespace = createMemo(() => props.guest.namespace?.trim() ?? '');
  const supportsBackup = createMemo(() => {
    const type = workloadType();
    return type === 'vm' || type === 'lxc';
  });

  const isOCIContainer = createMemo(() => {
    if (workloadType() !== 'lxc') return false;
    const container = props.guest as Container;
    return props.guest.type === 'oci' || container.isOci === true;
  });

  // OCI image info - extract clean image name from osTemplate (similar to Docker container image display)
  const ociImage = createMemo(() => {
    if (!isOCIContainer()) return null;
    const template = (props.guest as Container).osTemplate;
    if (!template) return null;
    // Strip common prefixes to get clean image reference
    let image = template;
    if (image.startsWith('oci:')) image = image.slice(4);
    if (image.startsWith('docker:')) image = image.slice(7);
    return image;
  });

  const typeInfo = createMemo(() => {
    const type = workloadType();
    if (type === 'vm') {
      return {
        label: 'VM',
        title: 'Virtual Machine',
        className: 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300',
        icon: (
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="4" width="18" height="12" rx="2" />
            <path d="M8 20h8M12 16v4" />
          </svg>
        ),
      };
    }
    if (type === 'docker') {
      return {
        label: 'Docker',
        title: 'Docker Container',
        className: 'bg-sky-100 text-sky-700 dark:bg-sky-900/50 dark:text-sky-300',
        icon: (
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="6" width="18" height="12" rx="2" />
            <path d="M3 10h18M7 6v12M13 6v12" />
          </svg>
        ),
      };
    }
    if (type === 'k8s') {
      return {
        label: 'K8s',
        title: 'Kubernetes Pod',
        className: 'bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300',
        icon: (
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M12 2l7 4v8l-7 4-7-4V6l7-4z" />
            <path d="M12 6v12" />
          </svg>
        ),
      };
    }
    if (isOCIContainer()) {
      return {
        label: 'OCI',
        title: `OCI Container${ociImage() ? ` • ${ociImage()}` : ''}`,
        className: 'bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300',
        icon: (
          <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
          </svg>
        ),
      };
    }
    return {
      label: 'LXC',
      title: 'LXC Container',
      className: 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300',
      icon: (
        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
        </svg>
      ),
    };
  });

  // Update custom URL when prop changes
  createEffect(() => {
    setCustomUrl(props.customUrl);
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
    return `${base} ${hover} ${defaultHover} ${alertBg} ${stoppedDimming}`;
  });

  const rowStyle = createMemo(() => {
    const styles: Record<string, string> = {};

    // Alert styling
    if (showAlertHighlight()) {
      const color = alertAccentColor();
      if (color) {
        styles['box-shadow'] = `inset 4px 0 0 0 ${color}`;
      }
    }

    return styles;
  });





  return (
    <>
      <tr
        class={`${rowClass()} ${props.onClick ? 'cursor-pointer' : ''}`}
        style={rowStyle()}
        data-guest-id={guestId()}
        onClick={props.onClick}
      >
        {/* Name - always visible */}
        <td class={`pr-2 py-1 align-middle whitespace-nowrap ${props.isGroupedView ? GROUPED_FIRST_CELL_INDENT : DEFAULT_FIRST_CELL_INDENT}`}>
          <div class="flex items-center gap-2 min-w-0">
            <div class={`transition-transform duration-200 ${props.isExpanded ? 'rotate-90' : ''}`}>
              <svg class="w-3.5 h-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
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
                  class="text-xs font-medium text-gray-900 dark:text-gray-100 select-none truncate"
                  title={props.guest.name}
                >
                  {props.guest.name}
                </span>
                {/* Show backup indicator in name cell only if backup column is hidden */}
                <Show when={!isColVisible('backup') && supportsBackup()}>
                  <BackupIndicator lastBackup={props.guest.lastBackup} isTemplate={props.guest.template} />
                </Show>

              </div>
            </div>



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
                class={`inline-flex items-center gap-1 px-1 py-0.5 text-[10px] font-medium rounded whitespace-nowrap ${typeInfo().className}`}
                title={typeInfo().title}
              >
                {typeInfo().icon}
                <span>{typeInfo().label}</span>
              </span>
            </div>
          </td>
        </Show>

        {/* VMID */}
        <Show when={isColVisible('vmid')}>
          <td class="px-2 py-1 align-middle">
            <div class="flex justify-center text-xs text-gray-600 dark:text-gray-400 whitespace-nowrap">
              <Show when={displayId()} fallback={<span class="text-gray-400">-</span>}>
                {displayId()}
              </Show>
            </div>
          </td>
        </Show>

        {/* CPU */}
        <Show when={isColVisible('cpu')}>
          <td class="px-2 py-1 align-middle" style={isMobile() ? { "min-width": "60px" } : { width: "140px", "min-width": "140px", "max-width": "140px" }}>
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
          <td class="px-2 py-1 align-middle" style={isMobile() ? { "min-width": "60px" } : { width: "140px", "min-width": "140px", "max-width": "140px" }}>
            <div title={memoryTooltip() ?? undefined}>
              <Show
                when={viewMode() === 'sparklines'}
                fallback={
                  <StackedMemoryBar
                    used={props.guest.memory?.used || 0}
                    total={props.guest.memory?.total || 0}
                    balloon={props.guest.memory?.balloon || 0}
                    swapUsed={props.guest.memory?.swapUsed || 0}
                    swapTotal={props.guest.memory?.swapTotal || 0}
                    resourceId={metricsKey()}
                    anomaly={memoryAnomaly()}
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
          </td>
        </Show>

        {/* Disk */}
        <Show when={isColVisible('disk')}>
          <td class="px-2 py-1 align-middle" style={isMobile() ? { "min-width": "60px" } : { width: "140px", "min-width": "140px", "max-width": "140px" }}>
            <Show
              when={hasDiskUsage()}
              fallback={
                <div class="flex justify-center">
                  <span class="text-xs text-gray-400" title={getDiskStatusTooltip()}>
                    -
                  </span>
                </div>
              }
            >
              <Show
                when={viewMode() === 'sparklines'}
                fallback={
                  <StackedDiskBar
                    disks={props.guest.disks}
                    aggregateDisk={props.guest.disk}
                    anomaly={diskAnomaly()}
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
              <Show when={props.guest.node} fallback={<span class="text-xs text-gray-400">-</span>}>
                <span class="text-xs text-gray-600 dark:text-gray-400 truncate max-w-[80px]" title={props.guest.node}>
                  {props.guest.node}
                </span>
              </Show>
            </div>
          </td>
        </Show>

        {/* Image */}
        <Show when={isColVisible('image')}>
          <td class="px-2 py-1 align-middle">
            <div class="flex justify-center">
              <Show
                when={workloadType() === 'docker' && dockerImage()}
                fallback={<span class="text-xs text-gray-400">-</span>}
              >
                <span
                  class="text-xs text-gray-600 dark:text-gray-400 truncate max-w-[140px]"
                  title={dockerImage()}
                >
                  {getShortImageName(dockerImage())}
                </span>
              </Show>
            </div>
          </td>
        </Show>

        {/* Namespace */}
        <Show when={isColVisible('namespace')}>
          <td class="px-2 py-1 align-middle">
            <div class="flex justify-center">
              <Show
                when={workloadType() === 'k8s' && namespace()}
                fallback={<span class="text-xs text-gray-400">-</span>}
              >
                <span
                  class="text-xs text-gray-600 dark:text-gray-400 truncate max-w-[120px]"
                  title={namespace()}
                >
                  {namespace()}
                </span>
              </Show>
            </div>
          </td>
        </Show>

        {/* Backup Status */}
        <Show when={isColVisible('backup')}>
          <td class="px-2 py-1 align-middle">
            <div class="flex justify-center">
              <Show when={supportsBackup()} fallback={<span class="text-xs text-gray-400">-</span>}>
                <Show when={!props.guest.template}>
                  <BackupStatusCell lastBackup={props.guest.lastBackup} />
                </Show>
                <Show when={props.guest.template}>
                  <span class="text-xs text-gray-400">-</span>
                </Show>
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
                maxVisible={0}
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
              <Show
                when={hasOsInfo()}
                fallback={
                  <Show
                    when={ociImage()}
                    fallback={<span class="text-xs text-gray-400">-</span>}
                  >
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

        {/* Link Column - at the end like NodeSummaryTable */}
        <Show when={isColVisible('link')}>
          <td class="px-0 py-1 align-middle text-center">
            <Show when={customUrl() && customUrl() !== ''} fallback={<span class="text-xs text-gray-300 dark:text-gray-700">-</span>}>
              <a
                href={customUrl()}
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex justify-center items-center text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 transition-colors"
                title={`Open ${customUrl()}`}
                onClick={(event) => event.stopPropagation()}
              >
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
              </a>
            </Show>
          </td>
        </Show>
      </tr>

    </>
  );
}
