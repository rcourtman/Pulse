import type { Component } from 'solid-js';
import { For, Show, createMemo, createSignal, onMount, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import { useNavigate } from '@solidjs/router';
import type { Host, HostRAIDArray } from '@/types/api';
import { formatBytes, formatRelativeTime, formatUptime } from '@/utils/format';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { StackedDiskBar } from '@/components/Dashboard/StackedDiskBar';
import { HostsFilter } from './HostsFilter';
import { useWebSocket } from '@/App';
import { StatusDot } from '@/components/shared/StatusDot';
import { getHostStatusIndicator } from '@/utils/status';
import { MetricText } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Dashboard/StackedMemoryBar';
import { EnhancedCPUBar } from '@/components/Dashboard/EnhancedCPUBar';
import { useBreakpoint, type ColumnPriority } from '@/hooks/useBreakpoint';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import { aiChatStore } from '@/stores/aiChat';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { useResourcesAsLegacy } from '@/hooks/useResources';

// Column definition for hosts table
export interface HostColumnDef {
  id: string;
  label: string;
  icon?: string;
  priority: ColumnPriority;
  toggleable?: boolean;
  width?: string;
  sortKey?: string;
}

// Host table column definitions
export const HOST_COLUMNS: HostColumnDef[] = [
  // Essential - always visible
  { id: 'name', label: 'Host', priority: 'essential', width: '140px', sortKey: 'name' },
  { id: 'platform', label: 'Platform', priority: 'essential', width: '90px', sortKey: 'platform' },
  { id: 'cpu', label: 'CPU', priority: 'essential', width: '140px', sortKey: 'cpu' },
  { id: 'memory', label: 'Memory', priority: 'essential', width: '140px', sortKey: 'memory' },
  { id: 'disk', label: 'Disk', priority: 'essential', width: '140px', sortKey: 'disk' },

  // Secondary - visible on md+, toggleable
  { id: 'temp', label: 'Temp', icon: '<svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/></svg>', priority: 'secondary', width: '50px', toggleable: true },
  { id: 'uptime', label: 'Uptime', icon: '<svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>', priority: 'secondary', width: '65px', toggleable: true, sortKey: 'uptime' },
  { id: 'agent', label: 'Agent', priority: 'secondary', width: '60px', toggleable: true },

  // Supplementary - visible on lg+, toggleable
  // Note: CPU count and load average removed - they're shown in the EnhancedCPUBar tooltip
  { id: 'ip', label: 'IP', icon: '<svg class="w-3.5 h-3.5 block" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"/></svg>', priority: 'supplementary', width: '50px', toggleable: true },

  // Detailed - visible on xl+, toggleable
  { id: 'arch', label: 'Arch', priority: 'detailed', width: '55px', toggleable: true },
  { id: 'kernel', label: 'Kernel', priority: 'detailed', width: '120px', toggleable: true },
  { id: 'raid', label: 'RAID', priority: 'detailed', width: '60px', toggleable: true },
];

// Network info cell with rich tooltip showing interfaces, IPs, and traffic (matches GuestRow pattern)
interface NetworkInterface {
  name: string;
  addresses?: string[];
  mac?: string;
  rxBytes?: number;
  txBytes?: number;
}

function HostNetworkInfoCell(props: { networkInterfaces: NetworkInterface[] }) {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

  const hasInterfaces = () => props.networkInterfaces.length > 0;

  const totalIps = () => {
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
        <Show when={hasInterfaces()} fallback={<span class="text-gray-400">—</span>}>
          {/* Network globe icon */}
          <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" />
          </svg>
          <span class="text-[10px] font-medium">{totalIps()}</span>
        </Show>
      </span>

      <Show when={showTooltip() && hasInterfaces()}>
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
            </div>
          </div>
        </Portal>
      </Show>
    </>
  );
}

// Temperature cell with rich tooltip showing all sensor readings
function HostTemperatureCell(props: { sensors: Record<string, number> | null | undefined }) {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

  // Get the primary (highest) temperature for display
  const primaryTemp = createMemo(() => {
    if (!props.sensors) return null;
    const temps = Object.values(props.sensors);
    if (temps.length === 0) return null;
    // Find package/composite temp first, otherwise show max
    const keys = Object.keys(props.sensors);
    const packageKey = keys.find(k =>
      k.toLowerCase().includes('package') ||
      k.toLowerCase().includes('composite') ||
      k.toLowerCase().includes('tctl')
    );
    if (packageKey) return props.sensors[packageKey];
    return Math.max(...temps);
  });

  const hasSensors = () => props.sensors && Object.keys(props.sensors).length > 0;

  // Color based on temperature
  const textColorClass = createMemo(() => {
    const temp = primaryTemp();
    if (temp === null) return 'text-gray-400';
    if (temp >= 80) return 'text-red-600 dark:text-red-400';
    if (temp >= 70) return 'text-yellow-600 dark:text-yellow-400';
    return 'text-gray-600 dark:text-gray-400';
  });

  const handleMouseEnter = (e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
    setShowTooltip(true);
  };

  const handleMouseLeave = () => {
    setShowTooltip(false);
  };

  // Sort sensors: package/composite first, then cores, then others
  const sortedSensors = createMemo(() => {
    if (!props.sensors) return [];
    return Object.entries(props.sensors).sort(([a], [b]) => {
      const aLower = a.toLowerCase();
      const bLower = b.toLowerCase();
      // Package/composite/tctl first
      const aIsPrimary = aLower.includes('package') || aLower.includes('composite') || aLower.includes('tctl');
      const bIsPrimary = bLower.includes('package') || bLower.includes('composite') || bLower.includes('tctl');
      if (aIsPrimary && !bIsPrimary) return -1;
      if (bIsPrimary && !aIsPrimary) return 1;
      // Then cores by number
      const aCoreMatch = aLower.match(/core\s*(\d+)/);
      const bCoreMatch = bLower.match(/core\s*(\d+)/);
      if (aCoreMatch && bCoreMatch) {
        return parseInt(aCoreMatch[1]) - parseInt(bCoreMatch[1]);
      }
      return a.localeCompare(b);
    });
  });

  return (
    <>
      <span
        class={`inline-flex items-center text-xs whitespace-nowrap ${textColorClass()} ${hasSensors() ? 'cursor-help' : ''}`}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        <Show when={primaryTemp() !== null} fallback={<span class="text-gray-400">—</span>}>
          {Math.round(primaryTemp()!)}°C
        </Show>
      </span>

      <Show when={showTooltip() && hasSensors()}>
        <Portal mount={document.body}>
          <div
            class="fixed z-[9999] pointer-events-none"
            style={{
              left: `${tooltipPos().x}px`,
              top: `${tooltipPos().y - 8}px`,
              transform: 'translate(-50%, -100%)',
            }}
          >
            <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2 py-1.5 min-w-[160px] max-w-[240px] border border-gray-700">
              <div class="font-medium mb-1 text-gray-300 border-b border-gray-700 pb-1">
                Temperature Sensors
              </div>

              <div class="space-y-0.5">
                <For each={sortedSensors()}>
                  {([name, temp]) => {
                    const colorClass = temp >= 80 ? 'text-red-400' : temp >= 70 ? 'text-yellow-400' : 'text-gray-200';
                    return (
                      <div class="flex justify-between gap-3 py-0.5">
                        <span class="text-gray-400 truncate max-w-[120px]">{name}</span>
                        <span class={`font-medium font-mono ${colorClass}`}>{Math.round(temp)}°C</span>
                      </div>
                    );
                  }}
                </For>
              </div>
            </div>
          </div>
        </Portal>
      </Show>
    </>
  );
}

// RAID status cell with rich tooltip showing array details
interface HostRAIDStatusCellProps {
  raid: HostRAIDArray[] | undefined;
}

function HostRAIDStatusCell(props: HostRAIDStatusCellProps) {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

  const hasArrays = () => props.raid && props.raid.length > 0;

  // Analyze overall status
  const status = createMemo(() => {
    if (!props.raid || props.raid.length === 0) {
      return { type: 'none' as const, label: '-', color: 'text-gray-400' };
    }

    let hasDegraded = false;
    let hasRebuilding = false;
    let maxRebuildPercent = 0;

    for (const array of props.raid) {
      const state = array.state.toLowerCase();
      if (state.includes('degraded') || array.failedDevices > 0) {
        hasDegraded = true;
      }
      if (state.includes('recover') || state.includes('resync') || array.rebuildPercent > 0) {
        hasRebuilding = true;
        maxRebuildPercent = Math.max(maxRebuildPercent, array.rebuildPercent);
      }
    }

    if (hasDegraded) {
      return { type: 'degraded' as const, label: 'Degraded', color: 'text-red-600 dark:text-red-400' };
    }
    if (hasRebuilding) {
      return {
        type: 'rebuilding' as const,
        label: `${Math.round(maxRebuildPercent)}%`,
        color: 'text-amber-600 dark:text-amber-400'
      };
    }
    return { type: 'ok' as const, label: 'OK', color: 'text-green-600 dark:text-green-400' };
  });

  const handleMouseEnter = (e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
    setShowTooltip(true);
  };

  const handleMouseLeave = () => {
    setShowTooltip(false);
  };

  // Get state color for individual devices
  const getDeviceStateColor = (state: string) => {
    const s = state.toLowerCase();
    if (s.includes('active') || s.includes('sync')) return 'text-green-400';
    if (s.includes('spare')) return 'text-blue-400';
    if (s.includes('faulty') || s.includes('removed')) return 'text-red-400';
    if (s.includes('rebuilding')) return 'text-amber-400';
    return 'text-gray-400';
  };

  // Get array state color
  const getArrayStateColor = (array: HostRAIDArray) => {
    const state = array.state.toLowerCase();
    if (state.includes('degraded') || array.failedDevices > 0) return 'text-red-400';
    if (state.includes('recover') || state.includes('resync') || array.rebuildPercent > 0) return 'text-amber-400';
    if (state.includes('clean') || state.includes('active')) return 'text-green-400';
    return 'text-gray-400';
  };

  return (
    <>
      <span
        class={`inline-flex items-center gap-1 text-xs whitespace-nowrap ${status().color} ${hasArrays() ? 'cursor-help' : ''}`}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        <Show when={hasArrays()} fallback={<span class="text-gray-400">—</span>}>
          {/* Status icon */}
          <Show when={status().type === 'ok'}>
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </Show>
          <Show when={status().type === 'degraded'}>
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
            </svg>
          </Show>
          <Show when={status().type === 'rebuilding'}>
            <svg class="w-3 h-3 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99" />
            </svg>
          </Show>
          <span class="text-[10px] font-medium">
            {props.raid!.length > 1 ? `${props.raid!.length} ` : ''}{status().label}
          </span>
        </Show>
      </span>

      <Show when={showTooltip() && hasArrays()}>
        <Portal mount={document.body}>
          <div
            class="fixed z-[9999] pointer-events-none"
            style={{
              left: `${tooltipPos().x}px`,
              top: `${tooltipPos().y - 8}px`,
              transform: 'translate(-50%, -100%)',
            }}
          >
            <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2.5 py-2 min-w-[200px] max-w-[320px] border border-gray-700">
              <div class="font-medium mb-1.5 text-gray-300 border-b border-gray-700 pb-1 flex items-center gap-1.5">
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z" />
                </svg>
                RAID Arrays ({props.raid!.length})
              </div>

              <div class="space-y-2">
                <For each={props.raid}>
                  {(array) => (
                    <div class="border-b border-gray-700/50 pb-1.5 last:border-0 last:pb-0">
                      {/* Array header */}
                      <div class="flex items-center justify-between gap-2 mb-1">
                        <div class="flex items-center gap-1.5">
                          <span class="font-mono text-blue-400">{array.device}</span>
                          <span class="text-gray-500 uppercase text-[9px]">{array.level}</span>
                        </div>
                        <span class={`font-medium capitalize ${getArrayStateColor(array)}`}>
                          {array.state}
                        </span>
                      </div>

                      {/* Array name if present */}
                      <Show when={array.name}>
                        <div class="text-[9px] text-gray-500 mb-1">{array.name}</div>
                      </Show>

                      {/* Device counts */}
                      <div class="flex flex-wrap gap-x-2 gap-y-0.5 text-[9px] text-gray-400 mb-1">
                        <span>Active: <span class="text-green-400">{array.activeDevices}</span></span>
                        <span>Working: <span class="text-gray-200">{array.workingDevices}</span></span>
                        <Show when={array.spareDevices > 0}>
                          <span>Spare: <span class="text-blue-400">{array.spareDevices}</span></span>
                        </Show>
                        <Show when={array.failedDevices > 0}>
                          <span>Failed: <span class="text-red-400">{array.failedDevices}</span></span>
                        </Show>
                      </div>

                      {/* Rebuild progress */}
                      <Show when={array.rebuildPercent > 0}>
                        <div class="mb-1">
                          <div class="flex items-center justify-between text-[9px] mb-0.5">
                            <span class="text-amber-400">Rebuilding</span>
                            <span class="text-gray-300">{array.rebuildPercent.toFixed(1)}%</span>
                          </div>
                          <div class="h-1 bg-gray-700 rounded-full overflow-hidden">
                            <div
                              class="h-full bg-amber-500 transition-all duration-300"
                              style={{ width: `${array.rebuildPercent}%` }}
                            />
                          </div>
                          <Show when={array.rebuildSpeed}>
                            <div class="text-[9px] text-gray-500 mt-0.5">
                              Speed: {array.rebuildSpeed}
                            </div>
                          </Show>
                        </div>
                      </Show>

                      {/* Individual devices */}
                      <Show when={array.devices && array.devices.length > 0}>
                        <div class="flex flex-wrap gap-1 mt-1">
                          <For each={array.devices}>
                            {(dev) => (
                              <span
                                class={`font-mono text-[9px] px-1 py-0.5 rounded bg-gray-800 ${getDeviceStateColor(dev.state)}`}
                                title={`${dev.device} - ${dev.state}`}
                              >
                                {dev.device.replace('/dev/', '')}
                              </span>
                            )}
                          </For>
                        </div>
                      </Show>
                    </div>
                  )}
                </For>
              </div>
            </div>
          </div>
        </Portal>
      </Show>
    </>
  );
}

type SortKey = 'name' | 'platform' | 'cpu' | 'memory' | 'disk' | 'uptime';

// eslint-disable-next-line @typescript-eslint/no-empty-interface
interface HostsOverviewProps { }

export const HostsOverview: Component<HostsOverviewProps> = () => {
  const navigate = useNavigate();
  const wsContext = useWebSocket();
  const [search, setSearch] = createSignal('');
  const [statusFilter, setStatusFilter] = createSignal<'all' | 'online' | 'degraded' | 'offline'>('all');
  const [sortKey, setSortKey] = createSignal<SortKey>('name');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const { isMobile } = useBreakpoint();

  // Use the hook directly to ensure reactivity is maintained
  // This fixes the issue where props.hosts would not update when the underlying data changes
  const { asHosts } = useResourcesAsLegacy();

  // Column visibility management
  const columnVisibility = useColumnVisibility(
    STORAGE_KEYS.HOSTS_HIDDEN_COLUMNS,
    HOST_COLUMNS as HostColumnDef[]
  );
  const visibleColumns = columnVisibility.visibleColumns;
  const visibleColumnIds = createMemo(() => visibleColumns().map(c => c.id));
  const isColVisible = (colId: string) => visibleColumnIds().includes(colId);

  const handleSort = (key: SortKey) => {
    if (sortKey() === key) {
      setSortDirection(sortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setSortKey(key);
      setSortDirection('asc');
    }
  };

  const renderSortIndicator = (key: SortKey) => {
    if (sortKey() !== key) return null;
    return sortDirection() === 'asc' ? '▲' : '▼';
  };

  // Keyboard listener to auto-focus search
  let searchInputRef: HTMLInputElement | undefined;

  const handleKeyDown = (e: KeyboardEvent) => {
    const target = e.target as HTMLElement;
    if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
      return;
    }
    if (e.ctrlKey || e.metaKey || e.altKey) {
      return;
    }
    if (e.key.length === 1 && searchInputRef) {
      e.preventDefault();
      searchInputRef.focus();
      setSearch(search() + e.key);
    }
  };

  onMount(() => {
    document.addEventListener('keydown', handleKeyDown);
  });

  onCleanup(() => {
    document.removeEventListener('keydown', handleKeyDown);
  });

  const connected = () => wsContext.connected();
  const reconnecting = () => wsContext.reconnecting();
  const reconnect = () => wsContext.reconnect();

  // Access asHosts() directly inside the memo to maintain reactivity
  const hosts = () => asHosts() as Host[];

  const isInitialLoading = createMemo(() => {
    return !connected() && !reconnecting() && hosts().length === 0;
  });

  const sortedHosts = createMemo(() => {
    const hostList = [...hosts()];
    const key = sortKey();
    const direction = sortDirection();

    return hostList.sort((a: Host, b: Host) => {
      let comparison = 0;
      switch (key) {
        case 'name':
          comparison = (a.displayName || a.hostname || a.id).localeCompare(b.displayName || b.hostname || b.id);
          break;
        case 'platform':
          comparison = (a.platform || '').localeCompare(b.platform || '');
          break;
        case 'cpu':
          comparison = (a.cpuUsage ?? 0) - (b.cpuUsage ?? 0);
          break;
        case 'memory':
          comparison = (a.memory?.usage ?? 0) - (b.memory?.usage ?? 0);
          break;
        case 'disk': {
          const aDisk = a.disks?.reduce((sum: number, d: { usage?: number }) => sum + (d.usage ?? 0), 0) ?? 0;
          const bDisk = b.disks?.reduce((sum: number, d: { usage?: number }) => sum + (d.usage ?? 0), 0) ?? 0;
          comparison = aDisk - bDisk;
          break;
        }
        case 'uptime':
          comparison = (a.uptimeSeconds ?? 0) - (b.uptimeSeconds ?? 0);
          break;
        default:
          comparison = 0;
      }
      return direction === 'asc' ? comparison : -comparison;
    });
  });

  const matchesSearch = (host: Host) => {
    const term = search().toLowerCase();
    if (!term) return true;
    const hostname = (host.hostname || '').toLowerCase();
    const displayName = (host.displayName || '').toLowerCase();
    const platform = (host.platform || '').toLowerCase();
    const osName = (host.osName || '').toLowerCase();
    return (
      hostname.includes(term) ||
      displayName.includes(term) ||
      platform.includes(term) ||
      osName.includes(term)
    );
  };

  const matchesStatus = (host: Host) => {
    const filter = statusFilter();
    if (filter === 'all') return true;
    const normalized = (host.status || '').toLowerCase();
    return normalized === filter;
  };

  const filteredHosts = createMemo(() => {
    return sortedHosts().filter((host) => matchesSearch(host) && matchesStatus(host));
  });

  const getDiskStats = (host: Host) => {
    if (!host.disks || host.disks.length === 0) return { percent: 0, used: 0, total: 0 };
    const totalUsed = host.disks.reduce((sum, d) => sum + (d.used ?? 0), 0);
    const totalSize = host.disks.reduce((sum, d) => sum + (d.total ?? 0), 0);
    return {
      percent: totalSize > 0 ? (totalUsed / totalSize) * 100 : 0,
      used: totalUsed,
      total: totalSize
    };
  };



  const thClass = "px-2 py-1 text-center text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600 whitespace-nowrap";

  return (
    <div class="space-y-4">
      <Show when={isInitialLoading()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 animate-spin text-blue-500"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
            }
            title="Loading host data..."
            description="Connecting to the monitoring service."
          />
        </Card>
      </Show>

      {/* Disconnected State */}
      <Show when={!connected() && !isInitialLoading()}>
        <Card padding="lg" tone="danger">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-red-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            }
            title="Connection lost"
            description={
              reconnecting()
                ? 'Attempting to reconnect…'
                : 'Unable to connect to the backend server'
            }
            tone="danger"
            actions={
              !reconnecting() ? (
                <button
                  onClick={() => reconnect()}
                  class="mt-2 inline-flex items-center px-4 py-2 text-xs font-medium rounded bg-red-600 text-white hover:bg-red-700 transition-colors"
                >
                  Reconnect now
                </button>
              ) : undefined
            }
          />
        </Card>
      </Show>

      <Show when={!isInitialLoading()}>
        <Show
          when={sortedHosts().length === 0}
          fallback={
            <>
              {/* Filters with column visibility */}
              <HostsFilter
                search={search}
                setSearch={setSearch}
                statusFilter={statusFilter}
                setStatusFilter={setStatusFilter}
                onReset={() => setSearch('')}
                searchInputRef={(el) => (searchInputRef = el)}
                availableColumns={columnVisibility.availableToggles()}
                isColumnHidden={columnVisibility.isHiddenByUser}
                onColumnToggle={columnVisibility.toggle}
                onColumnReset={columnVisibility.resetToDefaults}
              />

              {/* Host Table */}
              <Show
                when={filteredHosts().length > 0}
                fallback={
                  <Card padding="lg">
                    <EmptyState
                      title="No hosts found"
                      description={
                        search().trim()
                          ? 'No hosts match your search.'
                          : 'No hosts available'
                      }
                    />
                  </Card>
                }
              >
                <Card padding="none" tone="glass" class="overflow-hidden">
                  <div class="overflow-x-auto">
                    <table class="w-full border-collapse whitespace-nowrap">
                      <thead>
                        <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
                          {/* Essential columns */}
                          <th class={`${thClass} text-left pl-4`} onClick={() => handleSort('name')}>
                            Host {renderSortIndicator('name')}
                          </th>
                          <Show when={isColVisible('platform')}>
                            <th class={thClass} onClick={() => handleSort('platform')}>
                              Platform {renderSortIndicator('platform')}
                            </th>
                          </Show>
                          <Show when={isColVisible('cpu')}>
                            <th class={thClass} onClick={() => handleSort('cpu')}>
                              CPU {renderSortIndicator('cpu')}
                            </th>
                          </Show>
                          <Show when={isColVisible('memory')}>
                            <th class={thClass} onClick={() => handleSort('memory')}>
                              Memory {renderSortIndicator('memory')}
                            </th>
                          </Show>
                          <Show when={isColVisible('disk')}>
                            <th class={thClass} onClick={() => handleSort('disk')}>
                              Disk {renderSortIndicator('disk')}
                            </th>
                          </Show>

                          {/* Secondary columns */}
                          <Show when={isColVisible('temp')}>
                            <th class={thClass} title="Temperature">
                              <span class="inline-flex items-center justify-center" innerHTML={HOST_COLUMNS.find(c => c.id === 'temp')?.icon} />
                            </th>
                          </Show>
                          <Show when={isColVisible('uptime')}>
                            <th class={thClass} onClick={() => handleSort('uptime')} title="Uptime">
                              <span class="inline-flex items-center justify-center gap-1">
                                <span innerHTML={HOST_COLUMNS.find(c => c.id === 'uptime')?.icon} />
                                {renderSortIndicator('uptime')}
                              </span>
                            </th>
                          </Show>
                          <Show when={isColVisible('agent')}>
                            <th class={thClass}>Agent</th>
                          </Show>

                          {/* Supplementary columns */}
                          <Show when={isColVisible('ip')}>
                            <th class={thClass} title="IP Address">
                              <span class="inline-flex items-center justify-center" innerHTML={HOST_COLUMNS.find(c => c.id === 'ip')?.icon} />
                            </th>
                          </Show>

                          {/* Detailed columns */}
                          <Show when={isColVisible('arch')}>
                            <th class={thClass}>Arch</th>
                          </Show>
                          <Show when={isColVisible('kernel')}>
                            <th class={thClass}>Kernel</th>
                          </Show>
                          <Show when={isColVisible('raid')}>
                            <th class={thClass} title="Linux Software RAID (mdadm) Status">RAID</th>
                          </Show>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                        <For each={filteredHosts()}>
                          {(host) => <HostRow host={host} isColVisible={isColVisible} isMobile={isMobile} getDiskStats={getDiskStats} />}
                        </For>
                      </tbody>
                    </table>
                  </div>
                </Card>
              </Show>
            </>
          }
        >
          <Card padding="lg">
            <EmptyState
              icon={
                <svg class="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"
                  />
                </svg>
              }
              title="No hosts reporting"
              description="Install the Pulse host agent on Linux, macOS, or Windows machines to begin monitoring."
              actions={
                <button
                  type="button"
                  onClick={() => navigate('/settings/hosts')}
                  class="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 transition-colors"
                >
                  <span>Set up host agent</span>
                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                  </svg>
                </button>
              }
            />
          </Card>
        </Show>
      </Show>
    </div>
  );
};

// Individual host row component
interface HostRowProps {
  host: Host;
  isColVisible: (colId: string) => boolean;
  isMobile: () => boolean;

  getDiskStats: (host: Host) => { percent: number; used: number; total: number };
}

const HostRow: Component<HostRowProps> = (props) => {
  const { host } = props;

  // Check if this host is in AI context
  const isInAIContext = createMemo(() => aiChatStore.enabled && aiChatStore.hasContextItem(host.id));

  // Build context for AI - includes routing fields
  const buildHostContext = (): Record<string, unknown> => ({
    hostName: host.displayName || host.hostname,
    hostname: host.hostname,
    node: host.hostname,           // Used by AI for command routing
    target_host: host.hostname,    // Explicit routing hint
    platform: host.platform,
    osName: host.osName,
    osVersion: host.osVersion,
    cpuUsage: host.cpuUsage ? `${host.cpuUsage.toFixed(1)}%` : undefined,
    memoryUsage: host.memory?.usage ? `${host.memory.usage.toFixed(1)}%` : undefined,
    uptime: host.uptimeSeconds ? formatUptime(host.uptimeSeconds) : undefined,
  });

  // Handle row click - toggle AI context selection
  const handleRowClick = (event: MouseEvent) => {
    const target = event.target as HTMLElement;
    if (target.closest('a, button, [data-prevent-toggle]')) {
      return;
    }

    // If AI is enabled, toggle AI context
    if (aiChatStore.enabled) {
      if (aiChatStore.hasContextItem(host.id)) {
        aiChatStore.removeContextItem(host.id);
      } else {
        aiChatStore.addContextItem('host', host.id, host.displayName || host.hostname, buildHostContext());
        if (!aiChatStore.isOpen) {
          aiChatStore.open();
        }
      }
    }
  };

  const hostStatus = createMemo(() => getHostStatusIndicator(host));
  const cpuPercent = host.cpuUsage ?? 0;
  const memPercent = host.memory?.usage ?? 0;
  const diskStats = props.getDiskStats(host);

  const rowClass = () => {
    const base = 'transition-all duration-200';
    const hover = 'hover:bg-gray-50 dark:hover:bg-gray-800/50';
    const clickable = aiChatStore.enabled ? 'cursor-pointer' : '';
    const aiContext = isInAIContext() ? 'ai-context-row' : '';
    return `${base} ${hover} ${clickable} ${aiContext}`;
  };

  return (
    <tr class={rowClass()} onClick={handleRowClick}>
      {/* Host Name - always visible */}
      <td class="pl-4 pr-2 py-1 align-middle">
        <div class="flex items-center gap-2 min-w-0">
          <StatusDot
            variant={hostStatus().variant}
            title={hostStatus().label}
            ariaLabel={hostStatus().label}
            size="xs"
          />
          <div class="min-w-0 flex items-center gap-1.5">
            <div>
              <p class="text-sm font-semibold text-gray-900 dark:text-gray-100 whitespace-nowrap">
                {host.displayName || host.hostname || host.id}
              </p>
              <Show when={host.displayName && host.displayName !== host.hostname}>
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5 whitespace-nowrap">
                  {host.hostname}
                </p>
              </Show>
              <Show when={host.lastSeen}>
                <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5 whitespace-nowrap">
                  Updated {formatRelativeTime(host.lastSeen!)}
                </p>
              </Show>
            </div>
            {/* AI context indicator */}
            <Show when={isInAIContext()}>
              <span class="flex-shrink-0 text-purple-500 dark:text-purple-400" title="Selected for AI context">
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456z" />
                </svg>
              </span>
            </Show>
          </div>
        </div>
      </td>

      {/* Platform */}
      <Show when={props.isColVisible('platform')}>
        <td class="px-2 py-1 align-middle">
          <div class="text-xs text-gray-700 dark:text-gray-300">
            <p class="font-medium capitalize whitespace-nowrap">{host.platform || '—'}</p>
            <Show when={host.osName}>
              <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5 whitespace-nowrap">
                {host.osName} {host.osVersion}
              </p>
            </Show>
          </div>
        </td>
      </Show>

      {/* CPU */}
      <Show when={props.isColVisible('cpu')}>
        <td class="px-2 py-1 align-middle" style={{ "min-width": "140px" }}>
          <Show when={props.isMobile()}>
            <div class="md:hidden flex justify-center">
              <MetricText value={cpuPercent} type="cpu" />
            </div>
          </Show>
          <div class="hidden md:block">
            <EnhancedCPUBar
              usage={cpuPercent}
              loadAverage={host.loadAverage?.[0]}
              cores={host.cpuCount}
            />
          </div>
        </td>
      </Show>

      {/* Memory */}
      <Show when={props.isColVisible('memory')}>
        <td class="px-2 py-1 align-middle" style={{ "min-width": "140px" }}>
          <Show when={props.isMobile()}>
            <div class="md:hidden flex justify-center">
              <MetricText value={memPercent} type="memory" />
            </div>
          </Show>
          <div class="hidden md:block">
            <StackedMemoryBar
              used={host.memory?.used || 0}
              total={host.memory?.total || 0}
              balloon={host.memory?.balloon || 0}
              swapUsed={host.memory?.swapUsed || 0}
              swapTotal={host.memory?.swapTotal || 0}
            />
          </div>
        </td>
      </Show>

      {/* Disk */}
      <Show when={props.isColVisible('disk')}>
        <td class="px-2 py-1 align-middle" style={{ "min-width": "140px" }}>
          <Show when={props.isMobile()}>
            <div class="md:hidden flex justify-center">
              <MetricText value={diskStats.percent} type="disk" />
            </div>
          </Show>
          <div class="hidden md:block">
            <StackedDiskBar
              disks={host.disks}
              aggregateDisk={{
                total: diskStats.total,
                used: diskStats.used,
                free: diskStats.total - diskStats.used,
                usage: diskStats.percent / 100
              }}
            />
          </div>
        </td>
      </Show>

      {/* Temperature - shows primary temp with all sensors in tooltip */}
      <Show when={props.isColVisible('temp')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <HostTemperatureCell sensors={host.sensors?.temperatureCelsius} />
          </div>
        </td>
      </Show>

      {/* Uptime */}
      <Show when={props.isColVisible('uptime')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <Show when={host.uptimeSeconds} fallback={<span class="text-xs text-gray-400">—</span>}>
              <span class="text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
                {formatUptime(host.uptimeSeconds!)}
              </span>
            </Show>
          </div>
        </td>
      </Show>

      {/* Agent Version */}
      <Show when={props.isColVisible('agent')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <Show when={host.agentVersion} fallback={<span class="text-xs text-gray-400">—</span>}>
              <span class="text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
                {host.agentVersion}
              </span>
            </Show>
          </div>
        </td>
      </Show>

      {/* IP Address - uses icon + count with tooltip */}
      <Show when={props.isColVisible('ip')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <HostNetworkInfoCell networkInterfaces={host.networkInterfaces || []} />
          </div>
        </td>
      </Show>

      {/* Architecture */}
      <Show when={props.isColVisible('arch')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <Show when={host.architecture} fallback={<span class="text-xs text-gray-400">—</span>}>
              <span class="text-[10px] text-gray-700 dark:text-gray-300 whitespace-nowrap">
                {host.architecture}
              </span>
            </Show>
          </div>
        </td>
      </Show>

      {/* Kernel */}
      <Show when={props.isColVisible('kernel')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <Show when={host.kernelVersion} fallback={<span class="text-xs text-gray-400">—</span>}>
              <span
                class="text-[10px] text-gray-700 dark:text-gray-300 max-w-[100px] truncate"
                title={host.kernelVersion}
              >
                {host.kernelVersion}
              </span>
            </Show>
          </div>
        </td>
      </Show>

      {/* RAID Status */}
      <Show when={props.isColVisible('raid')}>
        <td class="px-2 py-1 align-middle">
          <div class="flex justify-center">
            <HostRAIDStatusCell raid={host.raid} />
          </div>
        </td>
      </Show>
    </tr>
  );
};
