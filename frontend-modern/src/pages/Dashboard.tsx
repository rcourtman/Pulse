import { For, Match, Switch, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { useDashboardOverview } from '@/hooks/useDashboardOverview';
import {
  buildBackupsPath,
  buildStoragePath,
  INFRASTRUCTURE_PATH,
  WORKLOADS_PATH,
} from '@/routing/resourceLinks';
import { formatBytes } from '@/utils/format';
import { METRIC_THRESHOLDS } from '@/utils/metricThresholds';

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

function infrastructureCpuBarClass(percent: number): string {
  const thresholds = METRIC_THRESHOLDS.cpu;
  if (percent > thresholds.critical) return 'bg-red-500/60 dark:bg-red-500/50';
  if (percent > thresholds.warning) return 'bg-yellow-500/60 dark:bg-yellow-500/50';
  return 'bg-green-500/60 dark:bg-green-500/50';
}

function storageCapacityBarClass(percent: number): string {
  if (percent > 90) return 'bg-red-500/60 dark:bg-red-500/50';
  if (percent > 80) return 'bg-yellow-500/60 dark:bg-yellow-500/50';
  return 'bg-green-500/60 dark:bg-green-500/50';
}

function formatPercent(value: number): string {
  return `${Math.round(value)}%`;
}

export default function Dashboard() {
  const { state, connected, reconnecting, reconnect, initialDataReceived } = useWebSocket();
  const overview = useDashboardOverview();
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
  const workloadTypes = createMemo(() => Object.entries(overview().workloads.byType));
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
              <h2
                id="environment-overview-heading"
                class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100"
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

                <div class="md:col-span-2 space-y-2">
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
              </div>

              <div class="mt-4 flex flex-wrap gap-2">
                <span class={statusBadgeClass('critical')}>
                  <span class="font-mono">{overview().health.criticalAlerts}</span>
                  <span>Critical alerts</span>
                </span>
                <span class={statusBadgeClass('warning')}>
                  <span class="font-mono">{overview().health.warningAlerts}</span>
                  <span>Warning alerts</span>
                </span>
              </div>
            </section>

            <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <section
                class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`}
                aria-labelledby="infrastructure-panel-heading"
              >
                <div class="flex items-center justify-between gap-3">
                  <h2
                    id="infrastructure-panel-heading"
                    class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100"
                  >
                    Infrastructure
                  </h2>
                  <a
                    href={INFRASTRUCTURE_PATH}
                    aria-label="View all infrastructure"
                    class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    View all →
                  </a>
                </div>

                <div class="mt-4 space-y-4">
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
                      <div class="space-y-2">
                        <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">
                          Top CPU consumers
                        </p>
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
                                    class={`h-full rounded ${infrastructureCpuBarClass(entry.percent)}`}
                                    style={{
                                      width: `${Math.max(0, Math.min(100, entry.percent))}%`,
                                    }}
                                  />
                                </div>
                              </li>
                            )}
                          </For>
                        </ul>
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
                  <h2
                    id="workloads-panel-heading"
                    class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100"
                  >
                    Workloads
                  </h2>
                  <a
                    href={WORKLOADS_PATH}
                    aria-label="View all workloads"
                    class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    View all →
                  </a>
                </div>

                <div class="mt-4 space-y-4">
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
                    </Match>
                  </Switch>
                </div>
              </section>

              <section class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`} aria-labelledby="storage-panel-heading">
                <div class="flex items-center justify-between gap-3">
                  <h2 id="storage-panel-heading" class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">
                    Storage
                  </h2>
                  <a
                    href={buildStoragePath()}
                    aria-label="View all storage"
                    class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    View all →
                  </a>
                </div>

                <div class="mt-4 space-y-4">
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
                            class={`h-full rounded ${storageCapacityBarClass(storageCapacityPercent())}`}
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
                      </div>
                    </Match>
                  </Switch>
                </div>
              </section>

              <section class={`${PANEL_BASE_CLASS} bg-white dark:bg-gray-800`} aria-labelledby="backups-panel-heading">
                <div class="flex items-center justify-between gap-3">
                  <h2 id="backups-panel-heading" class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">
                    Backups
                  </h2>
                  <a
                    href={buildBackupsPath()}
                    aria-label="View all backups"
                    class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    View all →
                  </a>
                </div>

                <div class="mt-4 space-y-1">
                  <p class="text-xs sm:text-sm font-medium text-gray-600 dark:text-gray-400">Summary</p>
                  <p class="text-sm text-gray-700 dark:text-gray-200">
                    Backup details are available on the Backups page
                  </p>
                </div>
              </section>
            </div>
          </section>
        </Match>
      </Switch>
    </main>
  );
}
