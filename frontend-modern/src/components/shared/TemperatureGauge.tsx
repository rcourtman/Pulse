import { Component, createMemo, Show } from 'solid-js';

interface TemperatureGaugeProps {
    value: number;
    min?: number | null;
    max?: number | null;
    critical?: number;
    warning?: number;
    label?: string;
    class?: string;
}

export const TemperatureGauge: Component<TemperatureGaugeProps> = (props) => {
    const critical = props.critical ?? 80;
    const warning = props.warning ?? 70;

    // Calculate percentage (assuming 0-100°C range for simplicity, or slightly dynamic)
    // Most CPUs idle around 30-40, max out at 100.
    const percent = createMemo(() => Math.min(100, Math.max(0, props.value)));

    const colorClass = createMemo(() => {
        if (props.value >= critical) return 'bg-red-500 dark:bg-red-500';
        if (props.value >= warning) return 'bg-yellow-500 dark:bg-yellow-500';
        return 'bg-green-500 dark:bg-green-500';
    });

    const textColorClass = createMemo(() => {
        if (props.value >= critical) return 'text-red-700 dark:text-red-400';
        if (props.value >= warning) return 'text-yellow-700 dark:text-yellow-400';
        return 'text-green-700 dark:text-green-400';
    });

    return (
        <div class={`flex items-center gap-2 ${props.class || ''}`}>
            {/* Text Value */}
            <span class={`text-xs font-medium w-[36px] text-right ${textColorClass()}`}>
                {Math.round(props.value)}°C
            </span>

            {/* Bar */}
            <div class="relative flex-1 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden min-w-[40px] max-w-[80px]">
                <div
                    class={`h-full rounded-full transition-all duration-500 ${colorClass()}`}
                    style={{ width: `${percent()}%` }}
                />

                {/* Min Marker */}
                <Show when={props.min !== null && props.min !== undefined}>
                    <div
                        class="absolute top-0 bottom-0 w-0.5 bg-blue-400 opacity-70"
                        style={{ left: `${Math.min(100, Math.max(0, props.min!))}%` }}
                        title={`Min: ${Math.round(props.min!)}°C`}
                    />
                </Show>

                {/* Max Marker */}
                <Show when={props.max !== null && props.max !== undefined}>
                    <div
                        class="absolute top-0 bottom-0 w-0.5 bg-red-400 opacity-70"
                        style={{ left: `${Math.min(100, Math.max(0, props.max!))}%` }}
                        title={`Max: ${Math.round(props.max!)}°C`}
                    />
                </Show>
            </div>
        </div>
    );
};
