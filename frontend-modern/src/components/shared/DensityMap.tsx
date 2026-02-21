import { Component, createMemo, createEffect, createSignal, onCleanup, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { TimeRange } from '@/api/charts';
import { timeRangeToMs } from '@/utils/timeRange';
import type { InteractiveSparklineSeries } from './InteractiveSparkline';

interface DensityMapProps {
    series: InteractiveSparklineSeries[];
    rangeLabel?: string;
    timeRange?: TimeRange;
    formatValue?: (value: number) => string;
}

const formatHoverTime = (timestamp: number): string =>
    new Date(timestamp).toLocaleString([], {
        month: 'short',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
    });

const clamp = (value: number, min: number, max: number): number =>
    Math.max(min, Math.min(value, max));

export const DensityMap: Component<DensityMapProps> = (props) => {
    let canvasRef: HTMLCanvasElement | undefined;
    let chartSurfaceRef: HTMLDivElement | undefined;
    const COLUMNS = 45; // Fixed number of time buckets
    const PADDING_Y = 2; // Padding between rows
    const PADDING_X = 2; // Padding between columns

    const [hoveredState, setHoveredState] = createSignal<{
        tooltipX: number;
        tooltipY: number;
        timestamp: number;
        value: number;
        seriesName: string;
        seriesColor: string;
    } | null>(null);

    const formatValue = (v: number) => (props.formatValue ? props.formatValue(v) : `${v.toFixed(1)}`);

    // Pre-process data
    const chartData = createMemo(() => {
        const range = props.timeRange || '1h';
        const rangeMs = timeRangeToMs(range);
        const windowEnd = Date.now();
        const windowStart = windowEnd - rangeMs;
        const bucketDuration = rangeMs / COLUMNS;

        // Filter series that actually have data
        const activeSeries = props.series.filter(s => s.data.length > 0);

        // Sort series to put the most active ones at the top
        const seriesWithVolume = activeSeries.map(series => {
            let total = 0;
            for (const p of series.data) {
                if (p.timestamp >= windowStart) total += p.value;
            }
            return { series, total };
        });
        seriesWithVolume.sort((a, b) => b.total - a.total);

        // Take top 20 to avoid completely squished rows
        const topSeries = seriesWithVolume.slice(0, 20).map(s => s.series);

        let globalMax = 0;
        const grid: { sum: number; count: number; max: number }[][] = topSeries.map(() =>
            Array(COLUMNS).fill(null).map(() => ({ sum: 0, count: 0, max: 0 }))
        );

        // Populate grid
        for (let r = 0; r < topSeries.length; r++) {
            for (const p of topSeries[r].data) {
                if (p.timestamp < windowStart || p.timestamp > windowEnd) continue;
                const col = Math.floor(((p.timestamp - windowStart) / rangeMs) * COLUMNS);
                const c = clamp(col, 0, COLUMNS - 1);

                grid[r][c].sum += p.value;
                grid[r][c].count += 1;
                if (p.value > grid[r][c].max) {
                    grid[r][c].max = p.value;
                }
                if (p.value > globalMax) {
                    globalMax = p.value;
                }
            }
        }

        const cellData = topSeries.map((_, r) => {
            return grid[r].map(cell => {
                const val = cell.count > 0 ? cell.max : 0; // Or average: sum / count
                return val;
            });
        });

        return {
            series: topSeries,
            grid: cellData,
            globalMax,
            windowStart,
            rangeMs,
            bucketDuration
        };
    });

    const drawCanvas = () => {
        if (!canvasRef) return;
        const ctx = canvasRef.getContext('2d');
        if (!ctx) return;

        const rect = canvasRef.getBoundingClientRect();
        const width = rect.width;
        const height = rect.height;
        if (width <= 0 || height <= 0) return;

        // Handle high DPI
        const dpr = typeof window !== 'undefined' ? window.devicePixelRatio || 1 : 1;
        canvasRef.width = width * dpr;
        canvasRef.height = height * dpr;
        ctx.scale(dpr, dpr);

        ctx.clearRect(0, 0, width, height);

        const data = chartData();
        if (data.series.length === 0 || data.globalMax <= 0) return;

        const rows = data.series.length;
        const cellW = (width / COLUMNS);
        const cellH = (height / rows);

        // Draw the grid
        for (let r = 0; r < rows; r++) {
            const cy = r * cellH;
            // Convert series color (e.g., #10b981 or rgb(16, 185, 129)) to base for alpha manipulation
            // To keep it simple, we draw a solid cell with varying opacity.
            // But multiple transparent layers can look messy if overlapping, here they don't overlap.

            for (let c = 0; c < COLUMNS; c++) {
                const cx = c * cellW;
                const val = data.grid[r][c];

                if (val <= 0) {
                    // Empty cell placeholder
                    ctx.fillStyle = 'rgba(128, 128, 128, 0.05)';
                    ctx.fillRect(cx + PADDING_X / 2, cy + PADDING_Y / 2, cellW - PADDING_X, cellH - PADDING_Y);
                    continue;
                }

                // Logarithmic scale for color intensity (makes smaller spikes visible against massive ones)
                // normalized ranges from ~0.1 to 1.0
                const normalized = Math.log(1 + (val / data.globalMax) * 99) / Math.log(100);

                // Base opacity is clamped
                const opacity = clamp(normalized, 0.15, 1.0);

                ctx.globalAlpha = opacity;
                ctx.fillStyle = data.series[r].color;
                // Optional: add a tiny border radius using roundRect if browser supports it
                if (ctx.roundRect) {
                    ctx.beginPath();
                    ctx.roundRect(cx + PADDING_X / 2, cy + PADDING_Y / 2, cellW - PADDING_X, cellH - PADDING_Y, 2);
                    ctx.fill();
                } else {
                    ctx.fillRect(cx + PADDING_X / 2, cy + PADDING_Y / 2, cellW - PADDING_X, cellH - PADDING_Y);
                }
            }
        }
        ctx.globalAlpha = 1.0;
    };

    createEffect(() => drawCanvas());

    const handleMouseMove = (e: MouseEvent) => {
        if (!canvasRef) return;
        const rect = canvasRef.getBoundingClientRect();
        const data = chartData();

        const mouseX = clamp(e.clientX - rect.left, 0, rect.width - 1);
        const mouseY = clamp(e.clientY - rect.top, 0, rect.height - 1);

        const c = Math.floor((mouseX / rect.width) * COLUMNS);
        const r = Math.floor((mouseY / rect.height) * data.series.length);

        if (r >= 0 && r < data.series.length && c >= 0 && c < COLUMNS) {
            const val = data.grid[r][c];

            // Calculate tooltip position (center of the block)
            const cellW = rect.width / COLUMNS;
            const cellH = rect.height / data.series.length;
            const targetX = rect.left + (c * cellW) + (cellW / 2);
            const targetY = rect.top + (r * cellH);

            const bucketStartTime = data.windowStart + (c * data.bucketDuration);

            setHoveredState({
                tooltipX: targetX,
                tooltipY: targetY,
                timestamp: bucketStartTime,
                value: val,
                seriesName: data.series[r].name || 'Unknown',
                seriesColor: data.series[r].color,
            });
        } else {
            setHoveredState(null);
        }
    };

    const handleMouseLeave = () => {
        setHoveredState(null);
    };

    // Resize listener
    createEffect(() => {
        if (typeof window === 'undefined') return;
        const handleResize = () => drawCanvas();
        window.addEventListener('resize', handleResize);
        onCleanup(() => window.removeEventListener('resize', handleResize));
    });

    return (
        <div class="relative w-full h-full flex flex-col group" ref={chartSurfaceRef}>
            <div class="flex-1 relative min-h-0 bg-transparent flex">
                {/* Y-axis: typically in a density map we might just show "Top Nodes" */}
                <div class="absolute left-0 top-0 h-full w-full pointer-events-none flex flex-col justify-between py-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    {/* We could overlay series names here, but it might get messy. Let's rely on tooltip. */}
                </div>

                <div class="h-full ml-1 mr-1 flex-1">
                    <canvas
                        ref={canvasRef}
                        class="w-full h-full cursor-crosshair block"
                        onMouseMove={handleMouseMove}
                        onMouseLeave={handleMouseLeave}
                    />
                </div>
            </div>

            {/* X-axis labels */}
            <div class="relative h-4 mt-1 pointer-events-none mx-1">
                <span class="absolute left-0 top-0 text-[9px] font-medium leading-none text-slate-500 transition-all duration-300">
                    -{props.rangeLabel || props.timeRange}
                </span>
                <span class="absolute right-0 top-0 text-[9px] font-medium leading-none text-slate-500 transition-all duration-300">
                    now
                </span>
            </div>

            <Portal>
                <Show when={hoveredState()}>
                    {(hover) => (
                        <div
                            class="fixed pointer-events-none bg-slate-900 dark:bg-slate-800 text-white text-xs rounded px-2 py-1.5 shadow-sm border border-slate-700"
                            style={{
                                left: `${hover().tooltipX}px`,
                                top: `${hover().tooltipY - 6}px`,
                                transform: 'translate(-50%, -100%)',
                                'z-index': '9999',
                            }}
                        >
                            <div class="font-medium text-center mb-1 text-slate-300">{formatHoverTime(hover().timestamp)}</div>
                            <div class="flex items-center gap-1.5 leading-tight">
                                <span class="w-2 h-2 rounded-sm" style={{ background: hover().seriesColor }} />
                                <span class="font-semibold text-white">{hover().seriesName}</span>
                                <span class="ml-2 font-medium text-emerald-400">{formatValue(hover().value)}</span>
                            </div>
                        </div>
                    )}
                </Show>
            </Portal>
        </div>
    );
};

export default DensityMap;
