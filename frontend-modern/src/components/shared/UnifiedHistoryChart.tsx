import { Component, createSignal, createEffect, onCleanup, onMount, Show, For, createMemo } from 'solid-js';
import { Portal } from 'solid-js/web';
import { AggregatedMetricPoint, ChartsAPI, HistoryTimeRange, ResourceType } from '@/api/charts';
import { formatBytes } from '@/utils/format';
import { hasFeature, loadLicenseStatus } from '@/stores/license';
import { logger } from '@/utils/logger';

interface UnifiedHistoryChartProps {
    resourceType: ResourceType;
    resourceId: string;
    height?: number;
    label?: string;
    range?: HistoryTimeRange;
    onRangeChange?: (range: HistoryTimeRange) => void;
    hideSelector?: boolean;
}

interface HoverInfo {
    timestamp: number;
    x: number;
    y: number;
    metrics: Record<string, { value: number; min: number; max: number; color: string; label: string; unit: string }>;
}

export const UnifiedHistoryChart: Component<UnifiedHistoryChartProps> = (props) => {
    let canvasRef: HTMLCanvasElement | undefined;
    let containerRef: HTMLDivElement | undefined;

    const [range, setRange] = createSignal<HistoryTimeRange>(props.range || '24h');
    const [metricsData, setMetricsData] = createSignal<Record<string, AggregatedMetricPoint[]>>({});
    const [loading, setLoading] = createSignal(false);
    const [error, setError] = createSignal<string | null>(null);
    const [hoveredPoint, setHoveredPoint] = createSignal<HoverInfo | null>(null);
    const [source, setSource] = createSignal<'store' | 'memory' | 'live' | null>(null);
    const [maxPoints, setMaxPoints] = createSignal<number | null>(null);
    const [group, setGroup] = createSignal<'utilization' | 'io'>('utilization');
    const [refreshTick, setRefreshTick] = createSignal(0);

    const metricGroups: Record<'utilization' | 'io', Record<string, { label: string; color: string; unit: string }>> = {
        utilization: {
            cpu: { label: 'CPU', color: '#8b5cf6', unit: '%' },      // violet-500
            memory: { label: 'Memory', color: '#f59e0b', unit: '%' }, // amber-500
            disk: { label: 'Disk', color: '#10b981', unit: '%' }      // emerald-500
        },
        io: {
            diskread: { label: 'Disk Read', color: '#3b82f6', unit: 'B/s' },  // blue-500
            diskwrite: { label: 'Disk Write', color: '#6366f1', unit: 'B/s' }, // indigo-500
            netin: { label: 'Net In', color: '#10b981', unit: 'B/s' },         // emerald-500
            netout: { label: 'Net Out', color: '#f59e0b', unit: 'B/s' }        // amber-500
        }
    };

    const isLocked = createMemo(() => (range() === '30d' || range() === '90d') && !hasFeature('long_term_metrics'));
    const lockDays = createMemo(() => (range() === '30d' ? '30' : '90'));
    const refreshIntervalMs = createMemo(() => {
        const r = range();
        switch (r) {
            case '7d':
                return 30000;
            case '30d':
                return 60000;
            case '90d':
                return 120000;
            default:
                return 10000;
        }
    });

    const loadData = async (resourceType: ResourceType, resourceId: string, rangeValue: HistoryTimeRange, pointsCap?: number | null) => {
        setLoading(true);
        setError(null);
        setSource(null);
        try {
            // Fetch all metrics for the resource
            const response = await ChartsAPI.getMetricsHistory({
                resourceType,
                resourceId,
                range: rangeValue,
                maxPoints: pointsCap ?? undefined
            });

            if ('metrics' in response) {
                setMetricsData(response.metrics);
                setSource(response.source ?? 'store');
            } else {
                // Should not happen with multi-metric query, but handle fallback
                setMetricsData({ [response.metric]: response.points });
                setSource(response.source ?? 'store');
            }
        } catch (err: any) {
            console.error('[UnifiedHistoryChart] Failed to load history:', err);
            setError('Failed to load history data');
            setSource(null);
        } finally {
            setLoading(false);
        }
    };

    onMount(() => {
        loadLicenseStatus();
    });

    createEffect(() => {
        if (props.range) setRange(props.range);
    });

    createEffect(() => {
        const resourceType = props.resourceType;
        const resourceId = props.resourceId;
        const rangeValue = props.range ?? range();
        const locked = isLocked();
        const pointsCap = maxPoints();
        refreshTick();

        if (!resourceType || !resourceId) return;
        if (locked) {
            setLoading(false);
            setError(null);
            return;
        }
        loadData(resourceType, resourceId, rangeValue, pointsCap);
    });

    createEffect(() => {
        const interval = refreshIntervalMs();
        if (!interval || interval <= 0) return;
        const timer = window.setInterval(() => {
            setRefreshTick((t) => t + 1);
        }, interval);
        onCleanup(() => window.clearInterval(timer));
    });

    const drawChart = () => {
        if (!canvasRef) return;
        const ctx = canvasRef.getContext('2d');
        if (!ctx) return;

        const w = canvasRef.parentElement?.clientWidth || 300;
        const h = props.height || 200;
        const activeGroup = group();
        const metricConfigs = metricGroups[activeGroup];

        const dpr = window.devicePixelRatio || 1;
        canvasRef.width = w * dpr;
        canvasRef.height = h * dpr;
        canvasRef.style.width = `${w}px`;
        canvasRef.style.height = `${h}px`;
        ctx.scale(dpr, dpr);

        ctx.clearRect(0, 0, w, h);

        const isDark = document.documentElement.classList.contains('dark');
        const gridColor = isDark ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.05)';
        const textColor = isDark ? '#9ca3af' : '#6b7280';
        const axisTextColor = isDark ? '#9ca3af' : '#6b7280';

        // Draw grid
        ctx.strokeStyle = gridColor;
        ctx.lineWidth = 1;
        [0, 0.5, 1].forEach(pct => {
            const y = h - 20 - (pct * (h - 40));
            ctx.beginPath();
            ctx.moveTo(40, y);
            ctx.lineTo(w, y);
            ctx.stroke();

            ctx.fillStyle = textColor;
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'right';
            ctx.textBaseline = 'middle';
            if (activeGroup === 'utilization') {
                ctx.fillText(`${Math.round(pct * 100)}%`, 35, y);
            }
        });

        // Plot each series
        const dataMap = metricsData();
        const pointsForAxis = Object.keys(metricConfigs)
            .map(metricId => dataMap[metricId])
            .find(points => points && points.length > 0);
        let axisStart = 0;
        let axisEnd = 0;
        if (pointsForAxis && pointsForAxis.length > 0) {
            axisStart = pointsForAxis[0].timestamp;
            axisEnd = pointsForAxis[pointsForAxis.length - 1].timestamp;
        }

        let maxAxisValue = 100;
        if (activeGroup === 'io') {
            let maxValue = 0;
            Object.keys(metricConfigs).forEach(metricId => {
                const points = dataMap[metricId];
                if (!points || points.length === 0) return;
                for (const p of points) {
                    const v = p.max || p.value || 0;
                    if (v > maxValue) maxValue = v;
                }
            });
            maxAxisValue = Math.max(1, maxValue * 1.1);
        }

        Object.entries(metricConfigs).forEach(([metricId, config]) => {
            const points = dataMap[metricId];
            if (!points || points.length === 0) {
                logger.debug(`[UnifiedHistoryChart] No points for ${metricId}`);
                return;
            }

            const startTime = points[0].timestamp;
            const endTime = points[points.length - 1].timestamp;
            const timeSpan = endTime - startTime || 1;

            const getX = (ts: number) => 40 + ((ts - startTime) / timeSpan) * (w - 40);
            const getY = (val: number) => {
                const clamped = Math.max(0, Math.min(val, maxAxisValue));
                return h - 20 - (clamped / maxAxisValue) * (h - 40);
            };

            // Draw Area (Transparent)
            ctx.fillStyle = `${config.color}15`; // 15 order opacity
            ctx.beginPath();
            ctx.moveTo(getX(points[0].timestamp), h - 20);
            points.forEach(p => ctx.lineTo(getX(p.timestamp), getY(p.value)));
            ctx.lineTo(getX(points[points.length - 1].timestamp), h - 20);
            ctx.fill();

            // Draw Line
            ctx.strokeStyle = config.color;
            ctx.lineWidth = 2;
            ctx.lineJoin = 'round';
            ctx.beginPath();
            points.forEach((p, i) => {
                if (i === 0) ctx.moveTo(getX(p.timestamp), getY(p.value));
                else ctx.lineTo(getX(p.timestamp), getY(p.value));
            });
            ctx.stroke();
        });

        if (axisStart > 0 && axisEnd > axisStart) {
            const timeSpan = axisEnd - axisStart;
            const getAxisX = (ts: number) => 40 + ((ts - axisStart) / timeSpan) * (w - 40);
            const formatTimeLabel = (ts: number) => {
                const date = new Date(ts);
                const r = range();
                if (r === '30d' || r === '90d' || r === '7d') {
                    return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
                }
                return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            };

            ctx.fillStyle = axisTextColor;
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'bottom';

            const labelCount = 4;
            for (let i = 0; i < labelCount; i++) {
                const t = axisStart + (timeSpan * i) / (labelCount - 1);
                const x = getAxisX(t);
                ctx.fillText(formatTimeLabel(t), x, h - 2);
            }
        }

        if (activeGroup === 'io') {
            ctx.fillStyle = textColor;
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'right';
            ctx.textBaseline = 'middle';
            [0, 0.5, 1].forEach(pct => {
                const y = h - 20 - (pct * (h - 40));
                const value = maxAxisValue * pct;
                ctx.fillText(`${formatBytes(value)}/s`, 35, y);
            });
        }
    };

    createEffect(() => {
        metricsData();
        drawChart();
    });

    createEffect(() => {
        if (!containerRef) return;
        const computeMaxPoints = (width: number) => {
            const safeWidth = Math.max(120, Math.floor(width));
            const dpr = window.devicePixelRatio || 1;
            const points = Math.round(safeWidth * dpr);
            return Math.min(1200, Math.max(180, points));
        };

        const updateMaxPoints = () => {
            const width = containerRef?.clientWidth || 0;
            if (width <= 0) return;
            const next = computeMaxPoints(width);
            if (next !== maxPoints()) {
                setMaxPoints(next);
            }
        };

        const ro = new ResizeObserver(() => {
            updateMaxPoints();
            drawChart();
        });
        ro.observe(containerRef);
        updateMaxPoints();
        onCleanup(() => ro.disconnect());
    });

    const handleMouseMove = (e: MouseEvent) => {
        if (!canvasRef) return;
        const rect = canvasRef.getBoundingClientRect();
        const x = e.clientX - rect.left;
        if (x < 40) {
            setHoveredPoint(null);
            return;
        }

        const dataMap = metricsData();
        const activeGroup = group();
        const metricConfigs = metricGroups[activeGroup];
        const firstMetric = Object.keys(metricConfigs).find(metricId => dataMap[metricId]?.length);
        const points = firstMetric ? dataMap[firstMetric] : undefined;
        if (!points || points.length === 0) return;

        const startTime = points[0].timestamp;
        const endTime = points[points.length - 1].timestamp;
        const ratio = (x - 40) / (rect.width - 40);
        const hoverTs = startTime + ratio * (endTime - startTime);

        const hoverInfo: HoverInfo = {
            timestamp: 0,
            x: e.clientX,
            y: rect.top + 20,
            metrics: {}
        };

        let pickedTs = 0;

        Object.entries(metricConfigs).forEach(([id, config]) => {
            const pArr = dataMap[id];
            if (!pArr || pArr.length === 0) return;

            let closest = pArr[0];
            let minDiff = Math.abs(pArr[0].timestamp - hoverTs);
            for (const p of pArr) {
                const diff = Math.abs(p.timestamp - hoverTs);
                if (diff < minDiff) {
                    minDiff = diff;
                    closest = p;
                }
            }
            pickedTs = closest.timestamp;
            hoverInfo.metrics[id] = {
                value: closest.value,
                min: closest.min || closest.value,
                max: closest.max || closest.value,
                color: config.color,
                label: config.label,
                unit: config.unit
            };
        });

        hoverInfo.timestamp = pickedTs;
        setHoveredPoint(hoverInfo);
    };

    const updateRange = (r: HistoryTimeRange) => {
        setRange(r);
        if (props.onRangeChange) props.onRangeChange(r);
    };

    return (
        <div class="flex flex-col bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4">
            <div class="flex items-center justify-between mb-4">
                <div class="flex items-center gap-3">
                    <span class="text-sm font-bold text-gray-700 dark:text-gray-200">{props.label || 'Unified History'}</span>
                    <Show when={source() && source() !== 'store'}>
                        <span
                            class={`text-[10px] font-semibold px-2 py-0.5 rounded-full uppercase tracking-wide ${
                                source() === 'live'
                                    ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                                    : 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                            }`}
                            title={source() === 'live' ? 'Live sample shown because history is not available yet.' : 'In-memory buffer shown while history is warming up.'}
                        >
                            {source() === 'live' ? 'Live' : 'Memory'}
                        </span>
                    </Show>
                    <div class="flex items-center gap-2">
                        <For each={Object.values(metricGroups[group()])}>
                            {(c) => (
                                <div class="flex items-center gap-1">
                                    <div class="w-2 h-2 rounded-full" style={{ 'background-color': c.color }} />
                                    <span class="text-[10px] text-gray-500">{c.label}</span>
                                </div>
                            )}
                        </For>
                    </div>
                </div>

                <div class="flex items-center gap-2">
                    <div class="flex bg-gray-100 dark:bg-gray-700 rounded-lg p-0.5">
                        {(['utilization', 'io'] as const).map(mode => (
                            <button
                                onClick={() => setGroup(mode)}
                                class={`px-2.5 py-1 text-[10px] font-semibold rounded-md transition-colors ${group() === mode
                                    ? 'bg-white dark:bg-gray-600 text-blue-600 dark:text-blue-400 shadow-sm'
                                    : 'text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                                    }`}
                                title={mode === 'utilization' ? 'CPU / Memory / Disk %' : 'Disk / Network throughput'}
                            >
                                {mode === 'utilization' ? 'Utilization' : 'IO'}
                            </button>
                        ))}
                    </div>
                    <Show when={!props.hideSelector}>
                        <div class="flex bg-gray-100 dark:bg-gray-700 rounded-lg p-0.5">
                            {(['24h', '7d', '30d', '90d'] as HistoryTimeRange[]).map(r => (
                                <button
                                    onClick={() => updateRange(r)}
                                    class={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${range() === r
                                        ? 'bg-white dark:bg-gray-600 text-blue-600 dark:text-blue-400 shadow-sm'
                                        : 'text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                                        }`}
                                >
                                    {r}
                                </button>
                            ))}
                        </div>
                    </Show>
                </div>
            </div>

            <div class="relative flex-1 min-h-[220px] w-full" ref={containerRef}>
                <canvas
                    ref={canvasRef}
                    class="block w-full h-full cursor-crosshair"
                    onMouseMove={handleMouseMove}
                    onMouseLeave={() => setHoveredPoint(null)}
                />

                <Show when={loading()}>
                    <div class="absolute inset-0 flex items-center justify-center bg-white/50 dark:bg-gray-800/50 backdrop-blur-sm">
                        <div class="w-8 h-8 border-3 border-blue-500 border-t-transparent rounded-full animate-spin" />
                    </div>
                </Show>

                <Show when={error()}>
                    <div class="absolute inset-0 flex items-center justify-center">
                        <p class="text-sm text-red-500 bg-red-50 dark:bg-red-900/20 px-3 py-1 rounded-full border border-red-100 dark:border-red-800">{error()}</p>
                    </div>
                </Show>

                <Show when={isLocked()}>
                    <div class="absolute inset-0 z-10 flex flex-col items-center justify-center backdrop-blur-md bg-white/40 dark:bg-gray-900/40 rounded-lg">
                        <div class="bg-indigo-600 rounded-full p-2.5 shadow-xl mb-3">
                            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                                <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
                                <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
                            </svg>
                        </div>
                        <h3 class="text-sm font-bold text-gray-900 dark:text-white mb-1 text-center">{lockDays()}-Day History Locked</h3>
                        <p class="text-[10px] text-gray-600 dark:text-gray-300 mb-3 text-center">
                            Upgrade to Pulse Pro to unlock {lockDays()} days of history.
                        </p>
                        <a href="https://pulserelay.pro/pricing" target="_blank" class="px-4 py-1.5 bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-bold rounded-full shadow-lg transition-all transform hover:scale-105">Upgrade to Pro</a>
                    </div>
                </Show>
            </div>

            <Portal>
                <Show when={hoveredPoint()}>
                    {(info) => (
                        <div
                            class="fixed pointer-events-none bg-gray-900/95 dark:bg-gray-800/95 text-white text-xs rounded-lg px-3 py-2 shadow-2xl border border-gray-700/50 z-[9999] backdrop-blur-md"
                            style={{
                                left: `${info().x}px`,
                                top: `${info().y}px`,
                                transform: 'translateX(-50%)'
                            }}
                        >
                            <div class="font-bold text-center border-b border-gray-700 pb-1.5 mb-1.5 opacity-80">{new Date(info().timestamp).toLocaleString()}</div>
                            <div class="space-y-1.5">
                                <For each={Object.entries(info().metrics)}>
                                    {([_, m]) => (
                                        <div class="flex items-center justify-between gap-6">
                                            <div class="flex items-center gap-1.5">
                                                <div class="w-1.5 h-1.5 rounded-full" style={{ 'background-color': m.color }} />
                                                <span class="opacity-70">{m.label}</span>
                                            </div>
                                            <span class="font-mono font-bold" style={{ color: m.color }}>
                                                {m.unit === '%' ? `${m.value.toFixed(1)}%` : `${formatBytes(m.value)}/s`}
                                            </span>
                                        </div>
                                    )}
                                </For>
                            </div>
                        </div>
                    )}
                </Show>
            </Portal>
        </div>
    );
};
