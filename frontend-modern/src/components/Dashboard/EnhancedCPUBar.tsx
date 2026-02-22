import { Show, createMemo } from 'solid-js';
import { formatPercent, formatAnomalyRatio, ANOMALY_SEVERITY_CLASS } from '@/utils/format';
import { useTooltip } from '@/hooks/useTooltip';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { getMetricColorClass } from '@/utils/metricThresholds';
import type { AnomalyReport } from '@/types/aiIntelligence';

interface EnhancedCPUBarProps {
    usage: number;          // CPU Usage % (0-100)
    loadAverage?: number;   // 1-minute load average
    cores?: number;         // Number of cores
    model?: string;         // CPU Model name (for tooltip)
    resourceId?: string;
    anomaly?: AnomalyReport | null;  // Baseline anomaly if detected
}

export function EnhancedCPUBar(props: EnhancedCPUBarProps) {
    const tip = useTooltip();

    // Bar color based on usage (from centralized thresholds)
    const barColor = createMemo(() => getMetricColorClass(props.usage, 'cpu'));

 const anomalyRatio = createMemo(() => formatAnomalyRatio(props.anomaly));

 return (
 <div class="metric-text w-full h-4 flex items-center justify-center">
 <div
 class="relative w-full h-full overflow-hidden bg-surface-hover rounded"
 onMouseEnter={tip.onMouseEnter}
 onMouseLeave={tip.onMouseLeave}
 >
 {/* Usage Bar */}
 <div
 class={`absolute top-0 left-0 h-full transition-all duration-300 ${barColor()}`}
 style={{ width: `${Math.min(props.usage, 100)}%` }}
 />

 {/* Label with optional anomaly indicator */}
 <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-slate-700 leading-none pointer-events-none">
 {formatPercent(props.usage)}
 <Show when={props.cores}>
 <span class="hidden sm:inline font-normal text-muted ml-1">({props.cores})</span>
 </Show>
 {/* Anomaly indicator */}
 <Show when={props.anomaly && anomalyRatio()}>
 <span
 class={`ml-1 font-bold animate-pulse ${ANOMALY_SEVERITY_CLASS[props.anomaly!.severity] ||'text-yellow-400'}`}
                            title={props.anomaly!.description}
                        >
                            {anomalyRatio()}
                        </span>
                    </Show>
                </span>
            </div>

            {/* Tooltip */}
            <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
                <div class="min-w-[160px]">
                    <div class="font-medium mb-1 text-slate-300 border-b border-slate-700 pb-1">
                        CPU Details
                    </div>

                    <Show when={props.model}>
                        <div class="text-[9px] text-slate-400 mb-1.5 truncate max-w-[200px]">
                            {props.model}
                        </div>
                    </Show>

                    <div class="flex justify-between gap-3 py-0.5">
                        <span class="text-slate-400">Usage</span>
                        <span class={`font-medium ${props.usage > 90 ? 'text-red-400' : 'text-slate-200'}`}>
                            {formatPercent(props.usage)}
                        </span>
                    </div>

                    <Show when={props.loadAverage !== undefined}>
                        <div class="flex justify-between gap-3 py-0.5">
                            <span class="text-slate-400">Load (1m)</span>
                            <span class="font-medium text-slate-200">
                                {props.loadAverage?.toFixed(2)}
                            </span>
                        </div>
                    </Show>
                </div>
            </TooltipPortal>
        </div>
    );
}
