import { Component, Show, For, createMemo, createEffect, createSignal, onCleanup } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { InteractiveSparkline } from '@/components/shared/InteractiveSparkline';
import type { Resource } from '@/types/resource';
import { getDisplayName } from '@/types/resource';
import type { MetricPoint, ChartData, TimeRange } from '@/api/charts';
import { useResources } from '@/hooks/useResources';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';
import {
    fetchInfrastructureSummaryAndCache,
    readInfrastructureSummaryCache,
} from '@/utils/infrastructureSummaryCache';
import { timeRangeToMs } from '@/utils/timeRange';

const HOST_COLORS = [
    '#3b82f6', // blue
    '#8b5cf6', // purple
    '#10b981', // emerald
    '#f97316', // orange
    '#ec4899', // pink
    '#06b6d4', // cyan
    '#f59e0b', // amber
    '#ef4444', // red
];

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
    const platformData = resource.platformData as { agent?: { agentId?: string } } | undefined;
    const agentId = platformData?.agent?.agentId;
    if (!agentId || typeof agentId !== 'string') return null;
    const trimmed = agentId.trim();
    return trimmed.length > 0 ? trimmed : null;
};

const formatDurationShort = (durationMs: number): string => {
    if (durationMs >= 24 * 60 * 60_000) {
        return `${Math.max(1, Math.round(durationMs / (24 * 60 * 60_000)))}d`;
    }
    if (durationMs >= 60 * 60_000) {
        return `${Math.max(1, Math.round(durationMs / (60 * 60_000)))}h`;
    }
    return `${Math.max(1, Math.round(durationMs / 60_000))}m`;
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
}

export const InfrastructureSummary: Component<InfrastructureSummaryProps> = (props) => {
    // Chart data keyed by resource identifier (node name, host id, etc.)
    const [chartMap, setChartMap] = createSignal<Map<string, ChartData>>(new Map());
    const [chartRange, setChartRange] = createSignal<TimeRange | null>(null);
    const [loadedRange, setLoadedRange] = createSignal<TimeRange | null>(null);
    const [isolatedHostKey, setIsolatedHostKey] = createSignal<string | null>(null);
    const [oldestDataTimestamp, setOldestDataTimestamp] = createSignal<number | null>(null);
    const [usingCachedData, setUsingCachedData] = createSignal(false);
    const selectedRange = createMemo<TimeRange>(() => props.timeRange || '1h');
    const hasCurrentRangeCharts = createMemo(() => chartRange() === selectedRange());
    const isCurrentRangeLoaded = createMemo(() => loadedRange() === selectedRange());

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
                // eslint-disable-next-line no-console
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
            // eslint-disable-next-line no-console
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
        setOldestDataTimestamp(cached.oldestDataTimestamp);
        setLoadedRange(range);
        setUsingCachedData(true);
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
            setOldestDataTimestamp(null);
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
        let appliedResponseMap = false;

        try {
            const fetched = await awaitAbortable(
                fetchInfrastructureSummaryAndCache(requestedRange, { caller: 'InfrastructureSummary' }),
                controller.signal,
            );
            if (requestId !== activeFetchRequest) {
                return;
            }
            requestSucceeded = true;
            const oldestTimestamp = fetched.oldestDataTimestamp;
            setOldestDataTimestamp(
                oldestTimestamp
            );
            const map = fetched.map;

            // If the backend returns an empty payload transiently, keep the last
            // good map to avoid flashing the "no history / static" fallbacks.
            const currentMapMatchesRequestedRange = chartRange() === requestedRange;
            if (map.size > 0 || chartMap().size === 0 || !currentMapMatchesRequestedRange) {
                appliedResponseMap = true;
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
                if (appliedResponseMap) {
                    setUsingCachedData(false);
                }
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
            setOldestDataTimestamp(null);
            setIsolatedHostKey(null);
            setUsingCachedData(false);
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
                setUsingCachedData(false);
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
    // The WebSocket state's hosts[] have internal IDs that match hostData chart keys,
    // and linkedNodeId/hostname fields that let us correlate with infrastructure resources.
    const findAgentChartData = (host: Resource): ChartData | undefined => {
        if (!hasCurrentRangeCharts()) return undefined;
        const map = chartMap();
        if (map.size === 0) return undefined;

        const agentId = getAgentIdFromResource(host);
        if (agentId) {
            const direct = map.get(agentId);
            if (direct) return direct;
        }

        const wsStore = getGlobalWebSocketStore();
        const wsHosts = wsStore.state.hosts;
        if (!wsHosts || wsHosts.length === 0) return undefined;

        const hostCandidates = new Set<string>([
            ...normalizeHostIdentifier(host.platformId),
            ...normalizeHostIdentifier(host.name),
            ...normalizeHostIdentifier(host.displayName),
            ...normalizeHostIdentifier(host.identity?.hostname),
        ]);

        // Find a WebSocket host agent that matches this infrastructure resource
        // by linked node ID, hostname, or name
        for (const wsHost of wsHosts) {
            // Match by linked node: agent is linked to a PVE node matching this resource
            const linkedMatch = wsHost.linkedNodeId && (
                host.id === wsHost.linkedNodeId ||
                host.name === wsHost.linkedNodeId ||
                host.platformId === wsHost.linkedNodeId
            );
            // Match by hostname: agent hostname matches this resource
            const wsHostNames = normalizeHostIdentifier(wsHost.hostname);
            const hostnameMatch = wsHostNames.some((candidate) => hostCandidates.has(candidate));

            if (linkedMatch || hostnameMatch) {
                const agentData = map.get(wsHost.id);
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

            const metricSeries = (metric: keyof ChartData): MetricPoint[] => {
                const primary = primaryData?.[metric];
                if (primary && primary.length > 0) return primary;
                const fallback = agentData?.[metric];
                if (fallback && fallback.length > 0) return fallback;
                return [];
            };

            return {
                key: host.id || host.platformId || host.name || `host-${i}`,
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

    createEffect(() => {
        const selected = isolatedHostKey();
        if (!selected) return;
        const exists = hostSeries().some((s) => s.key === selected);
        if (!exists) setIsolatedHostKey(null);
    });

    const visibleHostSeries = createMemo(() => {
        const selected = isolatedHostKey();
        const allHosts = hostSeries();
        if (!selected) return allHosts;
        const match = allHosts.find((s) => s.key === selected);
        return match ? [match] : allHosts;
    });

    const isolatedHostName = createMemo(() => {
        const selected = isolatedHostKey();
        if (!selected) return null;
        return hostSeries().find((s) => s.key === selected)?.name ?? null;
    });

    const toggleHostIsolation = (hostKey: string) => {
        setIsolatedHostKey((current) => (current === hostKey ? null : hostKey));
    };

    const hasData = (metric: 'cpu' | 'memory' | 'disk') =>
        visibleHostSeries().some((s) => s[metric].length >= 2);

    const networkSeries = createMemo(() =>
        visibleHostSeries().map((s) => ({
            data: s.network,
            color: s.color,
            name: s.name,
        }))
    );

    const hasNetData = () =>
        visibleHostSeries().some((s) => s.network.length >= 2);

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
    const selectedRangeMs = createMemo(() => timeRangeToMs(props.timeRange || '1h'));
    const availableRangeMs = createMemo(() => {
        const oldest = oldestDataTimestamp();
        if (!oldest) return 0;
        return Math.max(0, Date.now() - oldest);
    });
    const hasLimitedHistory = createMemo(() => {
        const requested = selectedRangeMs();
        const available = availableRangeMs();
        if (!isCurrentRangeLoaded() || requested <= 0 || available <= 0) return false;
        // Allow one sample interval of tolerance for timestamp jitter.
        return available + 30_000 < requested;
    });

    const seriesFor = (metric: 'cpu' | 'memory' | 'disk') =>
        visibleHostSeries().map((s) => ({ data: s[metric], color: s.color, name: s.name }));
    const networkLegendSeries = createMemo(() =>
        hostSeries().filter((s) => s.network.length >= 2)
    );
    const rangeLabel = () => props.timeRange || '1h';

    // Workload stats from WebSocket state (zero API calls)
    const { workloads } = useResources();

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

    // Mini host legend
    const HostLegend: Component<{ metric: 'cpu' | 'memory' | 'disk' }> = (legendProps) => {
        const visible = () => hostSeries().filter((s) => s[legendProps.metric].length >= 2);
        const selected = () => isolatedHostKey();
        return (
            <div class="flex flex-wrap gap-x-2 gap-y-0.5 mt-1">
                <For each={visible().slice(0, 6)}>
                    {(s) => (
                        <button
                            type="button"
                            class={`inline-flex items-center gap-1 text-[9px] transition-opacity ${
                                selected() === s.key
                                    ? 'text-gray-700 dark:text-gray-200 opacity-100'
                                    : selected()
                                        ? 'text-gray-500 dark:text-gray-400 opacity-45 hover:opacity-75'
                                        : 'text-gray-500 dark:text-gray-400 opacity-100 hover:opacity-75'
                            }`}
                            aria-pressed={selected() === s.key}
                            onClick={() => toggleHostIsolation(s.key)}
                            title={selected() === s.key ? `Showing only ${s.name}` : `Show only ${s.name}`}
                        >
                            <span class="w-1.5 h-1.5 rounded-full flex-shrink-0" style={{ background: s.color }} />
                            <span class="truncate max-w-[60px]">{s.name}</span>
                        </button>
                    )}
                </For>
                <Show when={visible().length > 6}>
                    <span class="text-[9px] text-gray-400 dark:text-gray-500">
                        +{visible().length - 6}
                    </span>
                </Show>
            </div>
        );
    };

    return (
        <Show when={props.hosts.length > 0}>
            <div class="space-y-2">
                <Show when={usingCachedData()}>
                    <div class="text-[11px] text-sky-700 dark:text-sky-300">
                        Showing cached trend data while live history updates.
                    </div>
                </Show>
                <Show when={hasLimitedHistory()}>
                    <div class="text-[11px] text-amber-700 dark:text-amber-300">
                        Showing {formatDurationShort(availableRangeMs())} of {formatDurationShort(selectedRangeMs())} history; longer-range data is still building.
                    </div>
                </Show>
                <Show when={isolatedHostName()}>
                    {(name) => (
                        <div class="text-[11px] text-blue-700 dark:text-blue-300">
                            Showing host: {name()} (click the legend again to clear)
                        </div>
                    )}
                </Show>
                <div class="grid gap-3 sm:gap-4 grid-cols-2 lg:grid-cols-4">
                {/* CPU Card */}
                <Card padding="sm" tone="glass">
                    <div class="flex flex-col h-full">
                        <div class="flex items-center justify-between mb-1.5">
                            <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">CPU</span>
                            <svg class="w-4 h-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 3v1.5M4.5 8.25H3m18 0h-1.5M4.5 12H3m18 0h-1.5m-15 3.75H3m18 0h-1.5M8.25 19.5V21M12 3v1.5m0 15V21m3.75-18v1.5m0 15V21m-9-1.5h10.5a2.25 2.25 0 002.25-2.25V6.75a2.25 2.25 0 00-2.25-2.25H6.75A2.25 2.25 0 004.5 6.75v10.5a2.25 2.25 0 002.25 2.25zm.75-12h9v9h-9v-9z" />
                            </svg>
                        </div>
                        <Show
                            when={hasData('cpu')}
                            fallback={
                                <div class="text-sm text-gray-400 dark:text-gray-500 py-2">
                                    {isCurrentRangeLoaded() ? 'No history yet' : 'Loading history...'}
                                </div>
                            }
                        >
                            <div class="flex-1 min-h-0">
                                <InteractiveSparkline
                                    series={seriesFor('cpu')}
                                    rangeLabel={rangeLabel()}
                                    timeRange={props.timeRange}
                                    yMode="percent"
                                />
                            </div>
                            <HostLegend metric="cpu" />
                        </Show>
                    </div>
                </Card>

                {/* Memory Card */}
                <Card padding="sm" tone="glass">
                    <div class="flex flex-col h-full">
                        <div class="flex items-center justify-between mb-1.5">
                            <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Memory</span>
                            <svg class="w-4 h-4 text-purple-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M6 3h12a2 2 0 012 2v14a2 2 0 01-2 2H6a2 2 0 01-2-2V5a2 2 0 012-2zm0 0v18m12-18v18M8 7h2m4 0h2M8 11h2m4 0h2M8 15h2m4 0h2" />
                            </svg>
                        </div>
                        <Show
                            when={hasData('memory')}
                            fallback={
                                <div class="text-sm text-gray-400 dark:text-gray-500 py-2">
                                    {isCurrentRangeLoaded() ? 'No history yet' : 'Loading history...'}
                                </div>
                            }
                        >
                            <div class="flex-1 min-h-0">
                                <InteractiveSparkline
                                    series={seriesFor('memory')}
                                    rangeLabel={rangeLabel()}
                                    timeRange={props.timeRange}
                                    yMode="percent"
                                />
                            </div>
                            <HostLegend metric="memory" />
                        </Show>
                    </div>
                </Card>

                {/* Storage Card */}
                <Card padding="sm" tone="glass">
                    <div class="flex flex-col h-full">
                        <div class="flex items-center justify-between mb-1.5">
                            <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Storage</span>
                            <svg class="w-4 h-4 text-cyan-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125" />
                            </svg>
                        </div>
                        <Show
                            when={hasData('disk')}
                            fallback={
                                <div class="text-sm text-gray-400 dark:text-gray-500 py-2">
                                    {isCurrentRangeLoaded() ? 'No history yet' : 'Loading history...'}
                                </div>
                            }
                        >
                            <div class="flex-1 min-h-0">
                                <InteractiveSparkline
                                    series={seriesFor('disk')}
                                    rangeLabel={rangeLabel()}
                                    timeRange={props.timeRange}
                                    yMode="percent"
                                />
                            </div>
                            <HostLegend metric="disk" />
                        </Show>
                    </div>
                </Card>

                <Show
                    when={shouldShowNetworkCard()}
                    fallback={
                        <Card padding="sm" tone="glass">
                            <div class="flex flex-col h-full">
                                <div class="flex items-center justify-between mb-1.5">
                                    <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Workloads</span>
                                    <svg class="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                        <path stroke-linecap="round" stroke-linejoin="round" d="M6 6.878V6a2.25 2.25 0 012.25-2.25h7.5A2.25 2.25 0 0118 6v.878m-12 0c.235-.083.487-.128.75-.128h10.5c.263 0 .515.045.75.128m-12 0A2.25 2.25 0 004.5 9v.878m13.5-3A2.25 2.25 0 0119.5 9v.878m0 0a2.246 2.246 0 00-.75-.128H5.25c-.263 0-.515.045-.75.128m15 0A2.25 2.25 0 0121 12v6a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 18v-6c0-.98.626-1.813 1.5-2.122" />
                                    </svg>
                                </div>
                                <div class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-gray-100">
                                    {workloadStats().running}
                                    <span class="text-sm font-normal text-gray-500 dark:text-gray-400 ml-1">running</span>
                                </div>
                                <Show when={workloadStats().total > 0} fallback={
                                    <div class="text-[10px] text-gray-400 dark:text-gray-500 mt-1">No workloads detected</div>
                                }>
                                    <div class="text-[10px] text-gray-500 dark:text-gray-400 mt-1">
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
                                        <div class="text-[10px] text-gray-400 dark:text-gray-500">
                                            {workloadStats().stopped} stopped
                                        </div>
                                    </Show>
                                </Show>
                            </div>
                        </Card>
                    }
                >
                    {/* 4th Card: Network */}
                    <Card padding="sm" tone="glass">
                        <div class="flex flex-col h-full">
                            <div class="flex items-center justify-between mb-1.5">
                                <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Network</span>
                                <svg class="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
                                </svg>
                            </div>
                            <Show
                                when={hasNetData()}
                                fallback={
                                    <div class="text-sm text-gray-400 dark:text-gray-500 py-2">
                                        {isCurrentRangeLoaded() ? 'No history yet' : 'Loading history...'}
                                    </div>
                                }
                            >
                                <div class="flex-1 min-h-0">
                                    <InteractiveSparkline
                                        series={networkSeries()}
                                        rangeLabel={rangeLabel()}
                                        timeRange={props.timeRange}
                                        yMode="auto"
                                        formatValue={formatRate}
                                        formatTopLabel={formatRate}
                                        sortTooltipByValue
                                    />
                                </div>
                                <div class="flex flex-wrap gap-x-2 gap-y-0.5 mt-1">
                                    <For each={networkLegendSeries().slice(0, 6)}>
                                        {(s) => (
                                            <button
                                                type="button"
                                                class={`inline-flex items-center gap-1 text-[9px] transition-opacity ${
                                                    isolatedHostKey() === s.key
                                                        ? 'text-gray-700 dark:text-gray-200 opacity-100'
                                                        : isolatedHostKey()
                                                            ? 'text-gray-500 dark:text-gray-400 opacity-45 hover:opacity-75'
                                                            : 'text-gray-500 dark:text-gray-400 opacity-100 hover:opacity-75'
                                                }`}
                                                aria-pressed={isolatedHostKey() === s.key}
                                                onClick={() => toggleHostIsolation(s.key)}
                                                title={isolatedHostKey() === s.key ? `Showing only ${s.name}` : `Show only ${s.name}`}
                                            >
                                                <span class="w-1.5 h-1.5 rounded-full flex-shrink-0" style={{ background: s.color }} />
                                                <span class="truncate max-w-[60px]">{s.name}</span>
                                            </button>
                                        )}
                                    </For>
                                    <Show when={networkLegendSeries().length > 6}>
                                        <span class="text-[9px] text-gray-400 dark:text-gray-500">
                                            +{networkLegendSeries().length - 6}
                                        </span>
                                    </Show>
                                </div>
                            </Show>
                        </div>
                    </Card>
                </Show>
            </div>
            </div>
        </Show>
    );
};

export default InfrastructureSummary;
