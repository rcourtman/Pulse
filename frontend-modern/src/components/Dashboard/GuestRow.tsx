import { createMemo, createSignal, createEffect, Show, For } from 'solid-js';
import { useTooltip } from '@/hooks/useTooltip';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { useNavigate } from '@solidjs/router';
import type { VM, Container, GuestNetworkInterface } from '@/types/api';
import type { WorkloadGuest, ViewMode } from '@/types/workloads';
import {
  formatBytes,
  formatUptime,
  formatSpeed,
  getBackupInfo,
  getShortImageName,
  type BackupStatus,
} from '@/utils/format';
import { TagBadges } from './TagBadges';
import { StackedDiskBar } from './StackedDiskBar';
import { StackedMemoryBar } from './StackedMemoryBar';

import { StatusDot } from '@/components/shared/StatusDot';
import { getGuestPowerIndicator, isGuestRunning } from '@/utils/status';
import { buildMetricKey } from '@/utils/metricsKeys';
import {
  getCanonicalWorkloadId,
  getWorkloadMetricsKind,
  resolveWorkloadType,
} from '@/utils/workloads';
import { getWorkloadTypeBadge } from '@/components/shared/workloadTypeBadges';
import { buildInfrastructureHrefForWorkload } from './infrastructureLink';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { EnhancedCPUBar } from '@/components/Dashboard/EnhancedCPUBar';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { UpdateButton } from '@/components/shared/ContainerUpdateBadge';
import { getWorkloadDockerHostId } from './workloadSelectors';

import { useAlertsActivation } from '@/stores/alertsActivation';
import { useAnomalyForMetric } from '@/hooks/useAnomalies';

type Guest = WorkloadGuest;

interface IODistributionStats {
  median: number;
  mad: number;
  max: number;
  p97: number;
  p99: number;
  count: number;
}

export interface WorkloadIOEmphasis {
  network: IODistributionStats;
  diskIO: IODistributionStats;
}

const EMPTY_IO_DISTRIBUTION: IODistributionStats = {
  median: 0,
  mad: 0,
  max: 0,
  p97: 0,
  p99: 0,
  count: 0,
};
const EMPTY_IO_EMPHASIS: WorkloadIOEmphasis = {
  network: EMPTY_IO_DISTRIBUTION,
  diskIO: EMPTY_IO_DISTRIBUTION,
};

interface IOEmphasis {
  className: string;
  showOutlierHint: boolean;
}

const getOutlierEmphasis = (value: number, stats: IODistributionStats): IOEmphasis => {
  if (!Number.isFinite(value) || value <= 0 || stats.max <= 0) {
    return { className: 'text-muted', showOutlierHint: false };
  }

  if (stats.count < 4) {
    const ratio = value / stats.max;
    if (ratio >= 0.995) {
      return { className: 'text-base-content font-medium', showOutlierHint: true };
    }
    return { className: 'text-muted', showOutlierHint: false };
  }

  if (stats.mad > 0) {
    const modifiedZ = (0.6745 * (value - stats.median)) / stats.mad;
    if (modifiedZ >= 6.5 && value >= stats.p99) {
      return { className: 'text-base-content font-semibold', showOutlierHint: true };
    }
    if (modifiedZ >= 5.5 && value >= stats.p97) {
      return { className: 'text-base-content font-medium', showOutlierHint: true };
    }
    return { className: 'text-muted', showOutlierHint: false };
  }

  if (value >= stats.p99)
    return { className: 'text-base-content font-semibold', showOutlierHint: true };
  if (value >= stats.p97)
    return { className: 'text-base-content font-medium', showOutlierHint: true };
  if (value > 0) return { className: 'text-muted', showOutlierHint: false };
  return { className: 'text-muted', showOutlierHint: false };
};

const GROUPED_FIRST_CELL_INDENT = 'pl-3 sm:pl-5 lg:pl-8';
const DEFAULT_FIRST_CELL_INDENT = 'pl-2 sm:pl-3';

const buildGuestId = (guest: Guest) => {
  return getCanonicalWorkloadId(guest);
};

// Type guard for VM vs Container
const isVM = (guest: Guest): guest is VM => {
  return resolveWorkloadType(guest) === 'vm';
};

// Backup status indicator colors and icons
const BACKUP_STATUS_CONFIG: Record<
  BackupStatus,
  { color: string; bgColor: string; icon: 'check' | 'warning' | 'x' }
> = {
  fresh: {
    color: 'text-green-600 dark:text-green-400',
    bgColor: 'bg-green-100 dark:bg-green-900',
    icon: 'check',
  },
  stale: {
    color: 'text-yellow-600 dark:text-yellow-400',
    bgColor: 'bg-yellow-100 dark:bg-yellow-900',
    icon: 'warning',
  },
  critical: {
    color: 'text-red-600 dark:text-red-400',
    bgColor: 'bg-red-100 dark:bg-red-900',
    icon: 'x',
  },
  never: { color: 'text-muted', bgColor: 'bg-surface-alt', icon: 'x' },
};

function BackupIndicator(props: {
  lastBackup: string | number | null | undefined;
  isTemplate: boolean;
}) {
  // Don't show for templates
  if (props.isTemplate) return null;

  const alertsActivation = useAlertsActivation();
  const backupInfo = createMemo(() =>
    getBackupInfo(props.lastBackup, alertsActivation.getBackupThresholds()),
  );
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
      <span class={`flex-shrink-0 ${config().color}`} title={tooltipText()}>
        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
          {/* Shield shape */}
          <path
            d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          {/* Inner icon based on status */}
          <Show when={config().icon === 'warning'}>
            <path
              d="M12 8v4M12 16h.01"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </Show>
          <Show when={config().icon === 'x'}>
            <path
              d="M10 10l4 4M14 10l-4 4"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </Show>
        </svg>
      </span>
    </Show>
  );
}

// Network info cell with rich tooltip showing interfaces, IPs, and MACs
function NetworkInfoCell(props: {
  ipAddresses: string[];
  networkInterfaces: GuestNetworkInterface[];
}) {
  const tip = useTooltip();

  const hasInterfaces = () => props.networkInterfaces.length > 0;
  const primaryIp = () =>
    props.ipAddresses[0] || props.networkInterfaces[0]?.addresses?.[0] || null;
  const totalIps = () => {
    if (props.ipAddresses.length > 0) return props.ipAddresses.length;
    return props.networkInterfaces.reduce((sum, iface) => sum + (iface.addresses?.length || 0), 0);
  };

  return (
    <>
      <span
        class="inline-flex items-center gap-1 text-xs text-muted"
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
      >
        <Show when={primaryIp()} fallback="-">
          {/* Network icon */}
          <svg
            class="w-3.5 h-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418"
            />
          </svg>
          <span class="text-[10px] font-medium">{totalIps()}</span>
        </Show>
      </span>

      <TooltipPortal
        when={tip.show() && (hasInterfaces() || props.ipAddresses.length > 0)}
        x={tip.pos().x}
        y={tip.pos().y}
      >
        <div class="min-w-[180px] max-w-[280px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">
            Network Interfaces
          </div>

          <Show when={hasInterfaces()}>
            <For each={props.networkInterfaces}>
              {(iface, idx) => (
                <div class="py-1" classList={{ 'border-t border-border': idx() > 0 }}>
                  <div class="flex items-center gap-2 text-blue-400 font-medium">
                    <span>{iface.name || 'eth' + idx()}</span>
                    <Show when={iface.mac}>
                      <span class="text-[9px] text-slate-500 font-normal">{iface.mac}</span>
                    </Show>
                  </div>
                  <Show when={iface.addresses && iface.addresses.length > 0}>
                    <div class="mt-0.5 flex flex-wrap gap-1">
                      <For each={iface.addresses}>
                        {(ip) => <span class="text-slate-300 font-mono">{ip}</span>}
                      </For>
                    </div>
                  </Show>
                  <Show when={!iface.addresses || iface.addresses.length === 0}>
                    <span class="text-slate-500 text-[9px]">No IP assigned</span>
                  </Show>
                  <Show when={(iface.rxBytes || 0) > 0 || (iface.txBytes || 0) > 0}>
                    <div class="mt-0.5 text-[9px] text-slate-500">
                      RX: {formatBytes(iface.rxBytes || 0)} / TX: {formatBytes(iface.txBytes || 0)}
                    </div>
                  </Show>
                </div>
              )}
            </For>
          </Show>

          <Show when={!hasInterfaces() && props.ipAddresses.length > 0}>
            <div class="py-1">
              <div class="flex items-center gap-2 text-blue-400 font-medium">
                <span>IP Addresses</span>
                <span class="text-[9px] text-slate-500 font-normal">No agent data</span>
              </div>
              <div class="mt-0.5 flex flex-wrap gap-1">
                <For each={props.ipAddresses}>
                  {(ip) => <span class="text-slate-300 font-mono">{ip}</span>}
                </For>
              </div>
            </div>
          </Show>
        </div>
      </TooltipPortal>
    </>
  );
}

// OS detection helper - simplified to just Linux vs Windows
type OSType = 'windows' | 'linux' | 'unknown';

function detectOSType(osName: string): OSType {
  const lower = osName.toLowerCase();
  if (lower.includes('windows')) return 'windows';
  // All Linux distros, BSDs, and Unix-likes -> linux
  if (
    lower.includes('linux') ||
    lower.includes('debian') ||
    lower.includes('ubuntu') ||
    lower.includes('alpine') ||
    lower.includes('centos') ||
    lower.includes('fedora') ||
    lower.includes('arch') ||
    lower.includes('nixos') ||
    lower.includes('suse') ||
    lower.includes('gentoo') ||
    lower.includes('rhel') ||
    lower.includes('rocky') ||
    lower.includes('alma') ||
    lower.includes('devuan') ||
    lower.includes('gnu') ||
    lower.includes('freebsd') ||
    lower.includes('openbsd') ||
    lower.includes('netbsd')
  ) {
    return 'linux';
  }
  return 'unknown';
}

// OS info cell with icon and Portal tooltip
function OSInfoCell(props: { osName: string; osVersion: string; agentVersion: string }) {
  const tip = useTooltip();
  const osType = createMemo(() => detectOSType(props.osName));

  // OS icons - Windows logo and terminal prompt for Linux
  const OSIcon = () => {
    const type = osType();
    const iconClass = 'w-3.5 h-3.5 text-muted';

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
          <svg
            class={iconClass}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <polyline points="4 17 10 11 4 5" />
            <line x1="12" y1="19" x2="20" y2="19" />
          </svg>
        );
      default:
        return <span class="text-slate-400">—</span>;
    }
  };

  return (
    <>
      <span
        class="inline-flex items-center gap-1"
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
      >
        <OSIcon />
      </span>

      <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
        <div class="min-w-[120px] max-w-[220px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">
            Operating System
          </div>
          <div class="py-0.5">
            <div class="text-base-content font-medium">{props.osName}</div>
            <Show when={props.osVersion}>
              <div class="text-slate-400">Version: {props.osVersion}</div>
            </Show>
            <Show when={props.agentVersion}>
              <div class="text-slate-500 text-[9px] mt-1 pt-1 border-t border-border">
                Agent: {props.agentVersion}
              </div>
            </Show>
          </div>
        </div>
      </TooltipPortal>
    </>
  );
}

// Backup status cell with Portal tooltip
function BackupStatusCell(props: { lastBackup: string | number | null | undefined }) {
  const tip = useTooltip();

  const alertsActivation = useAlertsActivation();
  const info = createMemo(() =>
    getBackupInfo(props.lastBackup, alertsActivation.getBackupThresholds()),
  );
  const config = createMemo(() => BACKUP_STATUS_CONFIG[info().status]);

  return (
    <>
      <span
        class={`flex-shrink-0 cursor-help ${config().color}`}
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
        aria-label={`Backup status: ${info().status}`}
      >
        <svg
          class="w-4 h-4"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
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

      <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
        <div class="min-w-[140px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">
            Backup Status
          </div>
          <Show when={info().status !== 'never'}>
            <div class="py-0.5">
              <div class="text-slate-400">Last backup</div>
              <div class="text-base-content font-medium">
                {new Date(props.lastBackup!).toLocaleDateString(undefined, {
                  weekday: 'short',
                  year: 'numeric',
                  month: 'short',
                  day: 'numeric',
                })}
              </div>
              <div class="text-slate-300">{new Date(props.lastBackup!).toLocaleTimeString()}</div>
            </div>
            <div class="pt-1 mt-1 border-t border-border">
              <span class={config().color}>{info().ageFormatted} ago</span>
            </div>
          </Show>
          <Show when={info().status === 'never'}>
            <div class="py-0.5 text-red-400">No backup has ever been recorded for this guest.</div>
          </Show>
        </div>
      </TooltipPortal>
    </>
  );
}

// Info cell (VMID / image / namespace) with Portal tooltip for truncated values
function InfoTooltipCell(props: { value: string; tooltip: string; type: string }) {
  const tip = useTooltip();

  const label = createMemo(() => {
    if (props.type === 'docker') return 'Image';
    if (props.type === 'k8s') return 'Namespace';
    return 'ID';
  });

  return (
    <>
      <span
        class="truncate max-w-[100px] cursor-help"
        onMouseEnter={tip.onMouseEnter}
        onMouseLeave={tip.onMouseLeave}
      >
        {props.value}
      </span>

      <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
        <div class="max-w-[280px]">
          <div class="font-medium mb-1 text-slate-300 border-b border-border pb-1">{label()}</div>
          <div class="py-0.5 text-base-content break-all">{props.tooltip}</div>
        </div>
      </TooltipPortal>
    </>
  );
}

// Column configuration using the priority system
export const GUEST_COLUMNS: ColumnDef[] = [
  { id: 'name', label: 'Name', width: '200px', sortKey: 'name' },
  { id: 'type', label: 'Type', width: '60px', sortKey: 'type' },
  { id: 'info', label: 'Info', width: '100px' }, // Merged identifier: VMID for VMs/LXCs, image for Docker, namespace for K8s
  { id: 'vmid', label: 'ID', width: '45px', sortKey: 'vmid' },

  // Core metrics
  { id: 'cpu', label: 'CPU', width: '140px', sortKey: 'cpu' },
  { id: 'memory', label: 'Mem', width: '140px', sortKey: 'memory' },
  { id: 'disk', label: 'Disk', width: '140px', sortKey: 'disk' },

  // Toggleable columns
  {
    id: 'ip',
    label: 'IP',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"
        />
      </svg>
    ),
    width: '45px',
    toggleable: true,
  },
  {
    id: 'uptime',
    label: 'Uptime',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    ),
    width: '60px',
    toggleable: true,
    sortKey: 'uptime',
  },
  {
    id: 'node',
    label: 'Node',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"
        />
      </svg>
    ),
    width: '70px',
    toggleable: true,
    sortKey: 'node',
  },
  {
    id: 'image',
    label: 'Image',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <rect x="3" y="6" width="18" height="12" rx="2" />
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M3 10h18M7 6v12M13 6v12"
        />
      </svg>
    ),
    width: '140px',
    minWidth: '120px',
    toggleable: true,
    sortKey: 'image',
  },
  {
    id: 'namespace',
    label: 'Namespace',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 2l7 4v8l-7 4-7-4V6l7-4z"
        />
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v12" />
      </svg>
    ),
    width: '110px',
    minWidth: '90px',
    toggleable: true,
    sortKey: 'namespace',
  },
  {
    id: 'context',
    label: 'Context',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 6v6m0 6h.01M4 6a8 8 0 018-4 8 8 0 018 4M4 18a8 8 0 008 4 8 8 0 008-4"
        />
      </svg>
    ),
    width: '120px',
    minWidth: '100px',
    toggleable: true,
    sortKey: 'contextLabel',
  },
  {
    id: 'backup',
    label: 'Backup',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
        />
      </svg>
    ),
    width: '50px',
    toggleable: true,
  },
  {
    id: 'tags',
    label: 'Tags',
    icon: (
      <svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"
        />
      </svg>
    ),
    width: '60px',
    toggleable: true,
  },
  { id: 'update', label: 'Update', width: '60px', toggleable: true },
  { id: 'os', label: 'OS', width: '45px', toggleable: true },
  {
    id: 'netIo',
    label: 'Net I/O',
    width: '130px',
    minWidth: '120px',
    toggleable: true,
    sortKey: 'netIo',
  },
  {
    id: 'diskIo',
    label: 'Disk I/O',
    width: '130px',
    minWidth: '120px',
    toggleable: true,
    sortKey: 'diskIo',
  },

  // Link column
  { id: 'link', label: '', width: '28px' },
];

/**
 * Columns relevant to each ViewMode.
 * - "all" uses the merged `info` column instead of vmid/image/namespace
 * - Type-specific views use the dedicated columns for that type
 */
export const VIEW_MODE_COLUMNS: Record<ViewMode, Set<string> | null> = {
  all: new Set([
    'name',
    'type',
    'info',
    'cpu',
    'memory',
    'disk',
    'ip',
    'uptime',
    'node',
    'backup',
    'tags',
    'os',
    'diskIo',
    'netIo',
    'link',
  ]),
  vm: new Set([
    'name',
    'vmid',
    'cpu',
    'memory',
    'disk',
    'ip',
    'uptime',
    'node',
    'backup',
    'tags',
    'os',
    'diskIo',
    'netIo',
    'link',
  ]),
  lxc: new Set([
    'name',
    'vmid',
    'cpu',
    'memory',
    'disk',
    'ip',
    'uptime',
    'node',
    'backup',
    'tags',
    'os',
    'diskIo',
    'netIo',
    'link',
  ]),
  docker: new Set([
    'name',
    'cpu',
    'memory',
    'uptime',
    'image',
    'context',
    'tags',
    'update',
    'link',
  ]),
  k8s: new Set(['name', 'cpu', 'memory', 'image', 'namespace', 'context', 'link']),
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
  isGroupedView?: boolean;
  /** IDs of columns that should be visible */
  visibleColumnIds?: string[];
  /** Click handler for the row */
  onClick?: () => void;
  /** Whether the row details are expanded */
  isExpanded?: boolean;
  /** Optional cross-row I/O emphasis stats for relative highlighting */
  ioEmphasis?: WorkloadIOEmphasis;
  /** Called when this row is hovered/unhovered */
  onHoverChange?: (guestId: string | null) => void;
}

export function GuestRow(props: GuestRowProps) {
  const navigate = useNavigate();
  const guestId = createMemo(() => buildGuestId(props.guest));
  const infrastructureHref = createMemo(() => buildInfrastructureHrefForWorkload(props.guest));

  // Use breakpoint hook directly for responsive behavior
  const { isMobile } = useBreakpoint();

  // PERFORMANCE: Use memoized Set for O(1) column visibility lookups instead of O(n) array.includes()
  const visibleColumnIdSet = createMemo(() =>
    props.visibleColumnIds ? new Set(props.visibleColumnIds) : null,
  );

  // Helper to check if a column is visible
  // If visibleColumnIds is not provided, show all columns for backwards compatibility
  const isColVisible = (colId: string) => {
    const set = visibleColumnIdSet();
    if (!set) return true;
    return set.has(colId);
  };

  const workloadType = createMemo(() => resolveWorkloadType(props.guest));

  // Create a stable key for per-resource metric correlation.
  const metricsKey = createMemo(() =>
    buildMetricKey(getWorkloadMetricsKind(props.guest), guestId()),
  );

  // Get anomalies for this guest's metrics (deterministic, no LLM)
  const cpuAnomaly = useAnomalyForMetric(guestId, () => 'cpu');
  const memoryAnomaly = useAnomalyForMetric(guestId, () => 'memory');
  const diskAnomaly = useAnomalyForMetric(guestId, () => 'disk');

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
  const contextLabel = createMemo(() => props.guest.contextLabel?.trim() ?? '');
  const clusterName = createMemo(() => props.guest.clusterName?.trim() ?? '');
  const isPveWorkload = createMemo(() => {
    const t = workloadType();
    return t === 'vm' || t === 'lxc';
  });

  // Merged identifier for the "all" view — shows the most relevant ID per type
  const infoValue = createMemo(() => {
    const type = workloadType();
    if (type === 'vm' || type === 'lxc') return displayId();
    if (type === 'docker') return dockerImage() ? getShortImageName(dockerImage()) : '';
    if (type === 'k8s') return namespace();
    return '';
  });
  const infoTooltip = createMemo(() => {
    const type = workloadType();
    if (type === 'docker') return dockerImage(); // Full image name in tooltip
    return infoValue();
  });
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
    if (isOCIContainer()) {
      return getWorkloadTypeBadge('oci', {
        title: `OCI Container${ociImage() ? ` • ${ociImage()}` : ''}`,
      });
    }
    if (workloadType() === 'docker') {
      const runtime = (props.guest.containerRuntime || '').trim();
      const normalized = runtime.toLowerCase();
      const label =
        normalized === 'podman'
          ? 'Podman'
          : normalized === 'docker'
            ? 'Docker'
            : runtime
              ? runtime
              : 'Containers';
      const title =
        normalized === 'podman'
          ? 'Podman Container'
          : normalized === 'docker'
            ? 'Docker Container'
            : runtime
              ? `${runtime} Container`
              : 'Container (Docker-compatible runtime)';
      return getWorkloadTypeBadge('docker', { label, title });
    }
    return getWorkloadTypeBadge(workloadType());
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
  const ioEmphasis = createMemo(() => props.ioEmphasis ?? EMPTY_IO_EMPHASIS);
  const diskIOTotal = createMemo(() => diskRead() + diskWrite());
  const networkTotal = createMemo(() => networkIn() + networkOut());
  const diskIOEmphasis = createMemo(() => getOutlierEmphasis(diskIOTotal(), ioEmphasis().diskIO));
  const networkEmphasis = createMemo(() =>
    getOutlierEmphasis(networkTotal(), ioEmphasis().network),
  );

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
  const memoryPercentOnly = createMemo(() => {
    const memory = props.guest.memory;
    if (!memory) return undefined;
    if ((memory.total ?? 0) > 0) return undefined;
    const usage = memory.usage ?? 0;
    if (!Number.isFinite(usage) || usage <= 0) return undefined;
    return Math.max(0, Math.min(usage, 100));
  });

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
    const base = 'transition-all duration-200 relative group cursor-pointer';

    if (props.isExpanded) {
      return `${base} bg-blue-50 dark:bg-blue-900 z-10 hover:shadow-sm`;
    }

    const hover = 'hover:shadow-sm';
    const alertBg = hasUnacknowledgedAlert()
      ? props.alertStyles?.severity === 'critical'
        ? 'bg-red-50 dark:bg-red-950'
        : 'bg-yellow-50 dark:bg-yellow-950'
      : '';
    const defaultHover = hasUnacknowledgedAlert() ? '' : 'hover:bg-surface-hover';
    const stoppedDimming = !isRunning() ? 'opacity-60' : '';

    return [base, hover, defaultHover, alertBg, stoppedDimming].filter(Boolean).join(' ');
  });

  const rowStyle = createMemo(() => {
    const styles: Record<string, string> = {
      'min-height': '32px',
    };

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
        class={`${rowClass()} ${props.onClick ? 'cursor-pointer group' : ''}`}
        style={rowStyle()}
        data-guest-id={guestId()}
        onClick={props.onClick}
        onMouseEnter={() => props.onHoverChange?.(guestId())}
        onMouseLeave={() => props.onHoverChange?.(null)}
      >
        {/* Name - always visible */}
        <td
          class={`pr-1.5 sm:pr-2 py-0.5 align-middle whitespace-nowrap ${props.isGroupedView ? GROUPED_FIRST_CELL_INDENT : DEFAULT_FIRST_CELL_INDENT}`}
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
                when={workloadType() === 'docker' && dockerImage()}
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
                when={workloadType() === 'k8s' && namespace()}
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
              <Show when={props.guest.type === 'docker' && getWorkloadDockerHostId(props.guest)}>
                <UpdateButton
                  updateStatus={props.guest.updateStatus}
                  hostId={getWorkloadDockerHostId(props.guest)}
                  containerId={props.guest.id ?? ''}
                  containerName={props.guest.name}
                  compact={true}
                />
              </Show>
            </div>
          </td>
        </Show>

        {/* Link Column - at the end like NodeSummaryTable */}
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
