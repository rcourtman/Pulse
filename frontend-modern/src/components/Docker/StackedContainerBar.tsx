import { Show, For, createMemo, createSignal } from 'solid-js';
import { Portal } from 'solid-js/web';

interface StackedContainerBarProps {
    running: number;
    stopped: number;
    error: number;
    total: number;
}

// Colors for container states
const STATE_COLORS = {
    running: 'rgba(34, 197, 94, 0.6)',   // green
    stopped: 'rgba(156, 163, 175, 0.6)', // gray
    error: 'rgba(239, 68, 68, 0.6)',     // red
};

export function StackedContainerBar(props: StackedContainerBarProps) {
    const [showTooltip, setShowTooltip] = createSignal(false);
    const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });
    let containerRef: HTMLDivElement | undefined;

    // Calculate segments
    const segments = createMemo(() => {
        if (props.total <= 0) return [];

        const runningPercent = (props.running / props.total) * 100;
        const stoppedPercent = (props.stopped / props.total) * 100;
        const errorPercent = (props.error / props.total) * 100;

        return [
            { type: 'Running', count: props.running, percent: runningPercent, color: STATE_COLORS.running },
            { type: 'Stopped', count: props.stopped, percent: stoppedPercent, color: STATE_COLORS.stopped },
            { type: 'Error', count: props.error, percent: errorPercent, color: STATE_COLORS.error },
        ].filter(s => s.count > 0);
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
                class="relative w-full h-full overflow-hidden bg-gray-200 dark:bg-gray-600 rounded"
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

                {/* Label overlay */}
                <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-gray-700 dark:text-gray-100 leading-none pointer-events-none">
                    <span class="flex items-center gap-1 whitespace-nowrap px-0.5">
                        <span>{props.running}/{props.total}</span>
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
                        <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2 py-1.5 min-w-[120px] border border-gray-700">
                            <div class="font-medium mb-1 text-gray-300 border-b border-gray-700 pb-1">
                                Container Status
                            </div>
                            <For each={segments()}>
                                {(item, idx) => (
                                    <div class="flex justify-between gap-3 py-0.5" classList={{ 'border-t border-gray-700/50': idx() > 0 }}>
                                        <span
                                            class="truncate"
                                            style={{ color: item.color.replace('0.6)', '1)') }}
                                        >
                                            {item.type}
                                        </span>
                                        <span class="whitespace-nowrap text-gray-300">
                                            {item.count}
                                        </span>
                                    </div>
                                )}
                            </For>
                        </div>
                    </div>
                </Portal>
            </Show>
        </div>
    );
}
