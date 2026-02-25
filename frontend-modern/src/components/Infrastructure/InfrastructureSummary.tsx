import { Component, For, Show, createMemo, createEffect, createSignal, onCleanup } from 'solid-js';
import { unwrap } from 'solid-js/store';
import { Card } from '@/components/shared/Card';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import { DensityMap } from '@/components/shared/DensityMap';
import { SparklineSkeleton } from '@/components/shared/SparklineSkeleton';
import type { Resource } from '@/types/resource';
import { getDisplayName, getDiskPercent } from '@/types/resource';
import type { MetricPoint, ChartData, TimeRange } from '@/api/charts';
import { useResources } from '@/hooks/useResources';
import {
  fetchInfrastructureSummaryAndCache,
  readInfrastructureSummaryCache,
} from '@/utils/infrastructureSummaryCache';
import {
  SUMMARY_TIME_RANGES,
  SUMMARY_TIME_RANGE_LABEL,
} from '@/components/shared/summaryTimeRange';
import { HOST_COLORS } from '@/pages/DashboardPanels/hostColors';
import { getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';

const normalizeHostIdentifier = (value?: string | null): string[] => {
  if (!value) return [];
  const normalized = value.trim().toLowerCase();
  if (!normalized) return [];
  const variants = new Set<string>([normalized]);
  const dotIndex = normalized.indexOf('.');
  if (dotIndex > 0) {
    variants.add(normalized.slice(0, dotIndex));
  }
  return Array.from(variants);
};

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const asTrimmedString = (value: unknown): string | null => {
  if (typeof value !== 'string') return null;
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
};

const getPlatformDataRecord = (resource: Resource): Record<string, unknown> | undefined =>
  resource.platformData ? (unwrap(resource.platformData) as Record<string, unknown>) : undefined;

const getPlatformAgentRecord = (resource: Resource): Record<string, unknown> | undefined =>
  asRecord(getPlatformDataRecord(resource)?.agent);

const getExplicitAgentIdFromResource = (resource: Resource): string | null => {
  return (
    asTrimmedString(resource.agent?.agentId) ||
    asTrimmedString(getPlatformAgentRecord(resource)?.agentId) ||
    asTrimmedString(getPlatformDataRecord(resource)?.agentId)
  );
};

const getAgentIdFromResource = (resource: Resource): string | null => {
  return (
    getExplicitAgentIdFromResource(resource) ||
    asTrimmedString(resource.discoveryTarget?.resourceId) ||
    asTrimmedString(resource.discoveryTarget?.hostId)
  );
};

const getLinkedNodeIdFromResource = (resource: Resource): string | null =>
  asTrimmedString(getPlatformDataRecord(resource)?.linkedNodeId) ||
  asTrimmedString(getPlatformAgentRecord(resource)?.linkedNodeId);

const getChartKeyCandidates = (resource: Resource): string[] => {
  const candidates = [
    getAgentIdFromResource(resource),
    asTrimmedString(resource.discoveryTarget?.resourceId),
    asTrimmedString(resource.discoveryTarget?.hostId),
    asTrimmedString(resource.id),
    asTrimmedString(resource.name),
    asTrimmedString(resource.platformId),
  ].filter((value): value is string => Boolean(value));

  return Array.from(new Set(candidates));
};

// Format bytes/sec to human-readable rate
const formatRate = (bytesPerSec: number): string => {
  if (bytesPerSec >= 1e9) return `${(bytesPerSec / 1e9).toFixed(1)} GB/s`;
  if (bytesPerSec >= 1e6) return `${(bytesPerSec / 1e6).toFixed(1)} MB/s`;
  if (bytesPerSec >= 1e3) return `${(bytesPerSec / 1e3).toFixed(0)} KB/s`;
  return `${Math.round(bytesPerSec)} B/s`;
};

// Combine a host's net in/out into a single throughput series.
// Buckets points into 30-second windows and sums rates from both directions.
function combineHostNetSeries(inSeries: MetricPoint[], outSeries: MetricPoint[]): MetricPoint[] {
  const bucketSize = 30_000; // 30 seconds
  const buckets = new Map<number, number>();
  for (const p of inSeries) {
    const bucket = Math.round(p.timestamp / bucketSize) * bucketSize;
    buckets.set(bucket, (buckets.get(bucket) || 0) + p.value);
  }
  for (const p of outSeries) {
    const bucket = Math.round(p.timestamp / bucketSize) * bucketSize;
    buckets.set(bucket, (buckets.get(bucket) || 0) + p.value);
  }

  return Array.from(buckets.entries())
    .sort((a, b) => a[0] - b[0])
    .map(([timestamp, value]) => ({ timestamp, value }));
}

interface InfrastructureSummaryProps {
  hosts: Resource[];
  timeRange?: TimeRange;
  hoveredHostId?: string | null;
  focusedHostId?: string | null;
  onTimeRangeChange?: (range: TimeRange) => void;
}

// In-memory full-resolution cache keyed by "org::range".
// Survives component unmount/remount (page navigation) without the
// downsampling artifacts that localStorage cache introduces.
// Capped at MAX_IN_MEMORY_ENTRIES to prevent unbounded growth.
const MAX_IN_MEMORY_INFRA_ENTRIES = 20;
const inMemoryChartCache = new Map<string, Map<string, ChartData>>();

function inMemoryCacheKey(range: TimeRange): string {
  return `${getOrgID() || 'default'}::${range}`;
}

/** @internal Test-only reset for in-memory chart cache. */
export function __resetInMemoryChartCacheForTests(): void {
  inMemoryChartCache.clear();
}

// Clear in-memory cache on org switch to prevent cross-org data leakage.
const unsubscribeInfraOrgSwitch = eventBus.on('org_switched', () => {
  inMemoryChartCache.clear();
});

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    unsubscribeInfraOrgSwitch();
  });
}

export const InfrastructureSummary: Component<InfrastructureSummaryProps> = (props) => {
  // Chart data keyed by resource identifier (node name, host id, etc.)
  const [chartMap, setChartMap] = createSignal<Map<string, ChartData>>(new Map());
  const [chartRange, setChartRange] = createSignal<TimeRange | null>(null);
  const [loadedRange, setLoadedRange] = createSignal<TimeRange | null>(null);
  const [fetchFailed, setFetchFailed] = createSignal(false);
  const selectedRange = createMemo<TimeRange>(() => props.timeRange || '1h');
  const hasCurrentRangeCharts = createMemo(() => chartRange() === selectedRange());
  const isCurrentRangeLoaded = createMemo(() => loadedRange() === selectedRange());

  const { workloads, byType } = useResources();
  const hostAgents = createMemo(() => byType('host'));

  // Track org switches so the effect re-runs when the org changes.
  const [orgVersion, setOrgVersion] = createSignal(0);
  const unsubscribeOrgSwitch = eventBus.on('org_switched', () => {
    setOrgVersion((v) => v + 1);
  });

  // Fetch charts data directly — no dependency on dashboard sparkline store
  let refreshTimer: ReturnType<typeof setInterval> | undefined;
  let activeFetchController: AbortController | null = null;
  let activeFetchRequest = 0;
  let activeScopeKey: string | null = null;

  const awaitAbortable = <T,>(promise: Promise<T>, signal: AbortSignal): Promise<T> => {
    if (signal.aborted) {
      return Promise.reject(new DOMException('Aborted', 'AbortError'));
    }
    return new Promise<T>((resolve, reject) => {
      const onAbort = () => {
        reject(new DOMException('Aborted', 'AbortError'));
      };
      signal.addEventListener('abort', onAbort, { once: true });
      promise.then(
        (value) => {
          signal.removeEventListener('abort', onAbort);
          resolve(value);
        },
        (error) => {
          signal.removeEventListener('abort', onAbort);
          reject(error);
        },
      );
    });
  };

  const fetchCharts = async (options?: { prioritize?: boolean }) => {
    if (props.hosts.length === 0) {
      return;
    }

    const prioritize = options?.prioritize === true;
    if (activeFetchController && !prioritize) {
      // Keep the current request; next timer tick will retry if needed.
      return;
    }
    if (activeFetchController && prioritize) {
      activeFetchController.abort();
    }

    const requestedRange = selectedRange();
    const controller = new AbortController();
    const requestId = ++activeFetchRequest;
    activeFetchController = controller;

    try {
      const fetched = await awaitAbortable(
        fetchInfrastructureSummaryAndCache(requestedRange, { caller: 'InfrastructureSummary' }),
        controller.signal,
      );
      if (requestId !== activeFetchRequest) {
        return;
      }
      const map = fetched.map;

      // If the backend returns an empty payload transiently, keep the last
      // good map to avoid flashing the "no history / static" fallbacks.
      const currentMapMatchesRequestedRange = chartRange() === requestedRange;
      if (map.size > 0 || chartMap().size === 0 || !currentMapMatchesRequestedRange) {
        const cacheKey = inMemoryCacheKey(requestedRange);
        if (
          !inMemoryChartCache.has(cacheKey) &&
          inMemoryChartCache.size >= MAX_IN_MEMORY_INFRA_ENTRIES
        ) {
          const oldest = inMemoryChartCache.keys().next().value;
          if (oldest !== undefined) inMemoryChartCache.delete(oldest);
        }
        inMemoryChartCache.set(cacheKey, map);
        setChartMap(map);
        setChartRange(requestedRange);
      }
      setFetchFailed(false);
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        return;
      }
      if (requestId === activeFetchRequest) {
        setFetchFailed(true);
        // Fall back to localStorage cache on fetch failure.
        if (chartRange() !== requestedRange) {
          const cached = readInfrastructureSummaryCache(requestedRange);
          if (cached) {
            setChartMap(cached.map);
            setChartRange(requestedRange);
          }
        }
      }
    } finally {
      if (activeFetchController === controller) {
        activeFetchController = null;
      }
      if (requestId === activeFetchRequest) {
        setLoadedRange(requestedRange);
      }
    }
  };

  createEffect(() => {
    // Start polling when there are hosts to show. Crucially, do NOT tear down
    // and recreate the interval on every props update, or we end up refetching
    // charts at the websocket update cadence (causing visible UI flashes).
    const hasHosts = props.hosts.length > 0;
    // Read orgVersion to subscribe to org switches.
    const currentOrg = orgVersion();
    if (!hasHosts) {
      if (refreshTimer) {
        clearInterval(refreshTimer);
        refreshTimer = undefined;
      }
      if (activeFetchController) {
        activeFetchController.abort();
        activeFetchController = null;
      }
      activeScopeKey = null;
      setChartMap(new Map());
      setChartRange(null);
      setLoadedRange(null);
      return;
    }

    if (!refreshTimer) {
      refreshTimer = setInterval(() => void fetchCharts(), 30_000);
    }

    const nextRange = selectedRange();
    const nextScopeKey = `${currentOrg}::${nextRange}`;
    if (activeScopeKey !== nextScopeKey) {
      activeScopeKey = nextScopeKey;

      // Hydrate from in-memory cache (full-resolution, no curve shift).
      // Only falls back to skeleton if this range was never fetched in this session.
      const memCached = inMemoryChartCache.get(inMemoryCacheKey(nextRange));
      if (memCached && memCached.size > 0) {
        setChartMap(memCached);
        setChartRange(nextRange);
        setLoadedRange(nextRange);
      } else {
        setChartMap(new Map());
        setChartRange(null);
        setLoadedRange(null);
      }
      setFetchFailed(false);
      void fetchCharts({ prioritize: true });
    }
  });

  onCleanup(() => {
    if (refreshTimer) clearInterval(refreshTimer);
    if (activeFetchController) {
      activeFetchController.abort();
      activeFetchController = null;
    }
    unsubscribeOrgSwitch();
  });

  // Match a unified resource to its chart data.
  // Chart data is keyed by backend composite IDs (e.g. "cluster-pve01" or "instance-pve01")
  // but unified resources have hashed IDs. We reconstruct the composite key or use suffix matching.
  const findChartData = (host: Resource): ChartData | undefined => {
    if (!hasCurrentRangeCharts()) return undefined;
    const map = chartMap();
    if (map.size === 0) return undefined;

    // 1. Agent ID match from unified platform data (most reliable for host agents).
    for (const key of getChartKeyCandidates(host)) {
      const match = map.get(key);
      if (match) return match;
    }

    // 2. Direct matches (works for host agents where IDs may align)
    // Reconstruct composite key for clustered Proxmox nodes: "clusterName-nodeName"
    if (host.clusterId && host.platformId) {
      const clusterKey = `${host.clusterId}-${host.platformId}`;
      const clusterMatch = map.get(clusterKey);
      if (clusterMatch) return clusterMatch;
    }

    // 3. Suffix match for standalone Proxmox nodes: key ends with "-{nodeName}"
    // Handles cases where the instance name prefix is unknown to the frontend
    const nameToMatch = host.platformId || host.name;
    if (nameToMatch) {
      const suffix = `-${nameToMatch}`;
      for (const [key, data] of map) {
        if (key.endsWith(suffix)) return data;
      }
    }

    return undefined;
  };

  // Find chart data from a linked host agent when the primary chart data
  // (typically from nodeData) doesn't include agent-specific metrics like
  // netin/netout/diskread/diskwrite.
  // Host agent resources have internal IDs that match hostData chart keys, and
  // platformData.linkedNodeId + identity.hostname fields that let us correlate
  // with infrastructure resources.
  const findAgentChartData = (host: Resource): ChartData | undefined => {
    if (!hasCurrentRangeCharts()) return undefined;
    const map = chartMap();
    if (map.size === 0) return undefined;

    const directAgentCandidates: string[] = [];
    const explicitAgentId = getExplicitAgentIdFromResource(host);
    if (explicitAgentId) {
      directAgentCandidates.push(explicitAgentId);
    }
    if (host.platformType === 'host-agent') {
      const discoveryResourceId = asTrimmedString(host.discoveryTarget?.resourceId);
      const discoveryHostId = asTrimmedString(host.discoveryTarget?.hostId);
      if (discoveryResourceId) directAgentCandidates.push(discoveryResourceId);
      if (discoveryHostId) directAgentCandidates.push(discoveryHostId);
    }

    for (const key of Array.from(new Set(directAgentCandidates))) {
      const direct = map.get(key);
      if (direct) return direct;
    }

    const agentHosts = hostAgents();
    if (!agentHosts || agentHosts.length === 0) return undefined;

    const nodeRefCandidates = new Set<string>(
      [host.id, host.name, host.platformId]
        .map((value) => value?.trim().toLowerCase())
        .filter((value): value is string => Boolean(value)),
    );
    const hostCandidates = new Set<string>([
      ...normalizeHostIdentifier(host.platformId),
      ...normalizeHostIdentifier(host.name),
      ...normalizeHostIdentifier(host.displayName),
      ...normalizeHostIdentifier(host.identity?.hostname),
    ]);

    // Find a host agent resource that matches this infrastructure resource
    // by linked node ID, hostname, or name
    for (const agentHost of agentHosts) {
      const linkedNodeId = getLinkedNodeIdFromResource(agentHost);
      const normalizedLinkedNodeId = linkedNodeId?.toLowerCase();

      // Match by linked node: agent is linked to a PVE node matching this resource
      const linkedMatch = normalizedLinkedNodeId
        ? nodeRefCandidates.has(normalizedLinkedNodeId)
        : false;
      // Match by hostname: agent hostname matches this resource
      const agentHostName = agentHost.identity?.hostname ?? agentHost.name;
      const agentHostNames = normalizeHostIdentifier(agentHostName);
      const hostnameMatch = agentHostNames.some((candidate) => hostCandidates.has(candidate));

      if (linkedMatch || hostnameMatch) {
        for (const key of getChartKeyCandidates(agentHost)) {
          const agentData = map.get(key);
          if (agentData) return agentData;
        }
      }
    }
    return undefined;
  };

  // Build sparkline series for all hosts
  const hostSeries = createMemo(() => {
    void chartMap(); // reactive dependency
    return props.hosts.map((host, i) => {
      const primaryData = findChartData(host);
      const agentData = findAgentChartData(host);
      const seriesId = host.id || host.platformId || host.name || `host-${i}`;

      const metricSeries = (metric: keyof ChartData): MetricPoint[] => {
        const primary = primaryData?.[metric];
        if (primary && primary.length > 0) return primary;
        const fallback = agentData?.[metric];
        if (fallback && fallback.length > 0) return fallback;
        return [];
      };

      return {
        key: seriesId,
        id: seriesId,
        cpu: metricSeries('cpu'),
        memory: metricSeries('memory'),
        disk: metricSeries('disk'),
        netin: metricSeries('netin'),
        netout: metricSeries('netout'),
        network: combineHostNetSeries(metricSeries('netin'), metricSeries('netout')),
        diskio: combineHostNetSeries(metricSeries('diskread'), metricSeries('diskwrite')),
        color: HOST_COLORS[i % HOST_COLORS.length],
        name: getDisplayName(host),
      };
    });
  });

  // When a resource drawer is open, filter sparklines to show only that resource
  const displaySeries = createMemo(() => {
    const focused = props.focusedHostId;
    const all = hostSeries();
    if (!focused) return all;
    const match = all.find((s) => s.id === focused);
    return match ? [match] : all;
  });

  const focusedHostName = createMemo(() => {
    const focused = props.focusedHostId;
    if (!focused) return null;
    const match = hostSeries().find((s) => s.id === focused);
    return match?.name ?? null;
  });

  const hasData = (metric: 'cpu' | 'memory' | 'disk') =>
    displaySeries().some((s) => s[metric].length >= 1);

  const networkSeries = createMemo(() =>
    displaySeries().map((s) => ({
      id: s.id,
      data: s.network,
      color: s.color,
      name: s.name,
    })),
  );

  const hasNetData = () => displaySeries().some((s) => s.network.length >= 1);

  const diskioSeries = createMemo(() =>
    displaySeries().map((s) => ({
      id: s.id,
      data: s.diskio,
      color: s.color,
      name: s.name,
    })),
  );

  const hasDiskIOData = () => displaySeries().some((s) => s.diskio.length >= 1);

  const avgDiskCapacity = createMemo(() => {
    const hosts = props.hosts.filter((h) => h.disk && h.disk.total);
    if (hosts.length === 0) return null;
    const avg = hosts.reduce((sum, h) => sum + getDiskPercent(h), 0) / hosts.length;
    return Math.round(avg);
  });

  // Keep the network card visible when we have capability but limited history.
  const hasNetworkCapability = createMemo(() =>
    props.hosts.some((host) => {
      const platformData = getPlatformDataRecord(host);
      const sources = (Array.isArray(platformData?.sources) ? platformData.sources : [])
        .map((source) => (typeof source === 'string' ? source.toLowerCase() : ''))
        .filter(Boolean);
      if (sources.includes('agent')) return true;

      // If current-rate metrics are present, treat as network-capable.
      const rx = host.network?.rxBytes ?? 0;
      const tx = host.network?.txBytes ?? 0;
      return rx > 0 || tx > 0;
    }),
  );

  const shouldShowNetworkCard = createMemo(() => hasNetData() || hasNetworkCapability());

  const seriesFor = (metric: 'cpu' | 'memory' | 'disk') =>
    displaySeries().map((s) => ({ id: s.id, data: s[metric], color: s.color, name: s.name }));
  const rangeLabel = () => props.timeRange || '1h';

  const workloadStats = createMemo(() => {
    const all = workloads();
    let running = 0;
    let stopped = 0;
    let vms = 0;
    let containers = 0;
    for (const w of all) {
      if (w.status === 'running' || w.status === 'online') {
        running++;
      } else {
        stopped++;
      }
      if (w.type === 'vm') {
        vms++;
      } else {
        containers++;
      }
    }
    return { total: all.length, running, stopped, vms, containers };
  });

  const hostCounts = createMemo(() => {
    const total = props.hosts.length;
    const online = props.hosts.filter((host) => host.status === 'online').length;
    const offline = total - online;
    return { total, online, offline };
  });

  return (
    <Show when={props.hosts.length > 0}>
      <div data-testid="infrastructure-summary" class="space-y-2">
        <div class="rounded-md border border-border bg-surface p-2 shadow-sm sm:p-3">
          <div class="mb-2 flex flex-wrap items-center justify-between gap-2 border-b border-border-subtle px-1 pb-2 text-[11px] text-slate-500">
            <div class="flex items-center gap-3">
              <span class="font-medium text-base-content">
                {hostCounts().total} {hostCounts().total === 1 ? 'resource' : 'resources'}
              </span>
              <Show when={hostCounts().online > 0}>
                <span class="text-emerald-600 dark:text-emerald-400">
                  {hostCounts().online} online
                </span>
              </Show>
              <Show when={hostCounts().offline > 0}>
                <span class="text-muted">{hostCounts().offline} offline</span>
              </Show>
            </div>
            <Show when={props.onTimeRangeChange}>
              <div class="inline-flex shrink-0 rounded border border-border bg-surface p-0.5 text-xs">
                <For each={SUMMARY_TIME_RANGES}>
                  {(range) => (
                    <button
                      type="button"
                      onClick={() => props.onTimeRangeChange?.(range)}
                      class={`rounded px-2 py-1 ${
                        selectedRange() === range
                          ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                          : 'text-muted hover:bg-surface-hover'
                      }`}
                    >
                      {SUMMARY_TIME_RANGE_LABEL[range]}
                    </button>
                  )}
                </For>
              </div>
            </Show>
          </div>
          <div class="grid gap-2 sm:gap-3 grid-cols-2 lg:grid-cols-4">
            {/* CPU Card */}
            <Card padding="sm" class="h-full">
              <div class="flex flex-col h-full">
                <div class="flex items-center mb-1.5 min-w-0">
                  <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">
                    CPU
                  </span>
                  <Show when={focusedHostName()}>
                    <span class="text-xs text-muted ml-1.5 truncate">
                      &mdash; {focusedHostName()}
                    </span>
                  </Show>
                </div>
                <Show
                  when={hasData('cpu')}
                  fallback={
                    isCurrentRangeLoaded() ? (
                      <div class="text-sm text-muted py-2">
                        {fetchFailed() ? 'Trend data unavailable' : 'No history yet'}
                      </div>
                    ) : (
                      <SparklineSkeleton />
                    )
                  }
                >
                  <div class="flex-1 min-h-0">
                    <InteractiveSparkline
                      series={seriesFor('cpu')}
                      rangeLabel={rangeLabel()}
                      timeRange={props.timeRange}
                      yMode="percent"
                      highlightNearestSeriesOnHover
                      highlightSeriesId={props.hoveredHostId}
                    />
                  </div>
                </Show>
              </div>
            </Card>

            {/* Memory Card */}
            <Card padding="sm" class="h-full">
              <div class="flex flex-col h-full">
                <div class="flex items-center mb-1.5 min-w-0">
                  <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">
                    Memory
                  </span>
                  <Show when={focusedHostName()}>
                    <span class="text-xs text-muted ml-1.5 truncate">
                      &mdash; {focusedHostName()}
                    </span>
                  </Show>
                </div>
                <Show
                  when={hasData('memory')}
                  fallback={
                    isCurrentRangeLoaded() ? (
                      <div class="text-sm text-muted py-2">
                        {fetchFailed() ? 'Trend data unavailable' : 'No history yet'}
                      </div>
                    ) : (
                      <SparklineSkeleton />
                    )
                  }
                >
                  <div class="flex-1 min-h-0">
                    <InteractiveSparkline
                      series={seriesFor('memory')}
                      rangeLabel={rangeLabel()}
                      timeRange={props.timeRange}
                      yMode="percent"
                      highlightNearestSeriesOnHover
                      highlightSeriesId={props.hoveredHostId}
                    />
                  </div>
                </Show>
              </div>
            </Card>

            {/* Disk I/O Card */}
            <Card padding="sm" class="h-full">
              <div class="flex flex-col h-full">
                <div class="flex items-center mb-1.5 min-w-0">
                  <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">
                    Disk I/O
                  </span>
                  <Show when={focusedHostName()}>
                    <span class="text-xs text-muted ml-1.5 truncate">
                      &mdash; {focusedHostName()}
                    </span>
                  </Show>
                  <Show when={!focusedHostName() && avgDiskCapacity() !== null}>
                    <span class="text-[10px] text-muted ml-auto shrink-0">
                      Capacity: {avgDiskCapacity()}%
                    </span>
                  </Show>
                </div>
                <Show
                  when={hasDiskIOData()}
                  fallback={
                    isCurrentRangeLoaded() ? (
                      <div class="text-sm text-muted py-2">
                        {fetchFailed() ? 'Trend data unavailable' : 'No history yet'}
                      </div>
                    ) : (
                      <SparklineSkeleton />
                    )
                  }
                >
                  <div class="flex-1 min-h-0">
                    <DensityMap
                      series={diskioSeries()}
                      rangeLabel={rangeLabel()}
                      timeRange={props.timeRange}
                      formatValue={formatRate}
                    />
                  </div>
                </Show>
              </div>
            </Card>

            <Show
              when={shouldShowNetworkCard()}
              fallback={
                <Card padding="sm" class="h-full">
                  <div class="flex flex-col h-full">
                    <div class="flex items-center justify-between mb-1.5">
                      <span class="text-xs font-medium text-muted uppercase tracking-wide">
                        Workloads
                      </span>
                      <svg
                        class="w-4 h-4 text-green-500"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        stroke-width="1.5"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M6 6.878V6a2.25 2.25 0 012.25-2.25h7.5A2.25 2.25 0 0118 6v.878m-12 0c.235-.083.487-.128.75-.128h10.5c.263 0 .515.045.75.128m-12 0A2.25 2.25 0 004.5 9v.878m13.5-3A2.25 2.25 0 0119.5 9v.878m0 0a2.246 2.246 0 00-.75-.128H5.25c-.263 0-.515.045-.75.128m15 0A2.25 2.25 0 0121 12v6a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 18v-6c0-.98.626-1.813 1.5-2.122"
                        />
                      </svg>
                    </div>
                    <div class="text-xl sm:text-2xl font-bold text-base-content">
                      {workloadStats().running}
                      <span class="text-sm font-normal text-muted ml-1">running</span>
                    </div>
                    <Show
                      when={workloadStats().total > 0}
                      fallback={
                        <div class="text-[10px] text-muted mt-1">No workloads detected</div>
                      }
                    >
                      <div class="text-[10px] text-muted mt-1">
                        <Show when={workloadStats().vms > 0}>
                          <span>{workloadStats().vms} VMs</span>
                        </Show>
                        <Show when={workloadStats().vms > 0 && workloadStats().containers > 0}>
                          <span class="mx-0.5">&middot;</span>
                        </Show>
                        <Show when={workloadStats().containers > 0}>
                          <span>{workloadStats().containers} containers</span>
                        </Show>
                      </div>
                      <Show when={workloadStats().stopped > 0}>
                        <div class="text-[10px] text-muted">{workloadStats().stopped} stopped</div>
                      </Show>
                    </Show>
                  </div>
                </Card>
              }
            >
              {/* 4th Card: Network */}
              <Card padding="sm" class="h-full">
                <div class="flex flex-col h-full">
                  <div class="flex items-center mb-1.5 min-w-0">
                    <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">
                      Network
                    </span>
                    <Show when={focusedHostName()}>
                      <span class="text-xs text-muted ml-1.5 truncate">
                        &mdash; {focusedHostName()}
                      </span>
                    </Show>
                  </div>
                  <Show
                    when={hasNetData()}
                    fallback={
                      isCurrentRangeLoaded() ? (
                        <div class="text-sm text-muted py-2">No history yet</div>
                      ) : (
                        <SparklineSkeleton />
                      )
                    }
                  >
                    <div class="flex-1 min-h-0">
                      <DensityMap
                        series={networkSeries()}
                        rangeLabel={rangeLabel()}
                        timeRange={props.timeRange}
                        formatValue={formatRate}
                      />
                    </div>
                  </Show>
                </div>
              </Card>
            </Show>
          </div>
        </div>
      </div>
    </Show>
  );
};

export default InfrastructureSummary;
