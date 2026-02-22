import { Component, For, Show, createMemo, createEffect, createSignal, onCleanup } from 'solid-js';
import { unwrap } from 'solid-js/store';
import { Card } from '@/components/shared/Card';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import { DensityMap } from '@/components/shared/DensityMap';
import { SparklineSkeleton } from '@/components/shared/SparklineSkeleton';
import type { Resource } from '@/types/resource';
import { getDisplayName } from '@/types/resource';
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

const getAgentIdFromResource = (resource: Resource): string | null => {
    const platformData =
        resource.platformData
            ? (unwrap(resource.platformData) as { agent?: { agentId?: string } } | undefined)
            : undefined;
    const agentId = platformData?.agent?.agentId;
    if (!agentId || typeof agentId !== 'string') return null;
    const trimmed = agentId.trim();
    return trimmed.length > 0 ? trimmed : null;
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

export const InfrastructureSummary: Component<InfrastructureSummaryProps> = (props) => {
    // Chart data keyed by resource identifier (node name, host id, etc.)
    const [chartMap, setChartMap] = createSignal<Map<string, ChartData>>(new Map());
    const [chartRange, setChartRange] = createSignal<TimeRange | null>(null);
    const [loadedRange, setLoadedRange] = createSignal<TimeRange | null>(null);
    const selectedRange = createMemo<TimeRange>(() => props.timeRange || '1h');
    const hasCurrentRangeCharts = createMemo(() => chartRange() === selectedRange());
    const isCurrentRangeLoaded = createMemo(() => loadedRange() === selectedRange());

    const { workloads, byType } = useResources();
    const hostAgents = createMemo(() => byType('host'));

    // Fetch charts data directly — no dependency on dashboard sparkline store
    let refreshTimer: ReturnType<typeof setInterval> | undefined;
    let activeFetchController: AbortController | null = null;
    let activeFetchRequest = 0;
    let activeRange: TimeRange | null = null;
    const infraSummaryPerfEnabled = import.meta.env.DEV && import.meta.env.VITE_INFRA_SUMMARY_PERF === '1';

    const hydrateFromRangeCache = (range: TimeRange): boolean => {
        const cached = readInfrastructureSummaryCache(range);
        if (!cached) {
            if (infraSummaryPerfEnabled) {
                console.debug('[InfraSummaryPerf] cache miss', { caller: 'InfrastructureSummary', range });
            }
            return false;
        }

        if (infraSummaryPerfEnabled) {
            const points = Array.from(cached.map.values()).reduce((total, data) => {
                total += data.cpu?.length ?? 0;
                total += data.memory?.length ?? 0;
                total += data.disk?.length ?? 0;
                total += data.netin?.length ?? 0;
                total += data.netout?.length ?? 0;
                return total;
            }, 0);
            console.debug('[InfraSummaryPerf] cache hit', {
                caller: 'InfrastructureSummary',
                range,
                ageMs: Date.now() - cached.cachedAt,
                series: cached.map.size,
                points,
            });
        }

        setChartMap(cached.map);
        setChartRange(range);
        setLoadedRange(range);
        return true;
    };

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
        let requestSucceeded = false;

        try {
            const fetched = await awaitAbortable(
                fetchInfrastructureSummaryAndCache(requestedRange, { caller: 'InfrastructureSummary' }),
                controller.signal,
            );
            if (requestId !== activeFetchRequest) {
                return;
            }
            requestSucceeded = true;
            const map = fetched.map;

            // If the backend returns an empty payload transiently, keep the last
            // good map to avoid flashing the "no history / static" fallbacks.
            const currentMapMatchesRequestedRange = chartRange() === requestedRange;
            if (map.size > 0 || chartMap().size === 0 || !currentMapMatchesRequestedRange) {
                setChartMap(map);
                setChartRange(requestedRange);
            }
        } catch (error) {
            if (error instanceof Error && error.name === 'AbortError') {
                return;
            }
            // Silently degrade — cards show fallback numbers
        } finally {
            if (activeFetchController === controller) {
                activeFetchController = null;
            }
            if (requestId === activeFetchRequest && requestSucceeded) {
                setLoadedRange(requestedRange);
            }
        }
    };

    createEffect(() => {
        // Start polling when there are hosts to show. Crucially, do NOT tear down
        // and recreate the interval on every props update, or we end up refetching
        // charts at the websocket update cadence (causing visible UI flashes).
        const hasHosts = props.hosts.length > 0;
        if (!hasHosts) {
            if (refreshTimer) {
                clearInterval(refreshTimer);
                refreshTimer = undefined;
            }
            if (activeFetchController) {
                activeFetchController.abort();
                activeFetchController = null;
            }
            activeRange = null;
            setChartMap(new Map());
            setChartRange(null);
            setLoadedRange(null);
            return;
        }

        if (!refreshTimer) {
            refreshTimer = setInterval(() => void fetchCharts(), 30_000);
        }

        const nextRange = selectedRange();
        if (activeRange !== nextRange) {
            activeRange = nextRange;
            if (!hydrateFromRangeCache(nextRange)) {
                setLoadedRange((current) => (current === nextRange ? current : null));
            }
            void fetchCharts({ prioritize: true });
        }
    });

    onCleanup(() => {
        if (refreshTimer) clearInterval(refreshTimer);
        if (activeFetchController) {
            activeFetchController.abort();
            activeFetchController = null;
        }
    });

    // Match a unified resource to its chart data.
    // Chart data is keyed by backend composite IDs (e.g. "cluster-pve01" or "instance-pve01")
    // but unified resources have hashed IDs. We reconstruct the composite key or use suffix matching.
    const findChartData = (host: Resource): ChartData | undefined => {
        if (!hasCurrentRangeCharts()) return undefined;
        const map = chartMap();
        if (map.size === 0) return undefined;

        // 1. Agent ID match from unified platform data (most reliable for host agents).
        const agentId = getAgentIdFromResource(host);
        if (agentId) {
            const agentMatch = map.get(agentId);
            if (agentMatch) return agentMatch;
        }

        // 2. Direct matches (works for host agents where IDs may align)
        const direct = map.get(host.id) || map.get(host.name) || map.get(host.platformId);
        if (direct) return direct;

        // 3. Reconstruct composite key for clustered Proxmox nodes: "clusterName-nodeName"
        if (host.clusterId && host.platformId) {
            const clusterKey = `${host.clusterId}-${host.platformId}`;
            const clusterMatch = map.get(clusterKey);
            if (clusterMatch) return clusterMatch;
        }

        // 4. Suffix match for standalone Proxmox nodes: key ends with "-{nodeName}"
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

        const agentId = getAgentIdFromResource(host);
        if (agentId) {
            const direct = map.get(agentId);
            if (direct) return direct;
        }

        const agentHosts = hostAgents();
        if (!agentHosts || agentHosts.length === 0) return undefined;

        const hostCandidates = new Set<string>([
            ...normalizeHostIdentifier(host.platformId),
            ...normalizeHostIdentifier(host.name),
            ...normalizeHostIdentifier(host.displayName),
            ...normalizeHostIdentifier(host.identity?.hostname),
        ]);

        // Find a host agent resource that matches this infrastructure resource
        // by linked node ID, hostname, or name
        for (const agentHost of agentHosts) {
            const platformData =
                agentHost.platformData
                    ? (unwrap(agentHost.platformData) as { linkedNodeId?: string } | undefined)
                    : undefined;
            const linkedNodeId = platformData?.linkedNodeId;

            // Match by linked node: agent is linked to a PVE node matching this resource
            const linkedMatch = linkedNodeId && (
                host.id === linkedNodeId ||
                host.name === linkedNodeId ||
                host.platformId === linkedNodeId
            );
            // Match by hostname: agent hostname matches this resource
            const agentHostName = agentHost.identity?.hostname ?? agentHost.name;
            const agentHostNames = normalizeHostIdentifier(agentHostName);
            const hostnameMatch = agentHostNames.some((candidate) => hostCandidates.has(candidate));

            if (linkedMatch || hostnameMatch) {
                const agentData = map.get(agentHost.id);
                if (agentData) return agentData;
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
        }))
    );

    const hasNetData = () =>
        displaySeries().some((s) => s.network.length >= 1);

    // Keep the network card visible when we have capability but limited history.
    const hasNetworkCapability = createMemo(() =>
        props.hosts.some((host) => {
            const platformData = host.platformData as { sources?: string[] } | undefined;
            const sources = (platformData?.sources ?? []).map((source) => source.toLowerCase());
            if (sources.includes('agent')) return true;

            // If current-rate metrics are present, treat as network-capable.
            const rx = host.network?.rxBytes ?? 0;
            const tx = host.network?.txBytes ?? 0;
            return rx > 0 || tx > 0;
        })
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
                <div class="rounded-md border border-slate-200 bg-white p-2 shadow-sm dark:border-slate-700 dark:bg-slate-800 sm:p-3">
                    <div class="mb-2 flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 px-1 pb-2 text-[11px] text-slate-500 dark:border-slate-700 dark:text-slate-400">
                        <div class="flex items-center gap-3">
                            <span class="font-medium text-base-content">
                                {hostCounts().total} {hostCounts().total === 1 ? 'resource' : 'resources'}
                            </span>
                            <Show when={hostCounts().online > 0}>
                                <span class="text-emerald-600 dark:text-emerald-400">{hostCounts().online} online</span>
                            </Show>
                            <Show when={hostCounts().offline > 0}>
                                <span class="text-muted">{hostCounts().offline} offline</span>
                            </Show>
                        </div>
                        <Show when={props.onTimeRangeChange}>
                            <div class="inline-flex shrink-0 rounded border border-slate-300 bg-white p-0.5 text-xs dark:border-slate-700 dark:bg-slate-900">
                                <For each={SUMMARY_TIME_RANGES}>
                                    {(range) => (
                                        <button
                                            type="button"
                                            onClick={() => props.onTimeRangeChange?.(range)}
                                            class={`rounded px-2 py-1 ${selectedRange() === range
                                                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                                                : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-700'
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
                                    <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">CPU</span>
                                    <Show when={focusedHostName()}>
                                        <span class="text-xs text-muted ml-1.5 truncate">&mdash; {focusedHostName()}</span>
                                    </Show>
                                </div>
                                <Show
                                    when={hasData('cpu')}
                                    fallback={
                                        isCurrentRangeLoaded() ? (
                                            <div class="text-sm text-muted py-2">
                                                No history yet
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
                                    <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">Memory</span>
                                    <Show when={focusedHostName()}>
                                        <span class="text-xs text-muted ml-1.5 truncate">&mdash; {focusedHostName()}</span>
                                    </Show>
                                </div>
                                <Show
                                    when={hasData('memory')}
                                    fallback={
                                        isCurrentRangeLoaded() ? (
                                            <div class="text-sm text-muted py-2">
                                                No history yet
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

                        {/* Storage Card */}
                        <Card padding="sm" class="h-full">
                            <div class="flex flex-col h-full">
                                <div class="flex items-center mb-1.5 min-w-0">
                                    <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">Storage</span>
                                    <Show when={focusedHostName()}>
                                        <span class="text-xs text-muted ml-1.5 truncate">&mdash; {focusedHostName()}</span>
                                    </Show>
                                </div>
                                <Show
                                    when={hasData('disk')}
                                    fallback={
                                        isCurrentRangeLoaded() ? (
                                            <div class="text-sm text-muted py-2">
                                                No history yet
                                            </div>
                                        ) : (
                                            <SparklineSkeleton />
                                        )
                                    }
                                >
                                    <div class="flex-1 min-h-0">
                                        <InteractiveSparkline
                                            series={seriesFor('disk')}
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

                        <Show
                            when={shouldShowNetworkCard()}
                            fallback={
                                <Card padding="sm" class="h-full">
                                    <div class="flex flex-col h-full">
                                        <div class="flex items-center justify-between mb-1.5">
                                            <span class="text-xs font-medium text-muted uppercase tracking-wide">Workloads</span>
                                            <svg class="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                                <path stroke-linecap="round" stroke-linejoin="round" d="M6 6.878V6a2.25 2.25 0 012.25-2.25h7.5A2.25 2.25 0 0118 6v.878m-12 0c.235-.083.487-.128.75-.128h10.5c.263 0 .515.045.75.128m-12 0A2.25 2.25 0 004.5 9v.878m13.5-3A2.25 2.25 0 0119.5 9v.878m0 0a2.246 2.246 0 00-.75-.128H5.25c-.263 0-.515.045-.75.128m15 0A2.25 2.25 0 0121 12v6a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 18v-6c0-.98.626-1.813 1.5-2.122" />
                                            </svg>
                                        </div>
                                        <div class="text-xl sm:text-2xl font-bold text-base-content">
                                            {workloadStats().running}
                                            <span class="text-sm font-normal text-muted ml-1">running</span>
                                        </div>
                                        <Show when={workloadStats().total > 0} fallback={
                                            <div class="text-[10px] text-muted mt-1">No workloads detected</div>
                                        }>
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
                                                <div class="text-[10px] text-muted">
                                                    {workloadStats().stopped} stopped
                                                </div>
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
                                        <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">Network</span>
                                        <Show when={focusedHostName()}>
                                            <span class="text-xs text-muted ml-1.5 truncate">&mdash; {focusedHostName()}</span>
                                        </Show>
                                    </div>
                                    <Show
                                        when={hasNetData()}
                                        fallback={
                                            isCurrentRangeLoaded() ? (
                                                <div class="text-sm text-muted py-2">
                                                    No history yet
                                                </div>
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
