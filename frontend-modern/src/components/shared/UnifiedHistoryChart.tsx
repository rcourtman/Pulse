import { Component, createSignal, createEffect, onCleanup, onMount, Show, For } from 'solid-js';
import { Portal } from 'solid-js/web';
import { AggregatedMetricPoint, ChartsAPI, HistoryTimeRange, ResourceType } from '@/api/charts';
import { formatBytes } from '@/utils/format';
import { hasFeature, loadLicenseStatus } from '@/stores/license';

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

    const metricConfigs = {
        cpu: { label: 'CPU', color: '#8b5cf6', unit: '%' },      // violet-500
        memory: { label: 'Memory', color: '#f59e0b', unit: '%' }, // amber-500
        disk: { label: 'Disk', color: '#10b981', unit: '%' }      // emerald-500
    };

    const loadData = async (resourceType: ResourceType, resourceId: string, rangeValue: HistoryTimeRange) => {
        setLoading(true);
        setError(null);
        try {
            // Fetch all metrics for the resource
            const response = await ChartsAPI.getMetricsHistory({
                resourceType,
                resourceId,
                range: rangeValue
            });

            if ('metrics' in response) {
                setMetricsData(response.metrics);
            } else {
                // Should not happen with multi-metric query, but handle fallback
                setMetricsData({ [response.metric]: response.points });
            }
        } catch (err: any) {
            console.error('[UnifiedHistoryChart] Failed to load history:', err);
            setError('Failed to load history data');
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

        if (!resourceType || !resourceId) return;
        loadData(resourceType, resourceId, rangeValue);
    });

    const drawChart = () => {
        if (!canvasRef) return;
        const ctx = canvasRef.getContext('2d');
        if (!ctx) return;

        const w = canvasRef.parentElement?.clientWidth || 300;
        const h = props.height || 200;

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
            ctx.fillText(`${Math.round(pct * 100)}%`, 35, y);
        });

        // Plot each series
        const dataMap = metricsData();

        Object.entries(metricConfigs).forEach(([metricId, config]) => {
            const points = dataMap[metricId];
            if (!points || points.length === 0) {
                console.log(`[UnifiedHistoryChart] No points for ${metricId}`);
                return;
            }

            const startTime = points[0].timestamp;
            const endTime = points[points.length - 1].timestamp;
            const timeSpan = endTime - startTime || 1;

            const getX = (ts: number) => 40 + ((ts - startTime) / timeSpan) * (w - 40);
            const getY = (val: number) => h - 20 - (Math.min(Math.max(val, 0), 100) / 100) * (h - 40);

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
    };

    createEffect(() => {
        metricsData();
        drawChart();
    });

    createEffect(() => {
        if (!containerRef) return;
        const ro = new ResizeObserver(() => drawChart());
        ro.observe(containerRef);
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
        const firstMetric = Object.keys(dataMap)[0];
        const points = dataMap[firstMetric];
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

    const isLocked = () => (range() === '30d' || range() === '90d') && !hasFeature('long_term_metrics');

    return (
        <div class="flex flex-col bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4">
            <div class="flex items-center justify-between mb-4">
                <div class="flex items-center gap-3">
                    <span class="text-sm font-bold text-gray-700 dark:text-gray-200">{props.label || 'Unified History'}</span>
                    <div class="flex items-center gap-2">
                        <For each={Object.values(metricConfigs)}>
                            {(c) => (
                                <div class="flex items-center gap-1">
                                    <div class="w-2 h-2 rounded-full" style={{ 'background-color': c.color }} />
                                    <span class="text-[10px] text-gray-500">{c.label}</span>
                                </div>
                            )}
                        </For>
                    </div>
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
                        <h3 class="text-sm font-bold text-gray-900 dark:text-white mb-3 text-center">90-Day History Locked</h3>
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
                                                {m.unit === '%' ? `${m.value.toFixed(1)}%` : formatBytes(m.value)}
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
