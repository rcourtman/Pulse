import { For, Match, Switch, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { useDashboardOverview } from '@/hooks/useDashboardOverview';
import { useDashboardTrends } from '@/hooks/useDashboardTrends';
import { useDashboardRecovery } from '@/hooks/useDashboardRecovery';
import { useDashboardLayout } from '@/hooks/useDashboardLayout';
import type { HistoryTimeRange } from '@/api/charts';
import type { Alert } from '@/types/api';
import {
  DashboardHero,
  RecentAlertsPanel,
  TrendCharts,
  DashboardCustomizer,
} from './DashboardPanels';
import type { DashboardWidgetDef, DashboardWidgetId } from './DashboardPanels/dashboardWidgets';
export default function Dashboard() {
  const { connected, reconnecting, reconnect, activeAlerts } = useWebSocket();

  // REST-backed resources: instant first paint, no WebSocket wait.
  const dashboardResources = useUnifiedResources({ query: '', cacheKey: 'dashboard-all' });
  const resources = createMemo(() => dashboardResources.resources?.() ?? []);

  const alertsList = createMemo<Alert[]>(() =>
    Object.values(activeAlerts as Record<string, Alert | undefined>).filter(
      (a): a is Alert => a !== undefined,
    ),
  );

  const overview = useDashboardOverview(resources, alertsList);
  const [trendRange, setTrendRange] = createSignal<HistoryTimeRange>('1h');
  const trends = useDashboardTrends(overview, resources, trendRange);
  const recovery = useDashboardRecovery();
  const layout = useDashboardLayout();

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
    () => !isLoading() && !hasConnectionError() && (resources()?.length ?? 0) === 0,
  );

  const storageCapacityPercent = createMemo(() => {
    const { totalUsed, totalCapacity } = overview().storage;
    if (totalCapacity <= 0) return 0;
    return Math.max(0, Math.min(100, (totalUsed / totalCapacity) * 100));
  });

  const renderWidget = (id: DashboardWidgetId) => {
    switch (id) {
      case 'trends':
        return (
          <TrendCharts
            trends={trends()}
            overview={overview()}
            trendRange={trendRange}
            setTrendRange={setTrendRange}
          />
        );
      case 'alerts':
        return (
          <RecentAlertsPanel
            alerts={alertsList()}
            criticalCount={overview().alerts.activeCritical}
            warningCount={overview().alerts.activeWarning}
            totalCount={overview().alerts.total}
          />
        );
      default: {
        const unreachable: never = id;
        return unreachable;
      }
    }
  };

  type WidgetGroup =
    | { type: 'full'; widget: DashboardWidgetDef }
    | { type: 'grid'; widgets: DashboardWidgetDef[] };
  const widgetGroups = createMemo<WidgetGroup[]>(() => {
    const visible = layout.visibleWidgets();
    const result: WidgetGroup[] = [];
    let currentQuarters: DashboardWidgetDef[] = [];

    const flushQuarters = () => {
      if (currentQuarters.length > 0) {
        result.push({ type: 'grid', widgets: currentQuarters });
        currentQuarters = [];
      }
    };

    for (const widget of visible) {
      if (widget.size === 'full') {
        flushQuarters();
        result.push({ type: 'full', widget });
      } else {
        currentQuarters.push(widget);
      }
    }

    flushQuarters();
    return result;
  });

  return (
    <main data-testid="dashboard-page" class="space-y-6">
      <div class="flex justify-end">
        <DashboardCustomizer
          allWidgets={layout.allWidgetsOrdered}
          isHidden={layout.isHidden}
          toggleWidget={layout.toggleWidget}
          moveUp={layout.moveUp}
          moveDown={layout.moveDown}
          resetToDefaults={layout.resetToDefaults}
          isDefault={layout.isDefault}
        />
      </div>

      <Switch>
        <Match when={isLoading() && !initialLoadComplete()}>
          <section class="space-y-2" data-testid="dashboard-loading">
            <div class="border border-border rounded-md p-4 sm:p-5 bg-surface">
              <div class="space-y-4">
                <For each={['h-4 w-44', 'h-10 w-40']}>
                  {(dims) => (
                    <div
                      data-testid="dashboard-skeleton-block"
                      class={`animate-pulse bg-surface-hover rounded ${dims}`}
                    />
                  )}
                </For>
                <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
                  <For each={Array.from({ length: 5 })}>
                    {() => (
                      <div
                        data-testid="dashboard-skeleton-block"
                        class="animate-pulse bg-surface-hover rounded h-8"
                      />
                    )}
                  </For>
                </div>
              </div>
            </div>
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
              <For each={Array.from({ length: 2 })}>
                {() => (
                  <div
                    data-testid="dashboard-skeleton-block"
                    class="border border-border rounded-md p-4 sm:p-5 bg-surface"
                  >
                    <div class="animate-pulse bg-surface-hover rounded h-24" />
                  </div>
                )}
              </For>
            </div>
          </section>
        </Match>

        <Match when={hasConnectionError()}>
          <section class="border border-border rounded-md p-4 sm:p-5 bg-surface" aria-live="polite">
            <h2 class="text-base sm:text-lg font-semibold text-base-content">
              Dashboard unavailable
            </h2>
            <p class="mt-2 text-sm text-muted">
              Real-time dashboard data is currently unavailable. Reconnect to try again.
            </p>
            <button
              type="button"
              onClick={() => reconnect()}
              class="mt-4 inline-flex items-center rounded-md border px-3 py-1.5 text-xs font-medium hover:bg-surface-hover"
            >
              Reconnect
            </button>
          </section>
        </Match>

        <Match when={isEmpty()}>
          <section class="border border-border rounded-md p-4 sm:p-5 bg-surface" aria-live="polite">
            <h2 class="text-base sm:text-lg font-semibold text-base-content">No resources yet</h2>
            <p class="mt-2 text-sm text-muted">
              Once connected platforms report resources, your dashboard overview will appear here.
            </p>
          </section>
        </Match>

        <Match when={initialLoadComplete() && !hasConnectionError() && !isEmpty()}>
          <section class="space-y-5">
            <DashboardHero
              criticalAlerts={overview().health.criticalAlerts}
              warningAlerts={overview().health.warningAlerts}
              infrastructure={{
                total: overview().infrastructure.total,
                online: overview().infrastructure.byStatus.online ?? 0,
                byType: overview().infrastructure.byType,
              }}
              workloads={{
                total: overview().workloads.total,
                running: overview().workloads.running,
                stopped: overview().workloads.stopped,
                byType: overview().workloads.byType,
              }}
              storage={{
                capacityPercent: storageCapacityPercent(),
                totalUsed: overview().storage.totalUsed,
                totalCapacity: overview().storage.totalCapacity,
                warningCount: overview().storage.warningCount,
                criticalCount: overview().storage.criticalCount,
              }}
              alerts={{
                activeCritical: overview().alerts.activeCritical,
                activeWarning: overview().alerts.activeWarning,
                total: overview().alerts.total,
              }}
              recovery={recovery()}
              topCPU={overview().infrastructure.topCPU}
            />
            <For each={widgetGroups()}>
              {(group) =>
                group.type === 'full' ? (
                  renderWidget(group.widget.id)
                ) : (
                  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 items-start">
                    <For each={group.widgets}>{(widget) => renderWidget(widget.id)}</For>
                  </div>
                )
              }
            </For>
          </section>
        </Match>
      </Switch>
    </main>
  );
}
