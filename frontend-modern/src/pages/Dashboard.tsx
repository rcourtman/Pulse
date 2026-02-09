import { For, Match, Switch, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { useDashboardOverview } from '@/hooks/useDashboardOverview';
import { useDashboardTrends } from '@/hooks/useDashboardTrends';
import { Sparkline } from '@/components/shared/Sparkline';
import type { TrendData } from '@/hooks/useDashboardTrends';
import { isInfrastructure, isStorage } from '@/types/resource';
import {
  ALERTS_OVERVIEW_PATH,
  AI_PATROL_PATH,
  buildBackupsPath,
  buildStoragePath,
  INFRASTRUCTURE_PATH,
  WORKLOADS_PATH,
} from '@/routing/resourceLinks';
import { formatBytes } from '@/utils/format';
import { getMetricColorClass } from '@/utils/metricThresholds';

const PANEL_BASE_CLASS =
  'border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm p-4 sm:p-5';

type StatusTone = 'online' | 'offline' | 'warning' | 'critical' | 'unknown';

function statusBadgeClass(tone: StatusTone): string {
  const base = 'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium';
  switch (tone) {
    case 'online':
      return `${base} border-emerald-200 bg-emerald-100 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300`;
    case 'warning':
      return `${base} border-amber-200 bg-amber-100 text-amber-700 dark:border-amber-800 dark:bg-amber-900/50 dark:text-amber-300`;
    case 'critical':
      return `${base} border-red-200 bg-red-100 text-red-700 dark:border-red-800 dark:bg-red-900/50 dark:text-red-300`;
    case 'offline':
      return `${base} border-gray-200 bg-gray-100 text-gray-700 dark:border-gray-600 dark:bg-gray-700/60 dark:text-gray-200`;
    default:
      return `${base} border-gray-200 bg-gray-100 text-gray-600 dark:border-gray-600 dark:bg-gray-700/60 dark:text-gray-400`;
  }
}


function formatPercent(value: number): string {
  return `${Math.round(value)}%`;
}

function formatDelta(delta: number | null): string | null {
  if (delta === null) return null;
  const sign = delta >= 0 ? '+' : '';
  return `${sign}${delta.toFixed(1)}%`;
}

function deltaColorClass(delta: number | null): string {
  if (delta === null) return 'text-gray-400 dark:text-gray-500';
  if (delta > 5) return 'text-red-500 dark:text-red-400';
  if (delta > 0) return 'text-amber-500 dark:text-amber-400';
  if (delta < -5) return 'text-emerald-500 dark:text-emerald-400';
  if (delta < 0) return 'text-blue-500 dark:text-blue-400';
  return 'text-gray-500 dark:text-gray-400';
}

type ActionPriority = 'critical' | 'high' | 'medium' | 'low';

interface ActionItem {
  id: string;
  priority: ActionPriority;
  label: string;
  link: string;
}

const PRIORITY_ORDER: Record<ActionPriority, number> = {
  critical: 0,
  high: 1,
  medium: 2,
  low: 3,
};

const MAX_ACTION_ITEMS = 8;

function priorityBadgeClass(priority: ActionPriority): string {
  switch (priority) {
    case 'critical':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300';
    case 'high':
      return 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300';
    case 'medium':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300';
    case 'low':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300';
  }
}

export default function Dashboard() {
  const { state, connected, reconnecting, reconnect, initialDataReceived, activeAlerts } = useWebSocket();
  const overview = useDashboardOverview();
  const trends = useDashboardTrends(overview);
  const trendsLoading = createMemo(() => trends().loading);
  const [loadingTimedOut, setLoadingTimedOut] = createSignal(false);
  let loadingTimeout: number | undefined;

  createEffect(() => {
    if (!initialDataReceived()) {
      if (loadingTimeout) return;
      loadingTimeout = window.setTimeout(() => {
        setLoadingTimedOut(true);
      }, 30000);
      return;
    }

    if (loadingTimeout) {
      window.clearTimeout(loadingTimeout);
      loadingTimeout = undefined;
    }
    setLoadingTimedOut(false);
  });

  onCleanup(() => {
    if (loadingTimeout) {
      window.clearTimeout(loadingTimeout);
    }
  });

  const resources = createMemo(() => (Array.isArray(state.resources) ? state.resources : []));
  const isLoading = createMemo(() => !initialDataReceived());
  const hasConnectionError = createMemo(() => {
    if (loadingTimedOut()) return true;
    return initialDataReceived() && !connected() && !reconnecting();
  });
  const isEmpty = createMemo(
    () => initialDataReceived() && resources().length === 0 && !hasConnectionError(),
  );
  const infrastructureTopCPU = createMemo(() => overview().infrastructure.topCPU.slice(0, 5));
  const infrastructureTopMemory = createMemo(() => overview().infrastructure.topMemory.slice(0, 5));
  const workloadTypes = createMemo(() => Object.entries(overview().workloads.byType));
  const workloadRunningPercent = createMemo(() => {
    const { total, running } = overview().workloads;
    if (total <= 0) return 0;
    return Math.max(0, Math.min(100, (running / total) * 100));
  });
  const workloadStoppedPercent = createMemo(() => {
    const { total, stopped } = overview().workloads;
    if (total <= 0) return 0;
    return Math.max(0, Math.min(100, (stopped / total) * 100));
  });
  const storageCapacityPercent = createMemo(() => {
    const { totalUsed, totalCapacity } = overview().storage;
    if (totalCapacity <= 0) return 0;
    return Math.max(0, Math.min(100, (totalUsed / totalCapacity) * 100));
  });

  const statusDistribution = createMemo(() => {
    const byStatus = overview().health.byStatus || {};
    return [
      {
        key: 'online',
        label: 'Online',
        count: (byStatus.online ?? 0) + (byStatus.running ?? 0),
        tone: 'online' as const,
      },
      {
        key: 'offline',
        label: 'Offline',
        count: (byStatus.offline ?? 0) + (byStatus.stopped ?? 0),
        tone: 'offline' as const,
      },
      {
        key: 'warning',
        label: 'Warning',
        count: byStatus.degraded ?? 0,
        tone: 'warning' as const,
      },
      {
        key: 'critical',
        label: 'Critical',
        count: byStatus.critical ?? 0,
        tone: 'critical' as const,
      },
      {
        key: 'unknown',
        label: 'Unknown',
        count: (byStatus.unknown ?? 0) + (byStatus.paused ?? 0),
        tone: 'unknown' as const,
      },
    ];
  });

  const healthPanelTintClass = createMemo(() => {
    const { criticalAlerts, warningAlerts } = overview().health;
    if (criticalAlerts > 0) return 'bg-red-50/30 dark:bg-red-900/10';
    if (warningAlerts > 0) return 'bg-amber-50/30 dark:bg-amber-900/10';
    return 'bg-white dark:bg-gray-800';
  });

  const actionItems = createMemo<ActionItem[]>(() => {
    const items: ActionItem[] = [];
    const resources = Array.isArray(state.resources) ? state.resources : [];
    const alerts = Object.values(activeAlerts as Record<string, import('@/types/api').Alert | undefined>).filter(
      (a): a is import('@/types/api').Alert => a !== undefined
    );

    // Tier 1: Active critical alerts
    for (const alert of alerts) {
      if (alert.level === 'critical') {
        items.push({
          id: `alert-crit-${alert.id}`,
          priority: 'critical',
          label: `Critical alert: ${alert.resourceName} — ${alert.message}`,
          link: ALERTS_OVERVIEW_PATH,
        });
      }
    }

    // Tier 2: Infrastructure offline
    for (const resource of resources) {
      if (isInfrastructure(resource) && (resource.status === 'offline')) {
        items.push({
          id: `infra-offline-${resource.id}`,
          priority: 'high',
          label: `Offline: ${resource.displayName || resource.name}`,
          link: INFRASTRUCTURE_PATH,
        });
      }
    }

    // Tier 3: Storage critical (≥90%)
    for (const resource of resources) {
      if (isStorage(resource) && resource.disk) {
        const diskPercent = typeof resource.disk.total === 'number' && resource.disk.total > 0
          ? (resource.disk.used ?? 0) / resource.disk.total * 100
          : null;
        if (diskPercent !== null && diskPercent >= 90) {
          items.push({
            id: `storage-crit-${resource.id}`,
            priority: 'high',
            label: `Storage critical: ${resource.displayName || resource.name} at ${Math.round(diskPercent)}%`,
            link: buildStoragePath(),
          });
        }
      }
    }

    // Tier 4: Active warning alerts
    for (const alert of alerts) {
      if (alert.level === 'warning') {
        items.push({
          id: `alert-warn-${alert.id}`,
          priority: 'medium',
          label: `Warning: ${alert.resourceName} — ${alert.message}`,
          link: ALERTS_OVERVIEW_PATH,
        });
      }
    }

    // Tier 5: Storage warning (≥80% but <90%)
    for (const resource of resources) {
      if (isStorage(resource) && resource.disk) {
        const diskPercent = typeof resource.disk.total === 'number' && resource.disk.total > 0
          ? (resource.disk.used ?? 0) / resource.disk.total * 100
          : null;
        if (diskPercent !== null && diskPercent >= 80 && diskPercent < 90) {
          items.push({
            id: `storage-warn-${resource.id}`,
            priority: 'medium',
            label: `Storage warning: ${resource.displayName || resource.name} at ${Math.round(diskPercent)}%`,
            link: buildStoragePath(),
          });
        }
      }
    }

    // Tier 6: CPU critical (≥90%)
    const topCPU = overview().infrastructure.topCPU;
    for (const entry of topCPU) {
      if (entry.percent >= 90) {
        items.push({
          id: `cpu-crit-${entry.id}`,
          priority: 'low',
          label: `High CPU: ${entry.name} at ${Math.round(entry.percent)}%`,
          link: INFRASTRUCTURE_PATH,
        });
      }
    }

    // Sort by priority then by label for stability
    items.sort((a, b) => {
      const priorityDiff = PRIORITY_ORDER[a.priority] - PRIORITY_ORDER[b.priority];
      if (priorityDiff !== 0) return priorityDiff;
      return a.label.localeCompare(b.label);
    });

    return items;
  });

  const displayedActions = createMemo(() => actionItems().slice(0, MAX_ACTION_ITEMS));
  const overflowCount = createMemo(() => Math.max(0, actionItems().length - MAX_ACTION_ITEMS));

  return (
    <main data-testid="dashboard-page" aria-labelledby="dashboard-title" class="space-y-6">
      <h1 id="dashboard-title" class="text-xl sm:text-2xl font-semibold text-gray-900 dark:text-gray-100">
        Dashboard
      </h1>

      <Switch>
        <Match when={isLoading()}>
          <section class="space-y-6" data-testid="dashboard-loading">
            <div class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`}>
              <div class="space-y-4">
                <div data-testid="dashboard-skeleton-block" class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-4 w-44" />
                <div data-testid="dashboard-skeleton-block" class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-10 w-40" />
                <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
                  <div data-testid="dashboard-skeleton-block" class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-8" />
                  <div data-testid="dashboard-skeleton-block" class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-8" />
                  <div data-testid="dashboard-skeleton-block" class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-8" />
                  <div data-testid="dashboard-skeleton-block" class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-8" />
                  <div data-testid="dashboard-skeleton-block" class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-8" />
                </div>
              </div>
            </div>
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <div data-testid="dashboard-skeleton-block" class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`}>
                <div class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-24" />
              </div>
              <div data-testid="dashboard-skeleton-block" class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`}>
                <div class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-24" />
              </div>
            </div>
          </section>
        </Match>

        <Match when={hasConnectionError()}>
          <section class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`} aria-live="polite">
            <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">Dashboard unavailable</h2>
            <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
              Real-time dashboard data is currently unavailable. Reconnect to try again.
            </p>
            <button
              type="button"
              onClick={() => reconnect()}
              class="mt-4 inline-flex items-center rounded-md border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
            >
              Reconnect
            </button>
          </section>
        </Match>

        <Match when={isEmpty()}>
          <section class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`} aria-live="polite">
            <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">No resources yet</h2>
            <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
              Once connected platforms report resources, your dashboard overview will appear here.
            </p>
          </section>
        </Match>

        <Match when={!isLoading() && !hasConnectionError() && !isEmpty()}>
          <section class="space-y-6">
            <section class={`${PANEL_BASE_CLASS} ${healthPanelTintClass()}`} aria-labelledby="environment-overview-heading">
              <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">DP-HEALTH</p>
              <h2
                id="environment-overview-heading"
                class="mt-1 text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100"
              >
                Environment Overview
              </h2>

              <div class="mt-4 grid grid-cols-1 md:grid-cols-3 gap-4">
                <div class="space-y-1">
                  <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Total resources</p>
                  <p class="text-2xl sm:text-3xl font-semibold font-mono text-gray-900 dark:text-gray-100">
                    {overview().health.totalResources}
                  </p>
                </div>

                <div class="space-y-2">
                  <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">
                    Status distribution
                  </p>
                  <div class="flex flex-wrap gap-2">
                    <For each={statusDistribution()}>
                      {(status) => (
                        <span class={statusBadgeClass(status.tone)}>
                          <span class="font-mono">{status.count}</span>
                          <span>{status.label}</span>
                        </span>
                      )}
                    </For>
                  </div>
                </div>

                <div class="space-y-2">
                  <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Alert summary</p>
                  <div class="flex flex-wrap gap-2">
                    <span class={statusBadgeClass('critical')}>
                      <span class="font-mono">{overview().health.criticalAlerts}</span>
                      <span>Critical alerts</span>
                    </span>
                    <span class={statusBadgeClass('warning')}>
                      <span class="font-mono">{overview().health.warningAlerts}</span>
                      <span>Warning alerts</span>
                    </span>
                  </div>
                </div>
              </div>
            </section>

            {actionItems().length > 0 && (
              <section
                class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`}
                aria-labelledby="action-queue-heading"
              >
                <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">DP-ACTION</p>
                <h2
                  id="action-queue-heading"
                  class="mt-1 text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100"
                >
                  Needs Action
                </h2>
                <div class="mt-2 mb-3 border-b border-gray-100 dark:border-gray-700/50" />

                <ul class="space-y-2" role="list">
                  <For each={displayedActions()}>
                    {(item) => (
                      <li class="flex items-start gap-2.5">
                        <span
                          class={`mt-0.5 inline-flex shrink-0 items-center rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase ${priorityBadgeClass(item.priority)}`}
                        >
                          {item.priority}
                        </span>
                        <a
                          href={item.link}
                          class="text-sm text-gray-700 dark:text-gray-200 hover:text-blue-600 dark:hover:text-blue-400 hover:underline truncate"
                        >
                          {item.label}
                        </a>
                      </li>
                    )}
                  </For>
                </ul>

                {overflowCount() > 0 && (
                  <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                    <a
                      href={ALERTS_OVERVIEW_PATH}
                      class="text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 hover:underline"
                    >
                      and {overflowCount()} more…
                    </a>
                  </p>
                )}
              </section>
            )}

            <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <section
                class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`}
                aria-labelledby="infrastructure-panel-heading"
                aria-busy={trendsLoading()}
              >
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">DP-INFRA</p>
                    <h2
                      id="infrastructure-panel-heading"
                      class="mt-1 text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100"
                    >
                      Infrastructure
                    </h2>
                  </div>
                  <a
                    href={INFRASTRUCTURE_PATH}
                    aria-label="View all infrastructure"
                    class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    View all →
                  </a>
                </div>
                <div class="mt-2 mb-3 border-b border-gray-100 dark:border-gray-700/50" />

                <div class="space-y-4">
                  <div class="space-y-1">
                    <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">
                      Total infrastructure
                    </p>
                    <p class="text-lg sm:text-xl font-semibold font-mono text-gray-900 dark:text-gray-100">
                      {overview().infrastructure.total}
                    </p>
                    <div class="flex flex-wrap items-center gap-3 text-xs sm:text-sm">
                      <span class="font-medium text-emerald-600 dark:text-emerald-400">
                        Online {overview().infrastructure.byStatus.online ?? 0}
                      </span>
                      <span class="font-medium text-gray-500 dark:text-gray-400">
                        Offline {overview().infrastructure.byStatus.offline ?? 0}
                      </span>
                    </div>
                  </div>

                  <Switch>
                    <Match when={overview().infrastructure.total === 0}>
                      <p class="text-sm text-gray-500 dark:text-gray-400">No infrastructure resources</p>
                    </Match>
                    <Match when={overview().infrastructure.total > 0}>
                      <div class="space-y-4">
                        <div class="space-y-2">
                          <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">
                            Top CPU consumers
                          </p>
                          <Switch>
                            <Match when={infrastructureTopCPU().length === 0}>
                              <p class="text-sm text-gray-500 dark:text-gray-400">No CPU metrics available</p>
                            </Match>
                            <Match when={infrastructureTopCPU().length > 0}>
                              <ul class="space-y-2">
                                <For each={infrastructureTopCPU()}>
                                  {(entry) => (
                                    <li class="space-y-1">
                                      <div class="flex items-center justify-between gap-3">
                                        <span class="truncate text-sm text-gray-700 dark:text-gray-200">{entry.name}</span>
                                        <span class="text-xs sm:text-sm font-mono font-semibold text-gray-700 dark:text-gray-200">
                                          {formatPercent(entry.percent)}
                                        </span>
                                      </div>
                                      <div class="h-2 overflow-hidden rounded bg-gray-100 dark:bg-gray-700/70">
                                        <div
                                          class={`h-full rounded ${getMetricColorClass(entry.percent, 'cpu')}`}
                                          style={{
                                            width: `${Math.max(0, Math.min(100, entry.percent))}%`,
                                          }}
                                        />
                                      </div>
                                      {(() => {
                                        const trendData: TrendData | undefined = trends().infrastructure.cpu.get(entry.id);
                                        if (!trendData || trendData.points.length < 2) return (
                                          <p class="text-[10px] text-gray-400 dark:text-gray-500" aria-label="No trend data available">
                                            No trend data
                                          </p>
                                        );
                                        return (
                                          <div class="flex items-center gap-2 mt-1">
                                            <div class="flex-1 min-w-0">
                                              <Sparkline data={trendData.points} metric="cpu" width={0} height={20} />
                                            </div>
                                            <span
                                              class={`text-[10px] font-mono font-medium whitespace-nowrap ${deltaColorClass(trendData.delta)}`}
                                              aria-label={`CPU trend ${formatDelta(trendData.delta) ?? 'unavailable'} over 1 hour`}
                                            >
                                              {formatDelta(trendData.delta) ?? '—'}
                                            </span>
                                          </div>
                                        );
                                      })()}
                                    </li>
                                  )}
                                </For>
                              </ul>
                            </Match>
                          </Switch>
                        </div>

                        <div class="space-y-2">
                          <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">
                            Top memory consumers
                          </p>
                          <Switch>
                            <Match when={infrastructureTopMemory().length === 0}>
                              <p class="text-sm text-gray-500 dark:text-gray-400">No memory metrics available</p>
                            </Match>
                            <Match when={infrastructureTopMemory().length > 0}>
                              <ul class="space-y-2">
                                <For each={infrastructureTopMemory()}>
                                  {(entry) => (
                                    <li class="space-y-1">
                                      <div class="flex items-center justify-between gap-3">
                                        <span class="truncate text-sm text-gray-700 dark:text-gray-200">{entry.name}</span>
                                        <span class="text-xs sm:text-sm font-mono font-semibold text-gray-700 dark:text-gray-200">
                                          {formatPercent(entry.percent)}
                                        </span>
                                      </div>
                                      <div class="h-2 overflow-hidden rounded bg-gray-100 dark:bg-gray-700/70">
                                        <div
                                          class={`h-full rounded ${getMetricColorClass(entry.percent, 'memory')}`}
                                          style={{
                                            width: `${Math.max(0, Math.min(100, entry.percent))}%`,
                                          }}
                                        />
                                      </div>
                                      {(() => {
                                        const trendData: TrendData | undefined = trends().infrastructure.memory.get(entry.id);
                                        if (!trendData || trendData.points.length < 2) return (
                                          <p class="text-[10px] text-gray-400 dark:text-gray-500" aria-label="No trend data available">
                                            No trend data
                                          </p>
                                        );
                                        return (
                                          <div class="flex items-center gap-2 mt-1">
                                            <div class="flex-1 min-w-0">
                                              <Sparkline data={trendData.points} metric="memory" width={0} height={20} />
                                            </div>
                                            <span
                                              class={`text-[10px] font-mono font-medium whitespace-nowrap ${deltaColorClass(trendData.delta)}`}
                                              aria-label={`Memory trend ${formatDelta(trendData.delta) ?? 'unavailable'} over 1 hour`}
                                            >
                                              {formatDelta(trendData.delta) ?? '—'}
                                            </span>
                                          </div>
                                        );
                                      })()}
                                    </li>
                                  )}
                                </For>
                              </ul>
                            </Match>
                          </Switch>
                        </div>
                      </div>
                    </Match>
                  </Switch>
                </div>
              </section>

              <section
                class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`}
                aria-labelledby="workloads-panel-heading"
              >
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">DP-WORK</p>
                    <h2
                      id="workloads-panel-heading"
                      class="mt-1 text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100"
                    >
                      Workloads
                    </h2>
                  </div>
                  <a
                    href={WORKLOADS_PATH}
                    aria-label="View all workloads"
                    class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    View all →
                  </a>
                </div>
                <div class="mt-2 mb-3 border-b border-gray-100 dark:border-gray-700/50" />

                <div class="space-y-4">
                  <div class="space-y-1">
                    <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">
                      Total workloads
                    </p>
                    <p class="text-lg sm:text-xl font-semibold font-mono text-gray-900 dark:text-gray-100">
                      {overview().workloads.total}
                    </p>
                    <div class="flex flex-wrap items-center gap-3 text-xs sm:text-sm">
                      <span class="font-medium text-emerald-600 dark:text-emerald-400">
                        Running {overview().workloads.running}
                      </span>
                      <span class="font-medium text-gray-500 dark:text-gray-400">
                        Stopped {overview().workloads.stopped}
                      </span>
                    </div>
                  </div>

                  <Switch>
                    <Match when={overview().workloads.total === 0}>
                      <p class="text-sm text-gray-500 dark:text-gray-400">No workloads</p>
                    </Match>
                    <Match when={overview().workloads.total > 0}>
                      <div class="space-y-4">
                        <div class="space-y-2">
                          <div class="flex items-center justify-between gap-3">
                            <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">
                              Running vs stopped
                            </p>
                            <span class="text-xs sm:text-sm font-mono font-semibold text-gray-700 dark:text-gray-200">
                              {formatPercent(workloadRunningPercent())} running
                            </span>
                          </div>
                          <div class="h-2 overflow-hidden rounded bg-gray-100 dark:bg-gray-700/70">
                            <div class="flex h-full w-full">
                              <div
                                class="h-full bg-emerald-500 dark:bg-emerald-400"
                                style={{
                                  width: `${workloadRunningPercent()}%`,
                                }}
                              />
                              <div
                                class="h-full bg-gray-400 dark:bg-gray-500"
                                style={{
                                  width: `${workloadStoppedPercent()}%`,
                                }}
                              />
                            </div>
                          </div>
                        </div>

                        <div class="space-y-2">
                          <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Type breakdown</p>
                          <ul class="space-y-1">
                            <For each={workloadTypes()}>
                              {([type, count]) => (
                                <li class="flex items-center justify-between gap-3 text-sm">
                                  <span class="text-gray-700 dark:text-gray-200">{type}</span>
                                  <span class="font-mono font-semibold text-gray-700 dark:text-gray-200">{count}</span>
                                </li>
                              )}
                            </For>
                          </ul>
                        </div>
                      </div>
                    </Match>
                  </Switch>
                </div>
              </section>

              <section
                class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`}
                aria-labelledby="storage-panel-heading"
                aria-busy={trendsLoading()}
              >
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">DP-STORE</p>
                    <h2 id="storage-panel-heading" class="mt-1 text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">
                      Storage
                    </h2>
                  </div>
                  <a
                    href={buildStoragePath()}
                    aria-label="View all storage"
                    class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    View all →
                  </a>
                </div>
                <div class="mt-2 mb-3 border-b border-gray-100 dark:border-gray-700/50" />

                <div class="space-y-4">
                  <div class="space-y-1">
                    <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Total pools</p>
                    <p class="text-lg sm:text-xl font-semibold font-mono text-gray-900 dark:text-gray-100">
                      {overview().storage.total}
                    </p>
                  </div>

                  <Switch>
                    <Match when={overview().storage.total === 0}>
                      <p class="text-sm text-gray-500 dark:text-gray-400">No storage resources</p>
                    </Match>
                    <Match when={overview().storage.total > 0}>
                      <div class="space-y-2">
                        <div class="flex items-center justify-between gap-3">
                          <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">
                            Overall capacity
                          </p>
                          <span class="text-xs sm:text-sm font-mono font-semibold text-gray-700 dark:text-gray-200">
                            {formatPercent(storageCapacityPercent())}
                          </span>
                        </div>
                        <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">
                          {formatBytes(overview().storage.totalUsed)} / {formatBytes(overview().storage.totalCapacity)}
                        </p>
                        <div class="h-2 overflow-hidden rounded bg-gray-100 dark:bg-gray-700/70">
                          <div
                            class={`h-full rounded ${getMetricColorClass(storageCapacityPercent(), 'disk')}`}
                            style={{
                              width: `${storageCapacityPercent()}%`,
                            }}
                          />
                        </div>
                        <div class="flex flex-wrap items-center gap-3 text-xs sm:text-sm">
                          {overview().storage.warningCount > 0 && (
                            <span class="font-medium text-amber-600 dark:text-amber-400">
                              Warnings {overview().storage.warningCount}
                            </span>
                          )}
                          {overview().storage.criticalCount > 0 && (
                            <span class="font-medium text-red-600 dark:text-red-400">
                              Critical {overview().storage.criticalCount}
                            </span>
                          )}
                        </div>
                        {(() => {
                          const storageTrend: TrendData | null = trends().storage.capacity;
                          if (!storageTrend || storageTrend.points.length < 2) return (
                            <p class="text-[10px] text-gray-400 dark:text-gray-500 mt-2" aria-label="No storage trend data available">
                              No trend data
                            </p>
                          );
                          return (
                            <div class="mt-3 space-y-1">
                              <div class="flex items-center justify-between">
                                <p class="text-[10px] font-medium text-gray-500 dark:text-gray-400">24h capacity trend</p>
                                <span
                                  class={`text-[10px] font-mono font-medium ${deltaColorClass(storageTrend.delta)}`}
                                  aria-label={`Storage capacity trend ${formatDelta(storageTrend.delta) ?? 'unavailable'} over 24 hours`}
                                >
                                  {formatDelta(storageTrend.delta) ?? '—'}
                                </span>
                              </div>
                              <Sparkline data={storageTrend.points} metric="disk" width={0} height={24} />
                            </div>
                          );
                        })()}
                      </div>
                    </Match>
                  </Switch>
                </div>
              </section>

              <section class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`} aria-labelledby="backups-panel-heading">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">DP-BACKUP</p>
                    <h2 id="backups-panel-heading" class="mt-1 text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">
                      Backups
                    </h2>
                  </div>
                  <a
                    href={buildBackupsPath()}
                    aria-label="View all backups"
                    class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    View all →
                  </a>
                </div>
                <div class="mt-2 mb-3 border-b border-gray-100 dark:border-gray-700/50" />

                <div class="space-y-1">
                  <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Summary</p>
                  <p class="text-sm text-gray-700 dark:text-gray-200">
                    Backup details are available on the Backups page
                  </p>
                </div>
              </section>
            </div>

            <section class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`} aria-labelledby="alerts-panel-heading">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">DP-ALERTS</p>
                  <h2 id="alerts-panel-heading" class="mt-1 text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">
                    Alerts & Findings
                  </h2>
                </div>
                <a
                  href={ALERTS_OVERVIEW_PATH}
                  aria-label="View all alerts"
                  class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                >
                  View all →
                </a>
              </div>
              <div class="mt-2 mb-3 border-b border-gray-100 dark:border-gray-700/50" />
              <div class="flex flex-wrap items-center gap-6">
                <div class="space-y-1">
                  <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Critical</p>
                  <p class="text-lg sm:text-xl font-semibold font-mono text-red-600 dark:text-red-400">
                    {overview().alerts.activeCritical}
                  </p>
                </div>
                <div class="space-y-1">
                  <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Warning</p>
                  <p class="text-lg sm:text-xl font-semibold font-mono text-amber-600 dark:text-amber-400">
                    {overview().alerts.activeWarning}
                  </p>
                </div>
                <div class="space-y-1">
                  <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Total active</p>
                  <p class="text-lg sm:text-xl font-semibold font-mono text-gray-900 dark:text-gray-100">
                    {overview().alerts.total}
                  </p>
                </div>
                <div class="ml-auto">
                  <a
                    href={AI_PATROL_PATH}
                    aria-label="View patrol findings"
                    class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    View findings →
                  </a>
                </div>
              </div>
            </section>
          </section>
        </Match>
      </Switch>
    </main>
  );
}
