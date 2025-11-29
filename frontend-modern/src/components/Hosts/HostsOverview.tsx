import type { Component } from 'solid-js';
import { For, Show, createMemo, createSignal, createEffect, on, onMount, onCleanup } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { Host } from '@/types/api';
import { formatBytes, formatPercent, formatRelativeTime, formatUptime } from '@/utils/format';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { StackedDiskBar } from '@/components/Dashboard/StackedDiskBar';
import { HostsFilter } from './HostsFilter';
import { useWebSocket } from '@/App';
import { StatusDot } from '@/components/shared/StatusDot';
import { getHostStatusIndicator } from '@/utils/status';
import { ResponsiveMetricCell, MetricText, useGridTemplate } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Dashboard/StackedMemoryBar';
import { TemperatureGauge } from '@/components/shared/TemperatureGauge';
import type { ColumnConfig } from '@/types/responsive';
import { STANDARD_COLUMNS } from '@/types/responsive';

// Global drawer state to persist across re-renders
const drawerState = new Map<string, boolean>();

type SortKey = 'name' | 'platform' | 'cpu' | 'memory' | 'uptime';

const HOST_COLUMNS: ColumnConfig[] = [
  { ...STANDARD_COLUMNS.name, label: 'Host', minWidth: '200px', sortKey: 'name' },
  { id: 'platform', label: 'Platform', priority: 'primary', minWidth: '120px', flex: 1, sortable: true, sortKey: 'platform' },
  { ...STANDARD_COLUMNS.cpu, maxWidth: '156px', sortKey: 'cpu' },
  { ...STANDARD_COLUMNS.memory, maxWidth: '156px', sortKey: 'memory' },
  { id: 'temperature', label: 'Temp', priority: 'secondary', minWidth: '80px', maxWidth: '100px' },
  { id: 'disk', label: 'Disk', minWidth: '140px', maxWidth: '156px', priority: 'secondary' },
  { ...STANDARD_COLUMNS.uptime, maxWidth: '100px', align: 'right', sortKey: 'uptime' },
];

interface HostsOverviewProps {
  hosts: Host[];
  connectionHealth: Record<string, boolean>;
}

export const HostsOverview: Component<HostsOverviewProps> = (props) => {
  const navigate = useNavigate();
  const wsContext = useWebSocket();
  const [search, setSearch] = createSignal('');
  const [statusFilter, setStatusFilter] = createSignal<'all' | 'online' | 'degraded' | 'offline'>('all');
  const [sortKey, setSortKey] = createSignal<SortKey>('name');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');

  const handleSort = (key: string) => {
    if (sortKey() === key) {
      setSortDirection(sortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setSortKey(key as SortKey);
      setSortDirection('asc');
    }
  };

  // Use responsive grid template
  const { gridTemplate, visibleColumns, isMobile } = useGridTemplate({ columns: HOST_COLUMNS });

  // Keyboard listener to auto-focus search
  let searchInputRef: HTMLInputElement | undefined;

  const handleKeyDown = (e: KeyboardEvent) => {
    // Don't interfere if user is already typing in an input
    const target = e.target as HTMLElement;
    if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
      return;
    }

    // Don't interfere with modifier key shortcuts (except Shift for capitals)
    if (e.ctrlKey || e.metaKey || e.altKey) {
      return;
    }

    // Focus search on printable characters and start typing
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

  const isLoading = createMemo(() => {
    return !connected() && !reconnecting();
  });

  const sortedHosts = createMemo(() => {
    const hosts = [...props.hosts];
    const key = sortKey();
    const direction = sortDirection();

    return hosts.sort((a, b) => {
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

  const renderCell = (column: ColumnConfig, host: Host) => {
    const cpuPercent = () => host.cpuUsage ?? 0;
    const memPercent = () => host.memory?.usage ?? 0;

    const hostStatus = createMemo(() => getHostStatusIndicator(host));

    switch (column.id) {
      case 'name':
        return (
          <div class="pl-4 pr-2 py-1 overflow-hidden">
            <div class="flex items-center gap-2 min-w-0">
              <StatusDot
                variant={hostStatus().variant}
                title={hostStatus().label}
                ariaLabel={hostStatus().label}
                size="xs"
              />
              <div class="min-w-0 flex-1">
                <p class="text-sm font-semibold text-gray-900 dark:text-gray-100 truncate">
                  {host.displayName || host.hostname || host.id}
                </p>
                <Show when={host.displayName && host.displayName !== host.hostname}>
                  <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate">
                    {host.hostname}
                  </p>
                </Show>
                <Show when={host.lastSeen}>
                  <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5 truncate">
                    Updated {formatRelativeTime(host.lastSeen!)}
                  </p>
                </Show>
              </div>
            </div>
          </div>
        );
      case 'platform':
        return (
          <div class="px-2 py-1 overflow-hidden">
            <div class="text-xs text-gray-700 dark:text-gray-300">
              <p class="font-medium capitalize">{host.platform || '—'}</p>
              <Show when={host.osName}>
                <p class="text-[10px] text-gray-500 dark:text-gray-400 mt-0.5">
                  {host.osName} {host.osVersion}
                </p>
              </Show>
            </div>
          </div>
        );
      case 'cpu':
        return (
          <div class="px-2 py-1 overflow-hidden">
            <ResponsiveMetricCell
              value={cpuPercent()}
              type="cpu"
              label={formatPercent(cpuPercent())}
              isRunning={true}
              showMobile={isMobile()}
              class="w-full"
            />
          </div>
        );
      case 'memory':
        return (
          <div class="px-2 py-1 overflow-hidden">
            <Show when={isMobile()}>
              <div class="md:hidden">
                <MetricText value={memPercent()} type="memory" />
              </div>
            </Show>
            <div class="hidden md:block w-full">
              <StackedMemoryBar
                used={host.memory?.used || 0}
                total={host.memory?.total || 0}
                balloon={host.memory?.balloon || 0}
                swapUsed={host.memory?.swapUsed || 0}
                swapTotal={host.memory?.swapTotal || 0}
              />
            </div>
          </div>
        );
      case 'temperature':
        const tempValue = (() => {
          if (!host.sensors?.temperatureCelsius) return null;
          const temps = host.sensors.temperatureCelsius;

          // Try to find a "package" or "composite" temperature first
          const packageKey = Object.keys(temps).find(k =>
            k.toLowerCase().includes('package') ||
            k.toLowerCase().includes('composite') ||
            k.toLowerCase().includes('tctl')
          );

          if (packageKey) return temps[packageKey];

          // Fallback: average of all core temps
          const coreKeys = Object.keys(temps).filter(k => k.toLowerCase().includes('core'));
          if (coreKeys.length > 0) {
            const sum = coreKeys.reduce((acc, k) => acc + temps[k], 0);
            return sum / coreKeys.length;
          }

          // Fallback: just take the first available value if any
          const values = Object.values(temps);
          if (values.length > 0) return values[0];

          return null;
        })();

        return (
          <div class="px-2 py-1 overflow-hidden">
            <Show when={tempValue !== null} fallback={<span class="text-xs text-gray-500 dark:text-gray-400">—</span>}>
              <TemperatureGauge value={tempValue!} />
            </Show>
          </div>
        );
      case 'disk':
        const diskStats = (() => {
          if (!host.disks || host.disks.length === 0) return { percent: 0, used: 0, total: 0 };
          const totalUsed = host.disks.reduce((sum, d) => sum + (d.used ?? 0), 0);
          const totalSize = host.disks.reduce((sum, d) => sum + (d.total ?? 0), 0);
          return {
            percent: totalSize > 0 ? (totalUsed / totalSize) * 100 : 0,
            used: totalUsed,
            total: totalSize
          };
        })();
        const diskPercent = diskStats.percent;


        return (
          <div class="px-2 py-1 overflow-hidden">
            <Show when={isMobile()}>
              <div class="md:hidden">
                <MetricText value={diskPercent} type="disk" />
              </div>
            </Show>
            <div class="hidden md:block w-full">
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
          </div>
        );
      case 'uptime':
        return (
          <div class="px-2 py-1 text-right overflow-hidden">
            <Show
              when={host.uptimeSeconds}
              fallback={<span class="text-xs text-gray-500 dark:text-gray-400">—</span>}
            >
              <span class="text-xs text-gray-700 dark:text-gray-300">
                {formatUptime(host.uptimeSeconds!)}
              </span>
            </Show>
          </div>
        );
      default:
        return null;
    }
  };

  return (
    <div class="space-y-4">
      <Show when={isLoading()}>
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
            title={reconnecting() ? 'Reconnecting to host agents...' : 'Loading host data...'}
            description={
              reconnecting()
                ? 'Re-establishing metrics from the monitoring service.'
                : connected()
                  ? 'Waiting for the first host update.'
                  : 'Connecting to the monitoring service.'
            }
          />
        </Card>
      </Show>

      <Show when={!isLoading()}>
        <Show
          when={sortedHosts().length === 0}
          fallback={
            <>
              {/* Filters */}
              <HostsFilter
                search={search}
                setSearch={setSearch}
                statusFilter={statusFilter}
                setStatusFilter={setStatusFilter}
                onReset={() => setSearch('')}
                searchInputRef={(el) => (searchInputRef = el)}
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
                    {/* Header */}
                    <div
                      class="grid border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 text-[11px] sm:text-xs font-medium uppercase tracking-wider sticky top-0 z-20 min-w-[520px] md:min-w-0"
                      style={{ 'grid-template-columns': gridTemplate() }}
                    >
                      <For each={visibleColumns()}>
                        {(column) => (
                          <div
                            class={`${column.id === 'name' ? 'pl-4 pr-2' : 'px-2'} py-1 flex items-center ${column.align === 'right' ? 'justify-end' : 'justify-start'} ${column.sortable ? 'cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-600' : ''}`}
                            onClick={() => column.sortable && column.sortKey && handleSort(column.sortKey)}
                          >
                            <div class="flex items-center gap-1">
                              {column.label}
                              {column.sortKey === sortKey() && (
                                <span class="text-gray-400">
                                  {sortDirection() === 'asc' ? '▲' : '▼'}
                                </span>
                              )}
                            </div>
                          </div>
                        )}
                      </For>
                    </div>

                    {/* Rows */}
                    <div class="divide-y divide-gray-200 dark:divide-gray-700 min-w-[520px] md:min-w-0">
                      <For each={filteredHosts()}>
                        {(host) => {
                          // Drawer state
                          const [drawerOpen, setDrawerOpen] = createSignal(drawerState.get(host.id) ?? false);

                          // Check if we have additional info to show in drawer
                          const hasDrawerContent = createMemo(() => {
                            return (
                              (host.disks && host.disks.length > 0) ||
                              (host.networkInterfaces && host.networkInterfaces.length > 0) ||
                              (host.raid && host.raid.length > 0) ||
                              host.loadAverage ||
                              host.cpuCount ||
                              host.kernelVersion ||
                              host.architecture ||
                              host.agentVersion ||
                              (host.sensors?.temperatureCelsius && Object.keys(host.sensors.temperatureCelsius).length > 0)
                            );
                          });

                          const toggleDrawer = (event: MouseEvent) => {
                            if (!hasDrawerContent()) return;
                            const target = event.target as HTMLElement;
                            if (target.closest('a, button, [data-prevent-toggle]')) {
                              return;
                            }
                            setDrawerOpen((prev) => !prev);
                          };

                          // Sync drawer state
                          createEffect(on(() => host.id, (id) => {
                            const stored = drawerState.get(id);
                            if (stored !== undefined) {
                              setDrawerOpen(stored);
                            } else {
                              setDrawerOpen(false);
                            }
                          }));

                          createEffect(() => {
                            drawerState.set(host.id, drawerOpen());
                          });

                          const rowClass = () => {
                            const base = 'grid items-center transition-all duration-200';
                            const hover = 'hover:bg-gray-50 dark:hover:bg-gray-800/50';
                            const clickable = hasDrawerContent() ? 'cursor-pointer' : '';
                            const expanded = drawerOpen() ? 'bg-gray-50 dark:bg-gray-800/40' : '';
                            return `${base} ${hover} ${clickable} ${expanded}`;
                          };

                          return (
                            <>
                              <div
                                class={rowClass()}
                                style={{ 'grid-template-columns': gridTemplate() }}
                                onClick={toggleDrawer}
                                aria-expanded={drawerOpen()}
                              >
                                <For each={visibleColumns()}>
                                  {(column) => renderCell(column, host)}
                                </For>
                              </div>

                              {/* Drawer - Additional Info */}
                              <Show when={drawerOpen() && hasDrawerContent()}>
                                <div class="bg-gray-50 dark:bg-gray-900/50 px-4 py-3 border-t border-gray-100 dark:border-gray-800/50">
                                  <div class="flex flex-wrap justify-start gap-3">
                                    {/* System Info */}
                                    <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                      <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">System</div>
                                      <div class="mt-2 space-y-1 text-[11px] text-gray-600 dark:text-gray-300">
                                        <Show when={host.cpuCount}>
                                          <div class="flex items-center justify-between gap-2">
                                            <span class="font-medium text-gray-700 dark:text-gray-200">CPUs</span>
                                            <span class="text-right text-gray-600 dark:text-gray-300">{host.cpuCount}</span>
                                          </div>
                                        </Show>
                                        <Show when={host.loadAverage && host.loadAverage.length > 0}>
                                          <div class="flex items-center justify-between gap-2">
                                            <span class="font-medium text-gray-700 dark:text-gray-200">Load Avg</span>
                                            <span class="text-right text-gray-600 dark:text-gray-300">{host.loadAverage!.map(l => l.toFixed(2)).join(', ')}</span>
                                          </div>
                                        </Show>
                                        <Show when={host.architecture}>
                                          <div class="flex items-center justify-between gap-2">
                                            <span class="font-medium text-gray-700 dark:text-gray-200">Arch</span>
                                            <span class="text-right text-gray-600 dark:text-gray-300">{host.architecture}</span>
                                          </div>
                                        </Show>
                                        <Show when={host.kernelVersion}>
                                          <div class="flex items-center justify-between gap-2">
                                            <span class="font-medium text-gray-700 dark:text-gray-200">Kernel</span>
                                            <span class="text-right text-gray-600 dark:text-gray-300 truncate">{host.kernelVersion}</span>
                                          </div>
                                        </Show>
                                        <Show when={host.agentVersion}>
                                          <div class="flex items-center justify-between gap-2">
                                            <span class="font-medium text-gray-700 dark:text-gray-200">Agent</span>
                                            <span class="text-right text-gray-600 dark:text-gray-300">{host.agentVersion}</span>
                                          </div>
                                        </Show>
                                      </div>
                                    </div>

                                    {/* Network Interfaces */}
                                    <Show when={host.networkInterfaces && host.networkInterfaces.length > 0}>
                                      <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">Network</div>
                                        <div class="mt-2 space-y-2 text-[11px]">
                                          <For each={host.networkInterfaces?.slice(0, 4)}>
                                            {(iface) => (
                                              <div class="rounded border border-dashed border-gray-200 p-2 dark:border-gray-700/70">
                                                <div class="font-medium text-gray-700 dark:text-gray-200">{iface.name}</div>
                                                <Show when={iface.addresses && iface.addresses.length > 0}>
                                                  <div class="flex flex-wrap gap-1 mt-1 text-[10px]">
                                                    <For each={iface.addresses}>
                                                      {(addr) => (
                                                        <span class="rounded bg-blue-100 px-1.5 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                                                          {addr}
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
                                    </Show>

                                    {/* Disk Info */}
                                    <Show when={host.disks && host.disks.length > 0}>
                                      <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">Disks</div>
                                        <div class="mt-2 space-y-2 text-[11px]">
                                          <For each={host.disks?.slice(0, 3)}>
                                            {(disk) => {
                                              const diskPercent = () => disk.usage ?? 0;
                                              return (
                                                <div class="rounded border border-dashed border-gray-200 p-2 dark:border-gray-700/70">
                                                  <div class="flex items-center justify-between">
                                                    <span class="font-medium text-gray-700 dark:text-gray-200 truncate">{disk.mountpoint || disk.device}</span>
                                                    <span class="text-[10px] text-gray-500 dark:text-gray-400">
                                                      {formatBytes(disk.used ?? 0, 0)} / {formatBytes(disk.total ?? 0, 0)}
                                                    </span>
                                                  </div>
                                                  <Show when={diskPercent() > 0}>
                                                    <div class="mt-1">
                                                      <MetricBar
                                                        value={diskPercent()}
                                                        label={formatPercent(diskPercent())}
                                                        type="disk"
                                                        class="max-w-none"
                                                      />
                                                    </div>
                                                  </Show>
                                                </div>
                                              );
                                            }}
                                          </For>
                                        </div>
                                      </div>
                                    </Show>

                                    {/* Temperature Sensors */}
                                    <Show when={host.sensors?.temperatureCelsius && Object.keys(host.sensors.temperatureCelsius).length > 0}>
                                      <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">Temperatures</div>
                                        <div class="mt-2 space-y-1 text-[11px] text-gray-600 dark:text-gray-300">
                                          <For each={Object.entries(host.sensors!.temperatureCelsius!).slice(0, 5)}>
                                            {([name, temp]) => (
                                              <div class="flex items-center justify-between gap-2">
                                                <span class="font-medium text-gray-700 dark:text-gray-200 truncate">{name}</span>
                                                <span class={`text-right ${temp > 80 ? 'text-red-600 dark:text-red-400 font-semibold' : 'text-gray-600 dark:text-gray-300'}`}>
                                                  {temp.toFixed(1)}°C
                                                </span>
                                              </div>
                                            )}
                                          </For>
                                        </div>
                                      </div>
                                    </Show>

                                    {/* RAID Arrays */}
                                    <Show when={host.raid && host.raid.length > 0}>
                                      <For each={host.raid!}>
                                        {(array) => {
                                          const isDegraded = () => array.state.toLowerCase().includes('degraded') || array.failedDevices > 0;
                                          const isRebuilding = () => array.state.toLowerCase().includes('recover') || array.state.toLowerCase().includes('resync') || array.rebuildPercent > 0;
                                          const isHealthy = () => !isDegraded() && !isRebuilding() && array.state.toLowerCase().includes('clean');

                                          const stateColor = () => {
                                            if (isDegraded()) return 'text-red-600 dark:text-red-400 font-semibold';
                                            if (isRebuilding()) return 'text-amber-600 dark:text-amber-400 font-semibold';
                                            if (isHealthy()) return 'text-green-600 dark:text-green-400';
                                            return 'text-gray-600 dark:text-gray-300';
                                          };

                                          return (
                                            <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                              <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">
                                                RAID {array.level.replace('raid', '')} - {array.device}
                                              </div>
                                              <div class="mt-2 space-y-1 text-[11px]">
                                                <div class="flex items-center justify-between gap-2">
                                                  <span class="font-medium text-gray-700 dark:text-gray-200">State</span>
                                                  <span class={stateColor()}>{array.state}</span>
                                                </div>
                                                <div class="flex items-center justify-between gap-2">
                                                  <span class="font-medium text-gray-700 dark:text-gray-200">Devices</span>
                                                  <span class="text-gray-600 dark:text-gray-300">
                                                    {array.activeDevices}/{array.totalDevices}
                                                    {array.failedDevices > 0 && <span class="text-red-600 dark:text-red-400"> ({array.failedDevices} failed)</span>}
                                                  </span>
                                                </div>
                                                <Show when={isRebuilding() && array.rebuildPercent > 0}>
                                                  <div class="flex items-center justify-between gap-2">
                                                    <span class="font-medium text-gray-700 dark:text-gray-200">Rebuild</span>
                                                    <span class="text-amber-600 dark:text-amber-400 font-medium">
                                                      {array.rebuildPercent.toFixed(1)}%
                                                    </span>
                                                  </div>
                                                  <Show when={array.rebuildSpeed}>
                                                    <div class="flex items-center justify-between gap-2">
                                                      <span class="font-medium text-gray-700 dark:text-gray-200">Speed</span>
                                                      <span class="text-gray-600 dark:text-gray-300">{array.rebuildSpeed}</span>
                                                    </div>
                                                  </Show>
                                                </Show>
                                              </div>
                                            </div>
                                          );
                                        }}
                                      </For>
                                    </Show>
                                  </div>
                                </div>
                              </Show>
                            </>
                          );
                        }}
                      </For>
                    </div>
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
