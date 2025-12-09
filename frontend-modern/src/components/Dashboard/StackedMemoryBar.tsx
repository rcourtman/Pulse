import { Show, For, createMemo, createSignal, onMount, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import { formatBytes, formatPercent } from '@/utils/format';
import { Sparkline } from '@/components/shared/Sparkline';
import { useMetricsViewMode } from '@/stores/metricsViewMode';
import { getMetricHistoryForRange, getMetricsVersion } from '@/stores/metricsHistory';

interface StackedMemoryBarProps {
    used: number;
    total: number;
    swapUsed?: number;
    swapTotal?: number;
    balloon?: number;
    resourceId?: string; // Required for sparkline mode to fetch history
}

// Colors for memory segments
const MEMORY_COLORS = {
    active: 'rgba(34, 197, 94, 0.6)',   // green
    balloon: 'rgba(234, 179, 8, 0.6)',  // yellow
    swap: 'rgba(168, 85, 247, 0.6)',    // purple
};

export function StackedMemoryBar(props: StackedMemoryBarProps) {
    const { viewMode, timeRange } = useMetricsViewMode();

    // Get metric history for sparkline
    // Depends on metricsVersion to re-fetch when data is seeded (e.g., on time range change)
    const metricHistory = createMemo(() => {
        // Subscribe to version changes so we re-read when new data is seeded
        getMetricsVersion();
        if (viewMode() !== 'sparklines' || !props.resourceId) return [];
        return getMetricHistoryForRange(props.resourceId, timeRange());
    });

    const [containerWidth, setContainerWidth] = createSignal(100);
    const [showTooltip, setShowTooltip] = createSignal(false);
    const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });
    let containerRef: HTMLDivElement | undefined;

    onMount(() => {
        if (!containerRef) return;
        setContainerWidth(containerRef.offsetWidth);

        const observer = new ResizeObserver((entries) => {
            for (const entry of entries) {
                setContainerWidth(entry.contentRect.width);
            }
        });

        observer.observe(containerRef);
        onCleanup(() => observer.disconnect());
    });

    // Calculate segments
    const segments = createMemo(() => {
        if (props.total <= 0) return [];

        const balloon = props.balloon || 0;

        // Proxmox balloon semantics:
        // - balloon = 0: ballooning not enabled/configured
        // - balloon = total (maxmem): ballooning configured but at max (no actual ballooning)
        // - balloon < total: active ballooning, guest limited to 'balloon' bytes
        //
        // Only show balloon segment when actual ballooning is in effect
        const hasActiveBallooning = balloon > 0 && balloon < props.total;

        // Used memory is what the guest is actually consuming
        const usedPercent = (props.used / props.total) * 100;

        if (hasActiveBallooning) {
            // With active ballooning:
            // - Green: actual used memory
            // - Yellow: balloon limit marker (shows where the guest is capped)
            // The balloon limit shows as a segment from used to balloon
            const balloonLimitPercent = Math.max(0, (balloon / props.total) * 100 - usedPercent);

            const segs = [
                { type: 'Active', bytes: props.used, percent: usedPercent, color: MEMORY_COLORS.active },
            ];

            // Only show balloon segment if there's room between used and balloon limit
            if (balloonLimitPercent > 0 && balloon > props.used) {
                segs.push({
                    type: 'Balloon',
                    bytes: balloon - props.used,
                    percent: balloonLimitPercent,
                    color: MEMORY_COLORS.balloon,
                });
            }

            return segs.filter(s => s.bytes > 0);
        }

        // No active ballooning - just show used memory as green
        return [
            { type: 'Active', bytes: props.used, percent: usedPercent, color: MEMORY_COLORS.active },
        ].filter(s => s.bytes > 0);
    });

    const swapPercent = createMemo(() => {
        if (!props.swapTotal || props.swapTotal <= 0) return 0;
        return (props.swapUsed || 0) / props.swapTotal * 100;
    });

    const hasSwap = createMemo(() => (props.swapTotal || 0) > 0);

    const displayLabel = createMemo(() => {
        const percent = (props.used / props.total) * 100;
        return formatPercent(percent);
    });

    const displaySublabel = createMemo(() => {
        return `${formatBytes(props.used, 0)}/${formatBytes(props.total, 0)}`;
    });

    // Estimate text width for label fitting (approx 6px per char + padding)
    const estimateTextWidth = (text: string): number => {
        return text.length * 6 + 10;
    };

    const showSublabel = createMemo(() => {
        const fullText = `${displayLabel()} (${displaySublabel()})`;
        return containerWidth() >= estimateTextWidth(fullText);
    });

    const handleMouseEnter = (e: MouseEvent) => {
        const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
        setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
        setShowTooltip(true);
    };

    const handleMouseLeave = () => {
        setShowTooltip(false);
    };

    return (
        <Show
            when={viewMode() === 'sparklines' && props.resourceId}
            fallback={
                <div ref={containerRef} class="metric-text w-full h-4 flex items-center justify-center">
                    <div
                        class="relative w-full h-full overflow-hidden bg-gray-200 dark:bg-gray-600 rounded cursor-help"
                        onMouseEnter={handleMouseEnter}
                        onMouseLeave={handleMouseLeave}
                    >
                        {/* Stacked segments - use absolute positioning like MetricBar for correct width scaling */}
                        <For each={segments()}>
                            {(segment, idx) => {
                                // Calculate left offset as sum of previous segments
                                const leftOffset = () => {
                                    let offset = 0;
                                    const segs = segments();
                                    for (let i = 0; i < idx(); i++) {
                                        offset += segs[i].percent;
                                    }
                                    return offset;
                                };
                                return (
                                    <div
                                        class="absolute top-0 h-full transition-all duration-300"
                                        style={{
                                            left: `${leftOffset()}%`,
                                            width: `${segment.percent}%`,
                                            'background-color': segment.color,
                                            'border-right': idx() < segments().length - 1 ? '1px solid rgba(255,255,255,0.3)' : 'none',
                                        }}
                                    />
                                );
                            }}
                        </For>

                        {/* Swap Indicator (Thin line at bottom if swap is used) */}
                        <Show when={hasSwap() && (props.swapUsed || 0) > 0}>
                            <div
                                class="absolute bottom-0 left-0 h-[3px] w-full bg-purple-500"
                                style={{ width: `${Math.min(swapPercent(), 100)}%` }}
                            />
                        </Show>

                        {/* Label overlay */}
                        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-gray-700 dark:text-gray-100 leading-none pointer-events-none">
                            <span class="flex items-center gap-1 whitespace-nowrap px-0.5">
                                <span>{displayLabel()}</span>
                                <Show when={showSublabel()}>
                                    <span class="metric-sublabel font-normal text-gray-500 dark:text-gray-300">
                                        ({displaySublabel()})
                                    </span>
                                </Show>
                            </span>
                        </span>
                    </div>

                    {/* Tooltip */}
                    <Show when={showTooltip()}>
                        <Portal mount={document.body}>
                            <div
                                class="fixed z-[9999] pointer-events-none"
                                style={{
                                    left: `${tooltipPos().x}px`,
                                    top: `${tooltipPos().y - 8}px`,
                                    transform: 'translate(-50%, -100%)',
                                }}
                            >
                                <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2 py-1.5 min-w-[140px] border border-gray-700">
                                    <div class="font-medium mb-1 text-gray-300 border-b border-gray-700 pb-1">
                                        Memory Composition
                                    </div>

                                    {/* RAM Breakdown */}
                                    <div class="flex justify-between gap-3 py-0.5">
                                        <span class="text-green-400">Used</span>
                                        <span class="whitespace-nowrap text-gray-300">
                                            {formatBytes(props.used, 0)}
                                        </span>
                                    </div>

                                    <Show when={(props.balloon || 0) > 0 && (props.balloon || 0) < props.total}>
                                        <div class="flex justify-between gap-3 py-0.5 border-t border-gray-700/50">
                                            <span class="text-yellow-400">Balloon Limit</span>
                                            <span class="whitespace-nowrap text-gray-300">
                                                {formatBytes(props.balloon || 0, 0)}
                                            </span>
                                        </div>
                                    </Show>

                                    <div class="flex justify-between gap-3 py-0.5 border-t border-gray-700/50">
                                        <span class="text-gray-400">Free</span>
                                        <span class="whitespace-nowrap text-gray-300">
                                            {formatBytes(props.total - props.used, 0)}
                                        </span>
                                    </div>

                                    {/* Swap Section */}
                                    <Show when={hasSwap()}>
                                        <div class="mt-1 pt-1 border-t border-gray-600">
                                            <div class="flex justify-between gap-3 py-0.5">
                                                <span class="text-purple-400">Swap</span>
                                                <span class="whitespace-nowrap text-gray-300">
                                                    {formatBytes(props.swapUsed || 0, 0)} / {formatBytes(props.swapTotal || 0, 0)}
                                                </span>
                                            </div>
                                        </div>
                                    </Show>
                                </div>
                            </div>
                        </Portal>
                    </Show>
                </div>
            }
        >
            {/* Sparkline mode - full width, flex centered like stacked bars */}
            <div class="metric-text w-full h-4 flex items-center justify-center min-w-0 overflow-hidden">
                <Sparkline
                    data={metricHistory()}
                    metric="memory"
                    width={0}
                    height={16}
                />
            </div>
        </Show>
    );
}
