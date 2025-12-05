import type { Component } from 'solid-js';
import { For, Show, createMemo, createSignal, createEffect, on, onMount, onCleanup } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { Host } from '@/types/api';
import { formatBytes, formatNumber, formatPercent, formatRelativeTime, formatUptime } from '@/utils/format';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { StackedDiskBar } from '@/components/Dashboard/StackedDiskBar';
import { HostsFilter } from './HostsFilter';
import { useWebSocket } from '@/App';
import { StatusDot } from '@/components/shared/StatusDot';
import { getHostStatusIndicator } from '@/utils/status';
import { MetricText } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Dashboard/StackedMemoryBar';
import { EnhancedCPUBar } from '@/components/Dashboard/EnhancedCPUBar';
import { TemperatureGauge } from '@/components/shared/TemperatureGauge';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import { AIAPI } from '@/api/ai';
import { aiChatStore } from '@/stores/aiChat';
import { logger } from '@/utils/logger';

// Global drawer state to persist across re-renders
const drawerState = new Map<string, boolean>();

type SortKey = 'name' | 'platform' | 'cpu' | 'memory' | 'disk' | 'uptime';

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
  const { isMobile } = useBreakpoint();

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
  const reconnect = () => wsContext.reconnect();

  const isInitialLoading = createMemo(() => {
    // Only show loading spinner when we've never been connected and have no hosts
    return !connected() && !reconnecting() && props.hosts.length === 0;
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
        case 'disk': {
          const aDisk = a.disks?.reduce((sum, d) => sum + (d.usage ?? 0), 0) ?? 0;
          const bDisk = b.disks?.reduce((sum, d) => sum + (d.usage ?? 0), 0) ?? 0;
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

  const getTemperatureValue = (host: Host) => {
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
  };

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
                    <table class="w-full border-collapse whitespace-nowrap">
                      <thead>
                        <tr class="bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
                          <th
                            class={`${thClass} text-left pl-4`}
                            onClick={() => handleSort('name')}
                          >
                            Host {renderSortIndicator('name')}
                          </th>
                          <th class={thClass} onClick={() => handleSort('platform')}>
                            Platform {renderSortIndicator('platform')}
                          </th>
                          <th class={thClass} onClick={() => handleSort('cpu')}>
                            CPU {renderSortIndicator('cpu')}
                          </th>
                          <th class={thClass} onClick={() => handleSort('memory')}>
                            Memory {renderSortIndicator('memory')}
                          </th>
                          <th class={thClass}>
                            Temp
                          </th>
                          <th class={thClass} onClick={() => handleSort('disk')}>
                            Disk {renderSortIndicator('disk')}
                          </th>
                          <th class={thClass} onClick={() => handleSort('uptime')}>
                            Uptime {renderSortIndicator('uptime')}
                          </th>
                          <th class={`${thClass} text-right pr-4`}>
                            Agent
                          </th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                        <For each={filteredHosts()}>
                          {(host) => {
                            // Drawer state
                            const [drawerOpen, setDrawerOpen] = createSignal(drawerState.get(host.id) ?? false);

                            // AI and annotations state
                            const [aiEnabled, setAiEnabled] = createSignal(false);
                            const [annotations, setAnnotations] = createSignal<string[]>([]);
                            const [newAnnotation, setNewAnnotation] = createSignal('');
                            const [saving, setSaving] = createSignal(false);

                            // Load AI settings and annotations when drawer opens
                            createEffect(() => {
                              if (drawerOpen()) {
                                AIAPI.getSettings()
                                  .then((settings) => setAiEnabled(settings.enabled && settings.configured))
                                  .catch((err) => logger.debug('[HostsOverview] AI settings check failed:', err));

                                GuestMetadataAPI.getMetadata(`host-${host.id}`)
                                  .then((meta) => {
                                    if (meta.notes && Array.isArray(meta.notes)) setAnnotations(meta.notes);
                                  })
                                  .catch((err) => logger.debug('[HostsOverview] Failed to load annotations:', err));
                              }
                            });

                            const saveAnnotations = async (updated: string[]) => {
                              setSaving(true);
                              try {
                                await GuestMetadataAPI.updateMetadata(`host-${host.id}`, { notes: updated });
                              } catch (err) {
                                logger.error('[HostsOverview] Failed to save annotations:', err);
                              } finally {
                                setSaving(false);
                              }
                            };

                            const addAnnotation = () => {
                              const text = newAnnotation().trim();
                              if (!text) return;
                              const updated = [...annotations(), text];
                              setAnnotations(updated);
                              setNewAnnotation('');
                              saveAnnotations(updated);
                            };

                            const removeAnnotation = (index: number) => {
                              const updated = annotations().filter((_, i) => i !== index);
                              setAnnotations(updated);
                              saveAnnotations(updated);
                            };

                            const handleKeyDown = (e: KeyboardEvent) => {
                              if (e.key === 'Enter') {
                                e.preventDefault();
                                addAnnotation();
                              }
                            };

                            const handleAskAI = () => {
                              const context: Record<string, unknown> = {
                                hostName: host.displayName || host.hostname,
                                hostname: host.hostname,
                                platform: host.platform,
                                osName: host.osName,
                                osVersion: host.osVersion,
                                cpuUsage: host.cpuUsage ? `${host.cpuUsage.toFixed(1)}%` : undefined,
                                memoryUsage: host.memory?.usage ? `${host.memory.usage.toFixed(1)}%` : undefined,
                                uptime: host.uptimeSeconds ? formatUptime(host.uptimeSeconds) : undefined,
                              };
                              if (annotations().length > 0) context.user_annotations = annotations();

                              aiChatStore.openForTarget('host', host.id, context);
                            };

                            // Check if we have additional info to show in drawer
                            const hasDrawerContent = createMemo(() => {
                              return (
                                (host.disks && host.disks.length > 0) ||
                                (host.diskIO && host.diskIO.length > 0) ||
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
                              const base = 'transition-all duration-200';
                              const hover = 'hover:bg-gray-50 dark:hover:bg-gray-800/50';
                              const clickable = hasDrawerContent() ? 'cursor-pointer' : '';
                              const expanded = drawerOpen() ? 'bg-gray-50 dark:bg-gray-800/40' : '';
                              return `${base} ${hover} ${clickable} ${expanded}`;
                            };

                            const hostStatus = createMemo(() => getHostStatusIndicator(host));
                            const cpuPercent = host.cpuUsage ?? 0;
                            const memPercent = host.memory?.usage ?? 0;
                            const tempValue = getTemperatureValue(host);
                            const diskStats = getDiskStats(host);

                            return (
                              <>
                                <tr
                                  class={rowClass()}
                                  onClick={toggleDrawer}
                                  aria-expanded={drawerOpen()}
                                >
                                  {/* Host Name */}
                                  <td class="pl-4 pr-2 py-1 align-middle">
                                    <div class="flex items-center gap-2 min-w-0">
                                      <StatusDot
                                        variant={hostStatus().variant}
                                        title={hostStatus().label}
                                        ariaLabel={hostStatus().label}
                                        size="xs"
                                      />
                                      <div class="min-w-0">
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
                                    </div>
                                  </td>

                                  {/* Platform */}
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

                                  {/* CPU */}
                                  <td class="px-2 py-1 align-middle" style={{ "min-width": "140px" }}>
                                    <Show when={isMobile()}>
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

                                  {/* Memory */}
                                  <td class="px-2 py-1 align-middle" style={{ "min-width": "140px" }}>
                                    <Show when={isMobile()}>
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

                                  {/* Temperature */}
                                  <td class="px-2 py-1 align-middle">
                                    <div class="flex justify-center">
                                      <Show when={tempValue !== null} fallback={<span class="text-xs text-gray-400">—</span>}>
                                        <TemperatureGauge value={tempValue!} />
                                      </Show>
                                    </div>
                                  </td>

                                  {/* Disk */}
                                  <td class="px-2 py-1 align-middle" style={{ "min-width": "140px" }}>
                                    <Show when={isMobile()}>
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

                                  {/* Uptime */}
                                  <td class="px-2 py-1 align-middle">
                                    <div class="flex justify-center">
                                      <Show
                                        when={host.uptimeSeconds}
                                        fallback={<span class="text-xs text-gray-400">—</span>}
                                      >
                                        <span class="text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
                                          {formatUptime(host.uptimeSeconds!)}
                                        </span>
                                      </Show>
                                    </div>
                                  </td>

                                  {/* Agent Version */}
                                  <td class="px-2 py-1 pr-4 align-middle">
                                    <div class="flex justify-end">
                                      <Show
                                        when={host.agentVersion}
                                        fallback={<span class="text-xs text-gray-400">—</span>}
                                      >
                                        <span class="text-xs text-gray-700 dark:text-gray-300 whitespace-nowrap">
                                          {host.agentVersion}
                                        </span>
                                      </Show>
                                    </div>
                                  </td>
                                </tr>

                                {/* Drawer - Additional Info */}
                                <Show when={drawerOpen() && hasDrawerContent()}>
                                  <tr>
                                    <td colspan={8} class="p-0">
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

                                          {/* Disk I/O */}
                                          <Show when={host.diskIO && host.diskIO.length > 0}>
                                            <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                              <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200">Disk I/O</div>
                                              <div class="mt-2 space-y-2 text-[11px]">
                                                <For each={host.diskIO?.slice(0, 4)}>
                                                  {(io) => (
                                                    <div class="rounded border border-dashed border-gray-200 p-2 dark:border-gray-700/70">
                                                      <div class="font-medium text-gray-700 dark:text-gray-200">{io.device}</div>
                                                      <div class="mt-1 grid grid-cols-2 gap-x-2 gap-y-0.5 text-[10px]">
                                                        <div class="flex items-center justify-between">
                                                          <span class="text-gray-500 dark:text-gray-400">Read:</span>
                                                          <span class="text-gray-600 dark:text-gray-300">{formatBytes(io.readBytes ?? 0, 1)}</span>
                                                        </div>
                                                        <div class="flex items-center justify-between">
                                                          <span class="text-gray-500 dark:text-gray-400">Write:</span>
                                                          <span class="text-gray-600 dark:text-gray-300">{formatBytes(io.writeBytes ?? 0, 1)}</span>
                                                        </div>
                                                        <div class="flex items-center justify-between">
                                                          <span class="text-gray-500 dark:text-gray-400">Read Ops:</span>
                                                          <span class="text-gray-600 dark:text-gray-300">{formatNumber(io.readOps ?? 0)}</span>
                                                        </div>
                                                        <div class="flex items-center justify-between">
                                                          <span class="text-gray-500 dark:text-gray-400">Write Ops:</span>
                                                          <span class="text-gray-600 dark:text-gray-300">{formatNumber(io.writeOps ?? 0)}</span>
                                                        </div>
                                                      </div>
                                                    </div>
                                                  )}
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

                                        {/* AI Context & Ask AI row */}
                                        <Show when={aiEnabled()}>
                                          <div class="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700 space-y-2">
                                            <div class="flex items-center gap-1.5">
                                              <span class="text-[10px] font-medium text-gray-500 dark:text-gray-400">AI Context</span>
                                              <Show when={saving()}>
                                                <span class="text-[9px] text-gray-400">saving...</span>
                                              </Show>
                                            </div>

                                            {/* Existing annotations */}
                                            <Show when={annotations().length > 0}>
                                              <div class="flex flex-wrap gap-1.5">
                                                <For each={annotations()}>
                                                  {(annotation, index) => (
                                                    <span class="inline-flex items-center gap-1 px-2 py-1 text-[11px] rounded-md bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-200">
                                                      <span class="max-w-[300px] truncate">{annotation}</span>
                                                      <button
                                                        type="button"
                                                        onClick={() => removeAnnotation(index())}
                                                        class="ml-0.5 p-0.5 rounded hover:bg-purple-200 dark:hover:bg-purple-800 transition-colors"
                                                        title="Remove"
                                                      >
                                                        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                                        </svg>
                                                      </button>
                                                    </span>
                                                  )}
                                                </For>
                                              </div>
                                            </Show>

                                            {/* Add new annotation + Ask AI */}
                                            <div class="flex items-center gap-2">
                                              <input
                                                type="text"
                                                value={newAnnotation()}
                                                onInput={(e) => setNewAnnotation(e.currentTarget.value)}
                                                onKeyDown={handleKeyDown}
                                                placeholder="Add context for AI (press Enter)..."
                                                class="flex-1 px-2 py-1.5 text-[11px] rounded border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-purple-500 focus:border-purple-500"
                                              />
                                              <button
                                                type="button"
                                                onClick={addAnnotation}
                                                disabled={!newAnnotation().trim()}
                                                class="px-2 py-1.5 text-[11px] rounded border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                                              >
                                                Add
                                              </button>
                                              <button
                                                type="button"
                                                onClick={handleAskAI}
                                                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-gradient-to-r from-purple-500 to-pink-500 text-white text-[11px] font-medium shadow-sm hover:from-purple-600 hover:to-pink-600 transition-all"
                                                title={`Ask AI about ${host.displayName || host.hostname}`}
                                              >
                                                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611l-2.576.43a18.003 18.003 0 01-5.118 0l-2.576-.43c-1.717-.293-2.299-2.379-1.067-3.611L5 14.5" />
                                                </svg>
                                                Ask AI
                                              </button>
                                            </div>
                                          </div>
                                        </Show>
                                      </div>
                                    </td>
                                  </tr>
                                </Show>
                              </>
                            );
                          }}
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
