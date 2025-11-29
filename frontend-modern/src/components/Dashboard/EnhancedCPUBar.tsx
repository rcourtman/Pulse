import { Show, createMemo, createSignal } from 'solid-js';
import { Portal } from 'solid-js/web';
import { formatPercent } from '@/utils/format';
import { useMetricsViewMode } from '@/stores/metricsViewMode';
import { getMetricHistory } from '@/stores/metricsHistory';
import { Sparkline } from '@/components/shared/Sparkline';

interface EnhancedCPUBarProps {
    usage: number;          // CPU Usage % (0-100)
    loadAverage?: number;   // 1-minute load average
    cores?: number;         // Number of cores
    model?: string;         // CPU Model name (for tooltip)
    resourceId?: string;    // For sparkline history
}

export function EnhancedCPUBar(props: EnhancedCPUBarProps) {
    const [showTooltip, setShowTooltip] = createSignal(false);
    const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });
    let containerRef: HTMLDivElement | undefined;

    // Calculate Load % relative to core count
    const loadPercent = createMemo(() => {
        if (props.loadAverage === undefined || !props.cores || props.cores === 0) return 0;
        return (props.loadAverage / props.cores) * 100;
    });

    const isOverloaded = createMemo(() => loadPercent() > 100);

    // Bar color based on usage
    const barColor = createMemo(() => {
        if (props.usage >= 90) return 'bg-red-500/60 dark:bg-red-500/50';
        if (props.usage >= 80) return 'bg-yellow-500/60 dark:bg-yellow-500/50';
        return 'bg-green-500/60 dark:bg-green-500/50';
    });

    // Load marker position (capped at 100%)
    const markerPosition = createMemo(() => Math.min(loadPercent(), 100));

    const handleMouseEnter = (e: MouseEvent) => {
        const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
        setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
        setShowTooltip(true);
    };

    const handleMouseLeave = () => {
        setShowTooltip(false);
    };

    const { viewMode } = useMetricsViewMode();

    // Get metric history for sparkline
    const metricHistory = createMemo(() => {
        if (viewMode() !== 'sparklines' || !props.resourceId) return [];
        return getMetricHistory(props.resourceId);
    });

    return (
        <Show
            when={viewMode() === 'sparklines' && props.resourceId}
            fallback={
                <div ref={containerRef} class="metric-text w-full h-4 flex items-center justify-center">
                    <div
                        class="relative w-full max-w-[140px] h-full overflow-hidden bg-gray-200 dark:bg-gray-600 rounded cursor-help"
                        onMouseEnter={handleMouseEnter}
                        onMouseLeave={handleMouseLeave}
                    >
                        {/* Usage Bar */}
                        <div
                            class={`absolute top-0 left-0 h-full transition-all duration-300 ${barColor()}`}
                            style={{ width: `${Math.min(props.usage, 100)}%` }}
                        />

                        {/* Load Average Marker */}
                        <Show when={props.loadAverage !== undefined && props.cores}>
                            <div
                                class={`absolute top-0 bottom-0 w-[2px] z-10 transition-all duration-300 ${isOverloaded() ? 'bg-purple-600 dark:bg-purple-400 shadow-[0_0_4px_rgba(147,51,234,0.8)]' : 'bg-gray-800 dark:bg-gray-200 opacity-60'
                                    }`}
                                style={{ left: `${markerPosition()}%` }}
                            />
                        </Show>

                        {/* Label */}
                        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-gray-700 dark:text-gray-100 leading-none pointer-events-none">
                            {formatPercent(props.usage)}
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
                                <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2 py-1.5 min-w-[160px] border border-gray-700">
                                    <div class="font-medium mb-1 text-gray-300 border-b border-gray-700 pb-1">
                                        CPU Details
                                    </div>

                                    <Show when={props.model}>
                                        <div class="text-[9px] text-gray-400 mb-1.5 truncate max-w-[200px]">
                                            {props.model}
                                        </div>
                                    </Show>

                                    <div class="flex justify-between gap-3 py-0.5">
                                        <span class="text-gray-400">Usage</span>
                                        <span class={`font-medium ${props.usage > 90 ? 'text-red-400' : 'text-gray-200'}`}>
                                            {formatPercent(props.usage)}
                                        </span>
                                    </div>

                                    <Show when={props.loadAverage !== undefined && props.cores}>
                                        <div class="flex justify-between gap-3 py-0.5">
                                            <span class="text-gray-400">Load (1m)</span>
                                            <span class={`font-medium ${isOverloaded() ? 'text-purple-400' : 'text-gray-200'}`}>
                                                {props.loadAverage?.toFixed(2)}
                                            </span>
                                        </div>
                                        <div class="flex justify-between gap-3 py-0.5">
                                            <span class="text-gray-400">Capacity</span>
                                            <span class={`font-medium ${isOverloaded() ? 'text-purple-400' : 'text-gray-200'}`}>
                                                {loadPercent().toFixed(0)}% of {props.cores} cores
                                            </span>
                                        </div>
                                    </Show>
                                </div>
                            </div>
                        </Portal>
                    </Show>
                </div>
            }
        >
            {/* Sparkline mode */}
            <div class="metric-text w-full h-6 flex items-center gap-1.5">
                <div class="flex-1 min-w-0">
                    <Sparkline
                        data={metricHistory()}
                        metric="cpu"
                        width={0}
                        height={24}
                    />
                </div>
                <span class="text-[10px] font-medium text-gray-800 dark:text-gray-100 whitespace-nowrap flex-shrink-0 min-w-[35px]">
                    {formatPercent(props.usage)}
                </span>
            </div>
        </Show>
    );
}
