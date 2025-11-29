import { Show, For, createMemo, createSignal, onMount, onCleanup } from 'solid-js';
import { Portal } from 'solid-js/web';
import { formatBytes, formatPercent } from '@/utils/format';

interface StackedMemoryBarProps {
    used: number;
    total: number;
    swapUsed?: number;
    swapTotal?: number;
    balloon?: number;
}

// Colors for memory segments
const MEMORY_COLORS = {
    active: 'rgba(34, 197, 94, 0.6)',   // green
    balloon: 'rgba(234, 179, 8, 0.6)',  // yellow
    swap: 'rgba(168, 85, 247, 0.6)',    // purple
};

export function StackedMemoryBar(props: StackedMemoryBarProps) {
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
        // Active memory is used minus balloon (since balloon is "used" but reclaimed)
        // Note: This is a simplification. Proxmox reports 'used' including balloon.
        const active = Math.max(0, props.used - balloon);

        // Calculate percentages relative to RAM total (swap is extra)
        // We visualize swap as an overlay or separate segment if we want to show it relative to RAM
        // But typically swap is separate. 
        // Let's stack RAM components (Active + Balloon) within the RAM bar.
        // And maybe show Swap as a separate indicator or just in tooltip?
        // The user asked for "Detailed Memory Composition".
        // Let's stack Active and Balloon within the main bar.
        // If Swap is used, we can perhaps show it as a segment that "overflows" or just a separate color if we treat total as RAM+Swap?
        // Standard approach: Bar represents RAM. Segments are Active, Balloon.
        // Swap is usually separate. But we can include it if we want to show "Memory Pressure".

        // Let's stick to RAM composition for the bar: Active (Green), Balloon (Yellow).
        // Swap usage will be shown in tooltip and maybe change the bar color if critical?
        // Actually, let's try to include Swap if it's significant, but it might be confusing if it exceeds 100% of RAM.

        // Alternative: The bar is RAM.
        // Green: Active
        // Yellow: Balloon

        const activePercent = (active / props.total) * 100;
        const balloonPercent = (balloon / props.total) * 100;

        const segs = [
            { type: 'Active', bytes: active, percent: activePercent, color: MEMORY_COLORS.active },
            { type: 'Balloon', bytes: balloon, percent: balloonPercent, color: MEMORY_COLORS.balloon },
        ].filter(s => s.bytes > 0);

        return segs;
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
        <div ref={containerRef} class="metric-text w-full h-4 flex items-center justify-center">
            <div
                class="relative w-full max-w-[140px] h-full overflow-hidden bg-gray-200 dark:bg-gray-600 rounded cursor-help"
                onMouseEnter={handleMouseEnter}
                onMouseLeave={handleMouseLeave}
            >
                {/* Stacked segments */}
                <div class="absolute top-0 left-0 h-full w-full flex">
                    <For each={segments()}>
                        {(segment, idx) => (
                            <div
                                class="h-full transition-all duration-300"
                                style={{
                                    width: `${segment.percent}%`,
                                    'background-color': segment.color,
                                    'border-right': idx() < segments().length - 1 ? '1px solid rgba(255,255,255,0.3)' : 'none',
                                }}
                            />
                        )}
                    </For>
                </div>

                {/* Swap Indicator (Thin line at bottom if swap is used) */}
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
                                <span class="text-green-400">Active</span>
                                <span class="whitespace-nowrap text-gray-300">
                                    {formatBytes(Math.max(0, props.used - (props.balloon || 0)), 0)}
                                </span>
                            </div>

                            <Show when={(props.balloon || 0) > 0}>
                                <div class="flex justify-between gap-3 py-0.5 border-t border-gray-700/50">
                                    <span class="text-yellow-400">Balloon</span>
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
    );
}
