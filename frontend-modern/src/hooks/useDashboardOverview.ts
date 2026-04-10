import { batch, createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';
import {
  ResourceAPI,
  type DashboardOverviewSummaryProblemResource,
  type DashboardOverviewSummaryResponse,
} from '@/api/resources';
import type { Alert } from '@/types/api';
import type {
  Resource,
  ResourceMetric,
  ResourceMetricsTarget,
  ResourceStatus,
  ResourceType,
} from '@/types/resource';
import { isInfrastructure, isStorage, isWorkload } from '@/types/resource';
import { getOrgID } from '@/utils/apiClient';
import { METRIC_THRESHOLDS, getMetricSeverity } from '@/utils/metricThresholds';
import { normalizeOrgScope } from '@/utils/orgScope';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import { resolvePlatformTypeFromSources, resolveSourceTypeFromSources } from '@/utils/sourcePlatforms';
import { OFFLINE_HEALTH_STATUSES, DEGRADED_HEALTH_STATUSES } from '@/utils/status';
import { eventBus } from '@/stores/events';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';

export interface ProblemResource {
  resource: Resource;
  problems: string[];
  worstValue: number; // 0–100 for metrics, 200 for offline, 150 for degraded
}

export interface DashboardOverviewSummary {
  health: {
    totalResources: number;
    byStatus: Record<string, number>;
  };
  infrastructure: {
    total: number;
    byStatus: Record<string, number>;
    byType: Record<string, number>;
    topCPU: Array<{ id: string; name: string; percent: number; metricsTarget?: ResourceMetricsTarget }>;
    topMemory: Array<{ id: string; name: string; percent: number; metricsTarget?: ResourceMetricsTarget }>;
  };
  workloads: {
    total: number;
    running: number;
    stopped: number;
    byType: Record<string, number>;
  };
  storage: {
    total: number;
    totalCapacity: number;
    totalUsed: number;
    warningCount: number;
    criticalCount: number;
  };
  problemResources: ProblemResource[];
}

export interface DashboardOverview extends DashboardOverviewSummary {
  alerts: {
    activeCritical: number;
    activeWarning: number;
    total: number;
  };
  health: DashboardOverviewSummary['health'] & {
    criticalAlerts: number;
    warningAlerts: number;
  };
}

const RESOURCE_STATUSES: ResourceStatus[] = [
  'online',
  'offline',
  'running',
  'stopped',
  'degraded',
  'paused',
  'unknown',
];

const DASHBOARD_OVERVIEW_CACHE_MAX_AGE_MS = 15_000;
const DASHBOARD_OVERVIEW_WS_DEBOUNCE_MS = 800;
const DASHBOARD_OVERVIEW_WS_MIN_REFETCH_INTERVAL_MS = 2_500;

type DashboardOverviewCacheEntry = {
  summary: DashboardOverviewSummary;
  hasSnapshot: boolean;
  cachedAt: number;
  lastFetchAt: number;
  sharedFetch: Promise<DashboardOverviewSummary> | null;
};

const dashboardOverviewCaches = new Map<string, DashboardOverviewCacheEntry>();

function createStatusCounter(): Record<string, number> {
  return RESOURCE_STATUSES.reduce<Record<string, number>>((acc, status) => {
    acc[status] = 0;
    return acc;
  }, {});
}

function createEmptyDashboardOverviewSummary(): DashboardOverviewSummary {
  return {
    health: {
      totalResources: 0,
      byStatus: createStatusCounter(),
    },
    infrastructure: {
      total: 0,
      byStatus: createStatusCounter(),
      byType: {},
      topCPU: [],
      topMemory: [],
    },
    workloads: {
      total: 0,
      running: 0,
      stopped: 0,
      byType: {},
    },
    storage: {
      total: 0,
      totalCapacity: 0,
      totalUsed: 0,
      warningCount: 0,
      criticalCount: 0,
    },
    problemResources: [],
  };
}

function incrementCount(counter: Record<string, number>, key: string): void {
  counter[key] = (counter[key] ?? 0) + 1;
}

function getMetricPercent(metric?: ResourceMetric): number | null {
  if (!metric) return null;

  if (
    typeof metric.total === 'number' &&
    typeof metric.used === 'number' &&
    Number.isFinite(metric.total) &&
    Number.isFinite(metric.used) &&
    metric.total > 0
  ) {
    return (metric.used / metric.total) * 100;
  }

  if (typeof metric.current === 'number' && Number.isFinite(metric.current)) {
    return metric.current;
  }

  return null;
}

function buildTopInfrastructureMetrics(
  resources: Resource[],
  metric: 'cpu' | 'memory',
): Array<{ id: string; name: string; percent: number }> {
  const rows = resources
    .map((resource) => {
      const percent =
        metric === 'cpu' ? getMetricPercent(resource.cpu) : getMetricPercent(resource.memory);
      if (percent === null) return null;

      return {
        id: resource.id,
        name: getPreferredResourceDisplayName(resource),
        percent,
      };
    })
    .filter((row): row is { id: string; name: string; percent: number } => row !== null);

  rows.sort((a, b) => b.percent - a.percent);
  return rows.slice(0, 5);
}

function buildProblemResources(resources: Resource[]): ProblemResource[] {
  const results: ProblemResource[] = [];

  for (const resource of resources) {
    const problems: string[] = [];
    let worstValue = 0;

    const status = resource.status;
    if (OFFLINE_HEALTH_STATUSES.has(status)) {
      problems.push('Offline');
      worstValue = 200;
    } else if (DEGRADED_HEALTH_STATUSES.has(status)) {
      problems.push('Degraded');
      worstValue = Math.max(worstValue, 150);
    }

    const cpuPercent = getMetricPercent(resource.cpu);
    if (cpuPercent !== null && getMetricSeverity(cpuPercent, 'cpu') === 'critical') {
      problems.push(`CPU ${Math.round(cpuPercent)}%`);
      worstValue = Math.max(worstValue, cpuPercent);
    }

    const memPercent = getMetricPercent(resource.memory);
    if (memPercent !== null && getMetricSeverity(memPercent, 'memory') === 'critical') {
      problems.push(`Memory ${Math.round(memPercent)}%`);
      worstValue = Math.max(worstValue, memPercent);
    }

    const diskPercent = getMetricPercent(resource.disk);
    if (diskPercent !== null && getMetricSeverity(diskPercent, 'disk') === 'critical') {
      problems.push(`Disk ${Math.round(diskPercent)}%`);
      worstValue = Math.max(worstValue, diskPercent);
    }

    if (problems.length > 0) {
      results.push({ resource, problems, worstValue });
    }
  }

  results.sort((a, b) => b.worstValue - a.worstValue);
  return results.slice(0, 8);
}

function countAlerts(alerts: Alert[]) {
  return alerts.reduce(
    (acc, alert) => {
      if (alert.level === 'critical') acc.critical += 1;
      if (alert.level === 'warning') acc.warning += 1;
      return acc;
    },
    { critical: 0, warning: 0 },
  );
}

function mergeDashboardAlertCounts(
  summary: DashboardOverviewSummary,
  alerts: Alert[],
): DashboardOverview {
  const alertCounts = countAlerts(alerts);
  return {
    ...summary,
    health: {
      ...summary.health,
      criticalAlerts: alertCounts.critical,
      warningAlerts: alertCounts.warning,
    },
    alerts: {
      activeCritical: alertCounts.critical,
      activeWarning: alertCounts.warning,
      total: alerts.length,
    },
  };
}

function hasFreshDashboardOverviewCache(entry: DashboardOverviewCacheEntry) {
  return entry.hasSnapshot && Date.now() - entry.cachedAt <= DASHBOARD_OVERVIEW_CACHE_MAX_AGE_MS;
}

function setDashboardOverviewCache(
  entry: DashboardOverviewCacheEntry,
  summary: DashboardOverviewSummary,
  at = Date.now(),
) {
  entry.summary = summary;
  entry.hasSnapshot = true;
  entry.cachedAt = at;
}

function getDashboardOverviewCacheEntry(cacheKey: string): DashboardOverviewCacheEntry {
  const existing = dashboardOverviewCaches.get(cacheKey);
  if (existing) {
    return existing;
  }

  const created: DashboardOverviewCacheEntry = {
    summary: createEmptyDashboardOverviewSummary(),
    hasSnapshot: false,
    cachedAt: 0,
    lastFetchAt: 0,
    sharedFetch: null,
  };
  dashboardOverviewCaches.set(cacheKey, created);
  return created;
}

function toSummaryProblemResource(
  problem: DashboardOverviewSummaryProblemResource,
): ProblemResource {
  const sources = (problem.sources ?? []).filter(
    (source): source is string => typeof source === 'string' && source.trim().length > 0,
  );
  const lastSeen = problem.lastSeen ? Date.parse(problem.lastSeen) : NaN;
  const canonical = problem.canonicalIdentity;
  const displayName = canonical?.displayName?.trim() || problem.name?.trim() || problem.id;

  return {
    resource: {
      id: problem.id,
      type: problem.type as ResourceType,
      name: displayName,
      displayName,
      platformId: canonical?.platformId?.trim() || problem.id,
      platformType: resolvePlatformTypeFromSources(sources) || 'agent',
      sourceType: resolveSourceTypeFromSources(sources),
      status: (problem.status?.trim().toLowerCase() || 'unknown') as ResourceStatus,
      lastSeen: Number.isFinite(lastSeen) ? lastSeen : 0,
      canonicalIdentity: canonical,
      policy: problem.policy,
      aiSafeSummary: problem.aiSafeSummary,
    },
    problems: Array.isArray(problem.problems) ? problem.problems : [],
    worstValue: Number.isFinite(problem.worstValue) ? problem.worstValue : 0,
  };
}

function normalizeSummaryResponse(
  response: DashboardOverviewSummaryResponse,
): DashboardOverviewSummary {
  const empty = createEmptyDashboardOverviewSummary();
  const healthByStatus = createStatusCounter();
  const infrastructureByStatus = createStatusCounter();

  for (const [status, count] of Object.entries(response.health?.byStatus ?? {})) {
    healthByStatus[status] = Number.isFinite(count) ? count : 0;
  }
  for (const [status, count] of Object.entries(response.infrastructure?.byStatus ?? {})) {
    infrastructureByStatus[status] = Number.isFinite(count) ? count : 0;
  }

  return {
    health: {
      totalResources: response.health?.totalResources ?? 0,
      byStatus: healthByStatus,
    },
    infrastructure: {
      total: response.infrastructure?.total ?? 0,
      byStatus: infrastructureByStatus,
      byType: { ...(response.infrastructure?.byType ?? {}) },
      topCPU: Array.isArray(response.infrastructure?.topCPU) ? response.infrastructure.topCPU : [],
      topMemory: Array.isArray(response.infrastructure?.topMemory)
        ? response.infrastructure.topMemory
        : [],
    },
    workloads: {
      total: response.workloads?.total ?? 0,
      running: response.workloads?.running ?? 0,
      stopped: response.workloads?.stopped ?? 0,
      byType: { ...(response.workloads?.byType ?? {}) },
    },
    storage: {
      total: response.storage?.total ?? 0,
      totalCapacity: response.storage?.totalCapacity ?? 0,
      totalUsed: response.storage?.totalUsed ?? 0,
      warningCount: response.storage?.warningCount ?? 0,
      criticalCount: response.storage?.criticalCount ?? 0,
    },
    problemResources: Array.isArray(response.problemResources)
      ? response.problemResources.map(toSummaryProblemResource)
      : empty.problemResources,
  };
}

async function fetchDashboardOverviewSummary(entry: DashboardOverviewCacheEntry, force = false) {
  if (!force && hasFreshDashboardOverviewCache(entry)) {
    return entry.summary;
  }
  if (entry.sharedFetch) {
    return entry.sharedFetch;
  }

  const request = (async () => {
    const response = await ResourceAPI.getDashboardSummary();
    const summary = normalizeSummaryResponse(response);
    const now = Date.now();
    setDashboardOverviewCache(entry, summary, now);
    entry.lastFetchAt = now;
    return summary;
  })();

  entry.sharedFetch = request;
  try {
    return await request;
  } finally {
    if (entry.sharedFetch === request) {
      entry.sharedFetch = null;
    }
  }
}

const shouldThrottleWsRefetch = (entry: DashboardOverviewCacheEntry) =>
  Date.now() - entry.lastFetchAt < DASHBOARD_OVERVIEW_WS_MIN_REFETCH_INTERVAL_MS;

export function computeDashboardOverviewSummary(resources: Resource[]): DashboardOverviewSummary {
  const healthByStatus = createStatusCounter();
  const infrastructureByStatus = createStatusCounter();
  const infrastructureByType: Record<string, number> = {};
  const workloadsByType: Record<string, number> = {};

  const infrastructureResources: Resource[] = [];

  let workloadsRunning = 0;
  let workloadsStopped = 0;
  let storageTotal = 0;
  let storageTotalCapacity = 0;
  let storageTotalUsed = 0;
  let storageWarningCount = 0;
  let storageCriticalCount = 0;

  resources.forEach((resource) => {
    incrementCount(healthByStatus, resource.status);

    if (isInfrastructure(resource)) {
      infrastructureResources.push(resource);
      incrementCount(infrastructureByStatus, resource.status);
      incrementCount(infrastructureByType, resource.type);
    }

    if (isWorkload(resource)) {
      incrementCount(workloadsByType, resource.type);
      if (resource.status === 'online' || resource.status === 'running') {
        workloadsRunning += 1;
      }
      if (resource.status === 'offline' || resource.status === 'stopped') {
        workloadsStopped += 1;
      }
    }

    if (isStorage(resource)) {
      storageTotal += 1;

      if (typeof resource.disk?.total === 'number' && Number.isFinite(resource.disk.total)) {
        storageTotalCapacity += resource.disk.total;
      }
      if (typeof resource.disk?.used === 'number' && Number.isFinite(resource.disk.used)) {
        storageTotalUsed += resource.disk.used;
      }

      const diskPercent = getMetricPercent(resource.disk);
      if (diskPercent !== null) {
        if (diskPercent > METRIC_THRESHOLDS.disk.critical) {
          storageCriticalCount += 1;
        } else if (diskPercent > METRIC_THRESHOLDS.disk.warning) {
          storageWarningCount += 1;
        }
      }
    }
  });

  return {
    health: {
      totalResources: resources.length,
      byStatus: healthByStatus,
    },
    infrastructure: {
      total: infrastructureResources.length,
      byStatus: infrastructureByStatus,
      byType: infrastructureByType,
      topCPU: buildTopInfrastructureMetrics(infrastructureResources, 'cpu'),
      topMemory: buildTopInfrastructureMetrics(infrastructureResources, 'memory'),
    },
    workloads: {
      total: Object.values(workloadsByType).reduce((sum, count) => sum + count, 0),
      running: workloadsRunning,
      stopped: workloadsStopped,
      byType: workloadsByType,
    },
    storage: {
      total: storageTotal,
      totalCapacity: storageTotalCapacity,
      totalUsed: storageTotalUsed,
      warningCount: storageWarningCount,
      criticalCount: storageCriticalCount,
    },
    problemResources: buildProblemResources(resources),
  };
}

export function computeDashboardOverview(
  resources: Resource[],
  activeAlerts: Alert[],
): DashboardOverview {
  return mergeDashboardAlertCounts(computeDashboardOverviewSummary(resources), activeAlerts);
}

export function useDashboardOverview(alerts: Accessor<Alert[]>) {
  const [orgScope, setOrgScope] = createSignal(normalizeOrgScope(getOrgID()));
  const resolveScopedCacheKey = () => `dashboard-overview:${orgScope()}`;
  let cacheEntry = getDashboardOverviewCacheEntry(resolveScopedCacheKey());
  const [summary, setSummary] = createSignal<DashboardOverviewSummary>(cacheEntry.summary);
  const [loading, setLoading] = createSignal(!cacheEntry.hasSnapshot);
  const [error, setError] = createSignal<unknown>(undefined);
  const wsStore = getGlobalWebSocketStore();
  let refreshHandle: ReturnType<typeof setTimeout> | undefined;
  let wsInitialized = false;
  let lastWsUpdateToken = '';
  let scopeVersion = 0;

  const runRefetch = async (options?: { force?: boolean; source?: 'initial' | 'ws' | 'manual' }) => {
    const force = options?.force === true;
    const source = options?.source ?? 'manual';

    if (!force && source === 'ws' && shouldThrottleWsRefetch(cacheEntry)) {
      return summary();
    }

    const shouldShowLoading = force || !cacheEntry.hasSnapshot;
    if (shouldShowLoading) {
      setLoading(true);
    }

    const requestVersion = scopeVersion;
    const entryForRequest = cacheEntry;
    try {
      const fetched = await fetchDashboardOverviewSummary(entryForRequest, force);
      if (requestVersion !== scopeVersion || entryForRequest !== cacheEntry) {
        return summary();
      }
      batch(() => {
        setSummary(fetched);
        setError(undefined);
      });
      return fetched;
    } catch (err) {
      setError(err);
      throw err;
    } finally {
      if (shouldShowLoading) {
        setLoading(false);
      }
    }
  };

  const refetch = async () => runRefetch({ force: true, source: 'manual' });

  if (!hasFreshDashboardOverviewCache(cacheEntry)) {
    void runRefetch({ source: 'initial' }).catch(() => undefined);
  }

  const scheduleRefetch = () => {
    if (refreshHandle !== undefined) {
      clearTimeout(refreshHandle);
    }

    const elapsedSinceFetch = Date.now() - cacheEntry.lastFetchAt;
    const minIntervalDelay = Math.max(
      0,
      DASHBOARD_OVERVIEW_WS_MIN_REFETCH_INTERVAL_MS - elapsedSinceFetch,
    );
    const delay = Math.max(DASHBOARD_OVERVIEW_WS_DEBOUNCE_MS, minIntervalDelay);

    refreshHandle = setTimeout(() => {
      refreshHandle = undefined;
      void runRefetch({ source: 'ws' }).catch(() => undefined);
    }, delay);
  };

  createEffect(() => {
    orgScope();

    if (!wsStore.connected() || !wsStore.initialDataReceived()) {
      wsInitialized = false;
      lastWsUpdateToken = '';
      return;
    }

    const lastUpdateToken = String(wsStore.state.lastUpdate ?? '');
    if (!wsInitialized) {
      wsInitialized = true;
      lastWsUpdateToken = lastUpdateToken;
      return;
    }
    if (lastUpdateToken === lastWsUpdateToken) {
      return;
    }

    lastWsUpdateToken = lastUpdateToken;
    scheduleRefetch();
  });

  const unsubscribeOrgSwitch = eventBus.on('org_switched', (nextOrgID?: string) => {
    const nextOrgScope = normalizeOrgScope(nextOrgID);
    if (nextOrgScope === orgScope()) {
      return;
    }

    scopeVersion += 1;
    setOrgScope(nextOrgScope);
    cacheEntry = getDashboardOverviewCacheEntry(resolveScopedCacheKey());
    wsInitialized = false;
    lastWsUpdateToken = '';

    batch(() => {
      setSummary(cacheEntry.summary);
      setLoading(!cacheEntry.hasSnapshot);
      setError(undefined);
    });

    if (!hasFreshDashboardOverviewCache(cacheEntry)) {
      void runRefetch({ force: true, source: 'initial' }).catch(() => undefined);
    }
  });

  onCleanup(() => {
    unsubscribeOrgSwitch();
    if (refreshHandle !== undefined) {
      clearTimeout(refreshHandle);
    }
  });

  const overview = createMemo(() => mergeDashboardAlertCounts(summary(), alerts()));

  return {
    overview,
    loading,
    error,
    refetch,
  };
}
