import { Show, createMemo, createSignal } from 'solid-js';
import { Portal } from 'solid-js/web';
import { formatBytes, formatPercent } from '@/utils/format';
import type { ZFSPool } from '@/types/api';

interface EnhancedStorageBarProps {
    used: number;
    total: number;
    free: number;
    zfsPool?: ZFSPool;
}

export function EnhancedStorageBar(props: EnhancedStorageBarProps) {
    const [showTooltip, setShowTooltip] = createSignal(false);
    const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });
    let containerRef: HTMLDivElement | undefined;

    const usagePercent = createMemo(() => {
        if (props.total <= 0) return 0;
        return (props.used / props.total) * 100;
    });

    const barColor = createMemo(() => {
        const p = usagePercent();
        if (p >= 90) return 'bg-red-500/60 dark:bg-red-500/50';
        if (p >= 80) return 'bg-yellow-500/60 dark:bg-yellow-500/50';
        return 'bg-green-500/60 dark:bg-green-500/50';
    });

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

    const handleMouseEnter = (e: MouseEvent) => {
        const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
        setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
        setShowTooltip(true);
    };

    const handleMouseLeave = () => {
        setShowTooltip(false);
    };

    return (
        <div ref={containerRef} class="metric-text w-full h-5 flex items-center justify-center">
            <div
                class="relative w-full max-w-[150px] h-full overflow-hidden bg-gray-200 dark:bg-gray-600 rounded"
                onMouseEnter={handleMouseEnter}
                onMouseLeave={handleMouseLeave}
            >
                {/* Usage Bar */}
                <div
                    class={`absolute top-0 left-0 h-full transition-all duration-300 ${barColor()}`}
                    style={{ width: `${Math.min(usagePercent(), 100)}%` }}
                />

                {/* Scrubbing/Resilvering Animation Overlay */}
                <Show when={isScrubbing() || isResilvering()}>
                    <div class="absolute inset-0 w-full h-full bg-[linear-gradient(45deg,transparent_25%,rgba(255,255,255,0.3)_50%,transparent_75%)] bg-[length:20px_20px] animate-[progress-bar-stripes_1s_linear_infinite]" />
                </Show>

                {/* Error Indicator (Red border/glow) */}
                <Show when={hasErrors()}>
                    <div class="absolute inset-0 border-2 border-red-500 animate-pulse rounded" />
                </Show>

                {/* Label */}
                <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none pointer-events-none">
                    <span class="whitespace-nowrap px-0.5">
                        {formatPercent(usagePercent())} (
                        {formatBytes(props.used, 0)}/
                        {formatBytes(props.total, 0)})
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
                        <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2 py-1.5 min-w-[160px] border border-gray-700">
                            <div class="font-medium mb-1 text-gray-300 border-b border-gray-700 pb-1">
                                Storage Details
                            </div>

                            <div class="flex justify-between gap-3 py-0.5">
                                <span class="text-gray-400">Used</span>
                                <span class="text-gray-200">{formatBytes(props.used)}</span>
                            </div>
                            <div class="flex justify-between gap-3 py-0.5">
                                <span class="text-gray-400">Free</span>
                                <span class="text-gray-200">{formatBytes(props.free)}</span>
                            </div>
                            <div class="flex justify-between gap-3 py-0.5 border-t border-gray-700/50 mt-0.5 pt-0.5">
                                <span class="text-gray-400">Total</span>
                                <span class="text-gray-200">{formatBytes(props.total)}</span>
                            </div>

                            <Show when={props.zfsPool}>
                                <div class="mt-1 pt-1 border-t border-gray-600">
                                    <div class="font-medium mb-0.5 text-blue-300">ZFS Status</div>
                                    <div class="flex justify-between gap-3 py-0.5">
                                        <span class="text-gray-400">State</span>
                                        <span class={hasErrors() ? 'text-red-400 font-bold' : 'text-green-400'}>
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
                    </div>
                </Portal>
            </Show>
        </div>
    );
}
