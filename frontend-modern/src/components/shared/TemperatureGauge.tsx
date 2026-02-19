import { Component, createMemo } from 'solid-js';
import { formatTemperature } from '@/utils/temperature';

interface TemperatureGaugeProps {
    value: number;
    min?: number | null;
    max?: number | null;
    critical?: number;
    warning?: number;
    class?: string;
}

export const TemperatureGauge: Component<TemperatureGaugeProps> = (props) => {
    const critical = props.critical ?? 80;
    const warning = props.warning ?? 70;

    const textColorClass = createMemo(() => {
        if (props.value >= critical) return 'text-red-600 dark:text-red-400';
        if (props.value >= warning) return 'text-yellow-600 dark:text-yellow-400';
        return 'text-slate-600 dark:text-slate-400';
    });

    return (
        <span class={`text-xs whitespace-nowrap ${textColorClass()} ${props.class || ''}`}>
            {formatTemperature(props.value)}
        </span>
    );
};
