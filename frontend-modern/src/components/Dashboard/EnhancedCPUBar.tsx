import { Show, createMemo, createSignal } from 'solid-js';
import { Portal } from 'solid-js/web'
import { formatPercent } from '@/utils/format';
import { useMetricsViewMode } from '@/stores/metricsViewMode';
import { getMetricHistoryForRange, getMetricsVersion } from '@/stores/metricsHistory';
import { Sparkline } from '@/components/shared/Sparkline';
import type { AnomalyReport } from '@/types/aiIntelligence';

interface EnhancedCPUBarProps {
    usage: number;          // CPU Usage % (0-100)
    loadAverage?: number;   // 1-minute load average
    cores?: number;         // Number of cores
    model?: string;         // CPU Model name (for tooltip)
    resourceId?: string;    // For sparkline history
    anomaly?: AnomalyReport | null;  // Baseline anomaly if detected
}

// Anomaly severity colors
const anomalySeverityClass: Record<string, string> = {
    critical: 'text-red-400',
    high: 'text-orange-400',
    medium: 'text-yellow-400',
    low: 'text-blue-400',
};

export function EnhancedCPUBar(props: EnhancedCPUBarProps) {
    const [showTooltip, setShowTooltip] = createSignal(false);
    const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });
    let containerRef: HTMLDivElement | undefined;

    // Bar color based on usage
    const barColor = createMemo(() => {
        if (props.usage >= 90) return 'bg-red-500/60 dark:bg-red-500/50';
        if (props.usage >= 80) return 'bg-yellow-500/60 dark:bg-yellow-500/50';
        return 'bg-green-500/60 dark:bg-green-500/50';
    });

    // Format anomaly ratio for display
    const anomalyRatio = createMemo(() => {
        if (!props.anomaly || props.anomaly.baseline_mean === 0) return null;
        const ratio = props.anomaly.current_value / props.anomaly.baseline_mean;
        if (ratio >= 2) return `${ratio.toFixed(1)}x`;
        if (ratio >= 1.5) return '↑↑';
        return '↑';
    });

    const handleMouseEnter = (e: MouseEvent) => {
        const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
        setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
        setShowTooltip(true);
    };

    const handleMouseLeave = () => {
        setShowTooltip(false);
    };

    const { viewMode, timeRange } = useMetricsViewMode();

    // Get metric history for sparkline
    // Depends on metricsVersion to re-fetch when data is seeded (e.g., on time range change)
    const metricHistory = createMemo(() => {
        // Subscribe to version changes so we re-read when new data is seeded
        getMetricsVersion();
        if (viewMode() !== 'sparklines' || !props.resourceId) return [];
        return getMetricHistoryForRange(props.resourceId, timeRange());
    });

    return (
        <Show
            when={viewMode() === 'sparklines' && props.resourceId}
            fallback={
                // Progress bar mode - full width, flex centered like stacked bars
                <div ref={containerRef} class="metric-text w-full h-4 flex items-center justify-center">
                    <div
                        class="relative w-full h-full overflow-hidden bg-gray-200 dark:bg-gray-600 rounded"
                        onMouseEnter={handleMouseEnter}
                        onMouseLeave={handleMouseLeave}
                    >
                        {/* Usage Bar */}
                        <div
                            class={`absolute top-0 left-0 h-full transition-all duration-300 ${barColor()}`}
                            style={{ width: `${Math.min(props.usage, 100)}%` }}
                        />

                        {/* Label with optional anomaly indicator */}
                        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-gray-700 dark:text-gray-100 leading-none pointer-events-none">
                            {formatPercent(props.usage)}
                            <Show when={props.cores}>
                                <span class="font-normal text-gray-500 dark:text-gray-300 ml-1">({props.cores} cores)</span>
                            </Show>
                            {/* Anomaly indicator */}
                            <Show when={props.anomaly && anomalyRatio()}>
                                <span
                                    class={`ml-1 font-bold animate-pulse ${anomalySeverityClass[props.anomaly!.severity] || 'text-yellow-400'}`}
                                    title={props.anomaly!.description}
                                >
                                    {anomalyRatio()}
                                </span>
                            </Show>
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

                                    <Show when={props.loadAverage !== undefined}>
                                        <div class="flex justify-between gap-3 py-0.5">
                                            <span class="text-gray-400">Load (1m)</span>
                                            <span class="font-medium text-gray-200">
                                                {props.loadAverage?.toFixed(2)}
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
            {/* Sparkline mode - full width, flex centered like stacked bars */}
            <div class="metric-text w-full h-4 flex items-center justify-center min-w-0 overflow-hidden">
                <Sparkline
                    data={metricHistory()}
                    metric="cpu"
                    width={0}
                    height={16}
                />
            </div>
        </Show>
    );
}
