import { For, Match, Switch, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { useDashboardOverview } from '@/hooks/useDashboardOverview';
import { useDashboardTrends } from '@/hooks/useDashboardTrends';
import { useDashboardBackups } from '@/hooks/useDashboardBackups';
import type { HistoryTimeRange } from '@/api/charts';
import type { Alert } from '@/types/api';
import { isInfrastructure, isStorage } from '@/types/resource';
import { ALERTS_OVERVIEW_PATH, buildStoragePath, INFRASTRUCTURE_PATH } from '@/routing/resourceLinks';
import { BackupStatusPanel, CompositionPanel, DashboardHero, RecentAlertsPanel, StoragePanel, TrendCharts } from './DashboardPanels';
import { type ActionItem, MAX_ACTION_ITEMS, PRIORITY_ORDER } from './DashboardPanels/dashboardHelpers';

export default function Dashboard() {
  const { connected, reconnecting, reconnect, activeAlerts } = useWebSocket();

  // REST-backed resources: instant first paint, no WebSocket wait.
  const dashboardResources = useUnifiedResources({ query: '', cacheKey: 'dashboard-all' });
  const resources = createMemo(() => dashboardResources.resources() ?? []);

  const alertsList = createMemo<Alert[]>(() =>
    Object.values(activeAlerts as Record<string, Alert | undefined>).filter((a): a is Alert => a !== undefined),
  );

  const overview = useDashboardOverview(resources, alertsList);
  const [trendRange, setTrendRange] = createSignal<HistoryTimeRange>('1h');
  const trends = useDashboardTrends(overview, resources, trendRange);
  const backups = useDashboardBackups(resources);

  // Loading timeout: if REST fetch takes >30s, treat as connection error.
  const [loadingTimedOut, setLoadingTimedOut] = createSignal(false);
  let loadingTimeout: number | undefined;

  const isLoading = createMemo(() => dashboardResources.loading());

  // Track whether we've completed the initial load so that subsequent
  // background refetches don't tear down the content tree (which causes
  // flickering and scroll-position resets).
  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);
  createEffect(() => {
    if (!isLoading() && !initialLoadComplete()) {
      setInitialLoadComplete(true);
    }
  });

  createEffect(() => {
    if (isLoading()) {
      if (!loadingTimeout) {
        loadingTimeout = window.setTimeout(() => setLoadingTimedOut(true), 30000);
      }
      return;
    }
    if (loadingTimeout) {
      window.clearTimeout(loadingTimeout);
      loadingTimeout = undefined;
    }
    setLoadingTimedOut(false);
  });

  onCleanup(() => {
    if (loadingTimeout) window.clearTimeout(loadingTimeout);
  });

  const hasConnectionError = createMemo(() => {
    if (loadingTimedOut()) return true;
    if (dashboardResources.error()) return true;
    return !isLoading() && !connected() && !reconnecting();
  });

  const isEmpty = createMemo(
    () => !isLoading() && !hasConnectionError() && resources().length === 0,
  );

  const actionItems = createMemo<ActionItem[]>(() => {
    const items: ActionItem[] = [];
    const allResources = resources();
    const alerts = alertsList();

    // Tier 1/4: Alerts
    for (const alert of alerts) {
      if (alert.level === 'critical') {
        items.push({ id: `alert-crit-${alert.id}`, priority: 'critical', label: `Critical alert: ${alert.resourceName} — ${alert.message}`, link: ALERTS_OVERVIEW_PATH });
        continue;
      }
      if (alert.level === 'warning') {
        items.push({ id: `alert-warn-${alert.id}`, priority: 'medium', label: `Warning: ${alert.resourceName} — ${alert.message}`, link: ALERTS_OVERVIEW_PATH });
      }
    }

    // Tier 2: Infrastructure offline
    for (const resource of allResources) {
      if (isInfrastructure(resource) && resource.status === 'offline') {
        items.push({ id: `infra-offline-${resource.id}`, priority: 'high', label: `Offline: ${resource.displayName || resource.name}`, link: INFRASTRUCTURE_PATH });
      }
    }

    // Tier 3/5: Storage capacity
    for (const resource of allResources) {
      if (isStorage(resource) && resource.disk) {
        const diskPercent = typeof resource.disk.total === 'number' && resource.disk.total > 0 ? ((resource.disk.used ?? 0) / resource.disk.total) * 100 : null;
        if (diskPercent === null) continue;

        if (diskPercent >= 90) {
          items.push({ id: `storage-crit-${resource.id}`, priority: 'high', label: `Storage critical: ${resource.displayName || resource.name} at ${Math.round(diskPercent)}%`, link: buildStoragePath() });
          continue;
        }

        if (diskPercent >= 80) {
          items.push({ id: `storage-warn-${resource.id}`, priority: 'medium', label: `Storage warning: ${resource.displayName || resource.name} at ${Math.round(diskPercent)}%`, link: buildStoragePath() });
        }
      }
    }

    // Tier 6: High CPU (>=90%)
    for (const entry of overview().infrastructure.topCPU) {
      if (entry.percent >= 90) {
        items.push({ id: `cpu-crit-${entry.id}`, priority: 'low', label: `High CPU: ${entry.name} at ${Math.round(entry.percent)}%`, link: INFRASTRUCTURE_PATH });
      }
    }

    items.sort((a, b) => {
      const priorityDiff = PRIORITY_ORDER[a.priority] - PRIORITY_ORDER[b.priority];
      if (priorityDiff !== 0) return priorityDiff;
      return a.label.localeCompare(b.label);
    });

    return items;
  });

  const displayedActions = createMemo(() => actionItems().slice(0, MAX_ACTION_ITEMS));


  const storageCapacityPercent = createMemo(() => {
    const { totalUsed, totalCapacity } = overview().storage;
    if (totalCapacity <= 0) return 0;
    return Math.max(0, Math.min(100, (totalUsed / totalCapacity) * 100));
  });

  return (
    <main data-testid="dashboard-page" aria-labelledby="dashboard-title" class="space-y-3">
      <Switch>
        <Match when={isLoading() && !initialLoadComplete()}>
          <section class="space-y-2" data-testid="dashboard-loading">
            <div class="border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm p-4 sm:p-5 bg-white dark:bg-gray-800">
              <div class="space-y-4">
                <For each={['h-4 w-44', 'h-10 w-40']}>
                  {(dims) => <div data-testid="dashboard-skeleton-block" class={`animate-pulse bg-gray-200 dark:bg-gray-700 rounded ${dims}`} />}
                </For>
                <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
                  <For each={Array.from({ length: 5 })}>
                    {() => <div data-testid="dashboard-skeleton-block" class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-8" />}
                  </For>
                </div>
              </div>
            </div>
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
              <For each={Array.from({ length: 2 })}>
                {() => (
                  <div data-testid="dashboard-skeleton-block" class="border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm p-4 sm:p-5 bg-white dark:bg-gray-800"><div class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded h-24" /></div>
                )}
              </For>
            </div>
          </section>
        </Match>

        <Match when={hasConnectionError()}>
          <section class="border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm p-4 sm:p-5 bg-white dark:bg-gray-800" aria-live="polite">
            <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">Dashboard unavailable</h2>
            <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">Real-time dashboard data is currently unavailable. Reconnect to try again.</p>
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
          <section class="border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm p-4 sm:p-5 bg-white dark:bg-gray-800" aria-live="polite">
            <h2 class="text-base sm:text-lg font-semibold text-gray-900 dark:text-gray-100">No resources yet</h2>
            <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">Once connected platforms report resources, your dashboard overview will appear here.</p>
          </section>
        </Match>

        <Match when={initialLoadComplete() && !hasConnectionError() && !isEmpty()}>
          <section class="space-y-5">
            <DashboardHero
              title="Dashboard"
              totalResources={overview().health.totalResources}
              criticalAlerts={overview().health.criticalAlerts}
              warningAlerts={overview().health.warningAlerts}
              byStatus={overview().health.byStatus}
              infrastructure={{
                total: overview().infrastructure.total,
                online: overview().infrastructure.byStatus.online ?? 0,
              }}
              workloads={{
                total: overview().workloads.total,
                running: overview().workloads.running,
              }}
              storage={{
                capacityPercent: storageCapacityPercent(),
                totalUsed: overview().storage.totalUsed,
                totalCapacity: overview().storage.totalCapacity,
              }}
              alerts={{
                activeCritical: overview().alerts.activeCritical,
                activeWarning: overview().alerts.activeWarning,
                total: overview().alerts.total,
              }}
              topIssues={displayedActions()}
            />

            <TrendCharts
              trends={trends()}
              overview={overview()}
              trendRange={trendRange}
              setTrendRange={setTrendRange}
            />

            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 items-start">
              <CompositionPanel
                infrastructureByType={overview().infrastructure.byType}
                workloadsByType={overview().workloads.byType}
              />
              <BackupStatusPanel backups={backups()} />
              <StoragePanel storage={overview().storage} storageTrend={trends().storage.capacity} loading={trends().loading} />
              <RecentAlertsPanel
                alerts={alertsList()}
                criticalCount={overview().alerts.activeCritical}
                warningCount={overview().alerts.activeWarning}
                totalCount={overview().alerts.total}
              />
            </div>
          </section>
        </Match>
      </Switch>
    </main>
  );
}
