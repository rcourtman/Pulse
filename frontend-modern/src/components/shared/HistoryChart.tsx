/**
 * HistoryChart Component
 *
 * Canvas-based chart for displaying historical metrics data (up to 90 days).
 * Includes user-friendly empty states and Pro-tier gating for 30d/90d data.
 */

import { Component, createEffect, createSignal, onCleanup, Show, createMemo, onMount } from 'solid-js';
import { ChartsAPI, type ResourceType, type HistoryTimeRange, type AggregatedMetricPoint } from '@/api/charts';
import { hasFeature, loadLicenseStatus } from '@/stores/license';
import { Portal } from 'solid-js/web';
import { formatBytes } from '@/utils/format';

interface HistoryChartProps {
    resourceType: ResourceType;
    resourceId: string;
    metric: 'cpu' | 'memory' | 'disk';
    height?: number;
    color?: string;
    label?: string;
    unit?: string;
    range?: HistoryTimeRange;
    onRangeChange?: (range: HistoryTimeRange) => void;
    hideSelector?: boolean;
}

export const HistoryChart: Component<HistoryChartProps> = (props) => {
    let canvasRef: HTMLCanvasElement | undefined;
    let containerRef: HTMLDivElement | undefined;

    const [range, setRange] = createSignal<HistoryTimeRange>(props.range || '24h');
    const [data, setData] = createSignal<AggregatedMetricPoint[]>([]);
    const [loading, setLoading] = createSignal(false);
    const [error, setError] = createSignal<string | null>(null);

    // Load license status on mount to ensure hasFeature works correctly
    onMount(() => {
        loadLicenseStatus();
    });

    // Sync internal range with props.range
    createEffect(() => {
        if (props.range) {
            setRange(props.range);
        }
    });

    // Handle range change
    const updateRange = (newRange: HistoryTimeRange) => {
        setRange(newRange);
        if (props.onRangeChange) {
            props.onRangeChange(newRange);
        }
    };

    // Feature gating check
    const isLongTermEnabled = () => hasFeature('long_term_metrics');

    // Check if current view is locked
    const isLocked = createMemo(() => {
        const r = range();
        // Lock if range > 7d and feature not enabled (7d is free, 30d/90d require Pro)
        return !isLongTermEnabled() && (r === '30d' || r === '90d');
    });

    const lockDays = createMemo(() => (range() === '30d' ? '30' : '90'));

    // Hover state for tooltip
    const [hoveredPoint, setHoveredPoint] = createSignal<{
        value: number;
        min: number;
        max: number;
        timestamp: number;
        x: number;
        y: number;
    } | null>(null);

    // Fetch data when range or resource changes
    createEffect(async () => {
        const r = range();
        const type = props.resourceType;
        const id = props.resourceId;
        const metric = props.metric;
        const locked = isLocked();

        if (!id || !type) return;

        if (locked) {
            setLoading(false);
            setError(null);
            return;
        }

        setLoading(true);
        setError(null);
        try {
            const result = await ChartsAPI.getMetricsHistory({
                resourceType: type,
                resourceId: id,
                metric: metric,
                range: r
            });

            if ('points' in result) {
                setData(result.points || []);
            } else {
                // Should not happen as we request single metric
                setData([]);
            }
        } catch (err) {
            console.error('Failed to fetch metrics history:', err);
            setError('Failed to load history data');
        } finally {
            setLoading(false);
        }
    });

    // Draw chart
    const drawChart = () => {
        if (!canvasRef) return;
        const canvas = canvasRef;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;

        const points = data();
        const w = canvas.parentElement?.clientWidth || 300;
        const h = props.height || 200;

        // Handle device pixel ratio
        const dpr = window.devicePixelRatio || 1;
        canvas.width = w * dpr;
        canvas.height = h * dpr;
        canvas.style.width = `${w}px`;
        canvas.style.height = `${h}px`;
        ctx.scale(dpr, dpr);

        // Clear
        ctx.clearRect(0, 0, w, h);

        // Colors
        const isDark = document.documentElement.classList.contains('dark');
        const gridColor = isDark ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.05)';
        const textColor = isDark ? '#9ca3af' : '#6b7280';

        // Dynamic color based on prop or default
        let mainColor = props.color || '#3b82f6'; // blue-500
        if (props.metric === 'cpu') mainColor = '#8b5cf6'; // violet-500
        if (props.metric === 'memory') mainColor = '#f59e0b'; // amber-500
        if (props.metric === 'disk') mainColor = '#10b981'; // emerald-500

        // Draw grid lines (horizontal)
        ctx.strokeStyle = gridColor;
        ctx.lineWidth = 1;

        // 0%, 50%, 100% lines
        [0, 0.5, 1].forEach(pct => {
            const y = h - 20 - (pct * (h - 40)); // padding
            ctx.beginPath();
            ctx.moveTo(40, y);
            ctx.lineTo(w, y);
            ctx.stroke();

            // Y-Axis labels
            ctx.fillStyle = textColor;
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'right';
            ctx.textBaseline = 'middle';
            let label = '';
            if (pct === 0) label = '0';
            else if (pct === 1) label = props.unit === '%' ? '100' : 'Max';
            else label = props.unit === '%' ? '50' : 'Avg';

            if (props.unit === '%') label += '%';
            ctx.fillText(label, 35, y);
        });

        // If no data or loading
        if (points.length === 0) {
            return; // Empty state handled in JSX
        }

        // Calculate Scale
        // X is time, Y is value
        const startTime = points[0].timestamp;
        const endTime = points[points.length - 1].timestamp;
        const timeSpan = Math.max(1, endTime - startTime);

        const maxValue = Math.max(100, ...points.map(p => p.max || p.value));
        const minValue = 0;

        // Plot
        const getX = (ts: number) => 40 + ((ts - startTime) / timeSpan) * (w - 40);
        const getY = (val: number) => (h - 20) - ((val - minValue) / (maxValue - minValue)) * (h - 40);

        // Fill area
        ctx.beginPath();
        points.forEach((p, i) => {
            if (i === 0) ctx.moveTo(getX(p.timestamp), h - 20);
            ctx.lineTo(getX(p.timestamp), getY(p.value));
        });
        if (points.length > 0) {
            ctx.lineTo(getX(points[points.length - 1].timestamp), h - 20);
        }
        ctx.closePath();

        const gradient = ctx.createLinearGradient(0, 0, 0, h);
        gradient.addColorStop(0, `${mainColor}66`); // 40%
        gradient.addColorStop(1, `${mainColor}11`); // 6%
        ctx.fillStyle = gradient;
        ctx.fill();

        // Stroke line
        ctx.beginPath();
        ctx.strokeStyle = mainColor;
        ctx.lineWidth = 2;
        points.forEach((p, i) => {
            if (i === 0) ctx.moveTo(getX(p.timestamp), getY(p.value));
            else ctx.lineTo(getX(p.timestamp), getY(p.value));
        });
        ctx.stroke();

        // Min/Max envelope (optional, for pro feel?)
        // Let's keep it clean for now, maybe add later.
    };

    // Reactivity
    createEffect(() => {
        drawChart();
    });

    // Resize observer
    createEffect(() => {
        if (!containerRef) return;
        const resizeObserver = new ResizeObserver(() => drawChart());
        resizeObserver.observe(containerRef);
        onCleanup(() => resizeObserver.disconnect());
    });

    // Mouse interaction
    const handleMouseMove = (e: MouseEvent) => {
        if (!canvasRef || data().length === 0) return;
        const rect = canvasRef.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const points = data();

        const w = rect.width;
        // Map x to timestamp
        const startTime = points[0].timestamp;
        const endTime = points[points.length - 1].timestamp;
        const timeSpan = endTime - startTime;

        // Inverse getX: x = 40 + ratio * (w-40)
        // ratio = (x - 40) / (w - 40)
        if (x < 40) return;
        const ratio = (x - 40) / (w - 40);
        const hoverTs = startTime + ratio * timeSpan;

        // Find nearest point
        // Using simple binary search/scan is efficient enough for ~1000 points?
        // Find index with minimal timestamps diff
        let closest = points[0];
        let minDiff = Math.abs(points[0].timestamp - hoverTs);

        // Optimisation: direct index calculation if uniform, but it's not guaranteed.
        // Iterating is fast enough for < 10000 points.
        for (const p of points) {
            const diff = Math.abs(p.timestamp - hoverTs);
            if (diff < minDiff) {
                minDiff = diff;
                closest = p;
            }
        }

        setHoveredPoint({
            value: closest.value,
            min: closest.min || closest.value,
            max: closest.max || closest.value,
            timestamp: closest.timestamp,
            x: rect.left + x,
            y: rect.top + 20, // Approximate
        });
    };

    const handleMouseLeave = () => setHoveredPoint(null);

    const ranges: HistoryTimeRange[] = ['24h', '7d', '30d', '90d'];

    return (
        <div class="flex flex-col h-full bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4">
            <div class="flex items-center justify-between mb-4">
                <div class="flex items-center gap-2">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{props.label || 'History'}</span>
                    <Show when={props.unit}>
                        <span class="text-xs text-gray-400">({props.unit})</span>
                    </Show>
                </div>

                {/* Time Range Selector */}
                <Show when={!props.hideSelector}>
                    <div class="flex bg-gray-100 dark:bg-gray-700 rounded-lg p-0.5">
                        {ranges.map(r => (
                            <button
                                onClick={() => updateRange(r)}
                                class={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${range() === r
                                    ? 'bg-white dark:bg-gray-600 text-gray-900 dark:text-white shadow-sm'
                                    : 'text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                                    }`}
                            >
                                {r}
                            </button>
                        ))}
                    </div>
                </Show>
            </div>

            <div class="relative flex-1 min-h-[200px] w-full" ref={containerRef}>
                <canvas
                    ref={canvasRef}
                    class="block w-full h-full cursor-crosshair"
                    onMouseMove={handleMouseMove}
                    onMouseLeave={handleMouseLeave}
                />

                {/* Empty State */}
                <Show when={!loading() && data().length === 0 && !error()}>
                    <div class="absolute inset-0 flex items-center justify-center bg-white/50 dark:bg-gray-800/50">
                        <div class="text-center">
                            <div class="text-gray-400 mb-2">
                                <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="mx-auto">
                                    <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
                                    <path d="M3 3v5h5" />
                                    <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" />
                                    <path d="M16 16l5 5" />
                                    <path d="M21 21v-5h-5" />
                                </svg>
                            </div>
                            <p class="text-sm text-gray-500">Collecting data... History will appear here.</p>
                        </div>
                    </div>
                </Show>

                {/* Loading State */}
                <Show when={loading()}>
                    <div class="absolute inset-0 flex items-center justify-center bg-white/50 dark:bg-gray-800/50 backdrop-blur-[1px]">
                        <div class="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
                    </div>
                </Show>

                {/* Error State */}
                <Show when={error()}>
                    <div class="absolute inset-0 flex items-center justify-center">
                        <p class="text-sm text-red-500">{error()}</p>
                    </div>
                </Show>

                {/* Pro Lock Overlay */}
                <Show when={isLocked()}>
                    <div class="absolute inset-0 z-10 flex flex-col items-center justify-center backdrop-blur-sm bg-white/60 dark:bg-gray-900/60 rounded-lg">
                        <div class="bg-gradient-to-br from-indigo-500 to-purple-600 rounded-full p-3 shadow-lg mb-3">
                            <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
                                <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
                            </svg>
                        </div>
                        <h3 class="text-lg font-bold text-gray-900 dark:text-white mb-1">{lockDays()}-Day History</h3>
                        <p class="text-sm text-gray-600 dark:text-gray-300 text-center max-w-[200px] mb-4">
                            Upgrade to Pulse Pro to unlock {lockDays()} days of historical data retention.
                        </p>
                        <a
                            href="https://pulserelay.pro/pricing"
                            target="_blank"
                            class="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium rounded-md shadow-sm transition-colors"
                        >
                            Unlock Pro Features
                        </a>
                    </div>
                </Show>
            </div>

            <Portal>
                <Show when={hoveredPoint()}>
                    {(point) => (
                        <div
                            class="fixed pointer-events-none bg-gray-900 dark:bg-gray-800 text-white text-xs rounded px-2 py-1 shadow-lg border border-gray-700 z-[9999]"
                            style={{
                                left: `${point().x}px`,
                                top: `${point().y}px`,
                                transform: 'translateX(-50%)' // Center
                            }}
                        >
                            <div class="font-medium text-center mb-0.5">{new Date(point().timestamp).toLocaleString()}</div>
                            <div class="text-gray-300">
                                {props.unit === '%' ?
                                    `${point().value.toFixed(1)}%` :
                                    formatBytes(point().value)}
                            </div>
                            <Show when={point().min !== point().value}>
                                <div class="text-[10px] text-gray-400 mt-0.5">
                                    Min: {props.unit === '%' ? point().min.toFixed(1) : formatBytes(point().min)} •
                                    Max: {props.unit === '%' ? point().max.toFixed(1) : formatBytes(point().max)}
                                </div>
                            </Show>
                        </div>
                    )}
                </Show>
            </Portal>
        </div>
    );
};
