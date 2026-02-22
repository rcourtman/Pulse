import { Show, createMemo } from 'solid-js';
import { formatBytes, formatPercent } from '@/utils/format';
import { useTooltip } from '@/hooks/useTooltip';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { getMetricColorClass } from '@/utils/metricThresholds';
import type { ZFSPool } from '@/types/api';

interface EnhancedStorageBarProps {
    used: number;
    total: number;
    free: number;
    zfsPool?: ZFSPool;
}

export function EnhancedStorageBar(props: EnhancedStorageBarProps) {
    const tip = useTooltip();

    const usagePercent = createMemo(() => {
        if (props.total <= 0) return 0;
        return (props.used / props.total) * 100;
    });

    const barColor = createMemo(() => getMetricColorClass(usagePercent(), 'disk'));

    const isScrubbing = createMemo(() => {
        return props.zfsPool?.scan?.toLowerCase().includes('scrub') ?? false;
    });

    const isResilvering = createMemo(() => {
        return props.zfsPool?.scan?.toLowerCase().includes('resilver') ?? false;
 });

 const hasErrors = createMemo(() => {
 if (!props.zfsPool) return false;
 return (
 props.zfsPool.readErrors > 0 ||
 props.zfsPool.writeErrors > 0 ||
 props.zfsPool.checksumErrors > 0
 );
 });

 return (
 <div class="metric-text w-full h-5 flex items-center min-w-0">
 <div
 class="relative w-full h-full overflow-hidden bg-surface-hover rounded"
 onMouseEnter={tip.onMouseEnter}
 onMouseLeave={tip.onMouseLeave}
 >
 {/* Usage Bar */}
 <div
 class={`absolute top-0 left-0 h-full transition-all duration-300 ${barColor()}`}
 style={{ width: `${Math.min(usagePercent(), 100)}%` }}
 />

 {/* Scrubbing/Resilvering Animation Overlay */}
 <Show when={isScrubbing() || isResilvering()}>
 <div class="absolute inset-0 w-full h-full animate-pulse" />
 </Show>

 {/* Error Indicator (Red border/glow) */}
 <Show when={hasErrors()}>
 <div class="absolute inset-0 border-2 border-red-500 animate-pulse rounded" />
 </Show>

 {/* Label */}
 <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-base-content leading-none pointer-events-none min-w-0 overflow-hidden">
 <span class="max-w-full min-w-0 whitespace-nowrap overflow-hidden text-ellipsis px-0.5 text-center">
 {formatPercent(usagePercent())} (
 {formatBytes(props.used)}/
 {formatBytes(props.total)})
 </span>
 </span>
 </div>

 {/* Tooltip */}
 <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
 <div class="min-w-[160px]">
 <div class="font-medium mb-1 text-slate-300 border-b border-slate-700 pb-1">
 Storage Details
 </div>

 <div class="flex justify-between gap-3 py-0.5">
 <span class="text-slate-400">Used</span>
 <span class="text-slate-200">{formatBytes(props.used)}</span>
 </div>
 <div class="flex justify-between gap-3 py-0.5">
 <span class="text-slate-400">Free</span>
 <span class="text-slate-200">{formatBytes(props.free)}</span>
 </div>
 <div class="flex justify-between gap-3 py-0.5 border-t border-slate-700 mt-0.5 pt-0.5">
 <span class="text-slate-400">Total</span>
 <span class="text-slate-200">{formatBytes(props.total)}</span>
 </div>

 <Show when={props.zfsPool}>
 <div class="mt-1 pt-1 border-t border-slate-600">
 <div class="font-medium mb-0.5 text-blue-300">ZFS Status</div>
 <div class="flex justify-between gap-3 py-0.5">
 <span class="text-slate-400">State</span>
 <span class={hasErrors() ?'text-red-400 font-bold' : 'text-green-400'}>
                                    {props.zfsPool?.state}
                                </span>
                            </div>
                            <Show when={props.zfsPool?.scan && props.zfsPool.scan !== 'none'}>
                                <div class="text-yellow-400 italic mt-0.5 max-w-[200px] break-words">
                                    {props.zfsPool?.scan}
                                </div>
                            </Show>
                            <Show when={hasErrors()}>
                                <div class="text-red-400 mt-0.5 font-bold">
                                    Errors: R:{props.zfsPool?.readErrors} W:{props.zfsPool?.writeErrors} C:{props.zfsPool?.checksumErrors}
                                </div>
                            </Show>
                        </div>
                    </Show>
                </div>
            </TooltipPortal>
        </div>
    );
}
