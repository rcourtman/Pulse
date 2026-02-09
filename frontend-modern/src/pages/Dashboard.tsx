import { For, Match, Switch, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { useDashboardOverview } from '@/hooks/useDashboardOverview';

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

            <div class="grid grid-cols-1 lg:grid-cols-2 gap-4" />
          </section>
        </Match>
      </Switch>
    </main>
  );
}
