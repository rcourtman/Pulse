/**
 * ThresholdBadge Component
 *
 * A pill-shaped badge that displays a threshold value with color coding
 * based on severity. Used in resource cards for at-a-glance threshold visibility.
 */

import { Component, Show } from 'solid-js';
import { getThresholdSeverityColor, SEVERITY_COLORS } from '../types';
import { formatTemperature } from '@/utils/temperature';

export interface ThresholdBadgeProps {
    /** The metric key (cpu, memory, disk, etc.) */
    metric: string;
    /** The current threshold value */
    value: number | undefined;
    /** The default value (to detect overrides) */
    defaultValue?: number;
    /** Whether this value differs from the default */
    isOverridden?: boolean;
    /** Whether there's an active alert for this metric */
    hasAlert?: boolean;
    /** Click handler for editing */
    onClick?: () => void;
    /** Badge size variant */
    size?: 'sm' | 'md' | 'lg';
    /** Show the metric label */
    showLabel?: boolean;
    /** Custom label text */
    label?: string;
}

/**
 * Format the display value based on metric type
 */
const formatDisplayValue = (metric: string, value: number | undefined): string => {
    if (value === undefined || value === null) return '—';
    if (value <= 0) return 'Off';

    // Percentage metrics
    if (['cpu', 'memory', 'disk', 'usage', 'memoryWarnPct', 'memoryCriticalPct'].includes(metric)) {
        return `${value}%`;
    }

    // Temperature
    if (metric === 'temperature') {
        return formatTemperature(value);
    }

    // Time-based
    if (metric === 'restartWindow') {
        return `${value}s`;
    }

    // Size-based
    if (metric === 'warningSizeGiB' || metric === 'criticalSizeGiB') {
        return `${Math.round(value * 10) / 10} GiB`;
    }

    // MB/s metrics
    if (['diskRead', 'diskWrite', 'networkIn', 'networkOut'].includes(metric)) {
        return `${value}`;
    }

    // Days for backup/snapshot age
    if (metric.includes('days') || metric.includes('Days')) {
        return `${value}d`;
    }

    return String(value);
};


/**
 * Get readable label for a metric
 */
const getMetricLabel = (metric: string): string => {
    const labels: Record<string, string> = {
        cpu: 'CPU',
        memory: 'Mem',
        disk: 'Disk',
        temperature: 'Temp',
        diskRead: 'Read',
        diskWrite: 'Write',
        networkIn: 'In',
        networkOut: 'Out',
        usage: 'Usage',
    };
    return labels[metric] || metric;
};

export const ThresholdBadge: Component<ThresholdBadgeProps> = (props) => {
    const sizeClasses = {
        sm: 'px-1.5 py-0.5 text-xs',
        md: 'px-2 py-1 text-sm',
        lg: 'px-3 py-1.5 text-base',
    };

    const size = () => props.size || 'sm';
    const severity = () => getThresholdSeverityColor(props.value, props.metric);
    const colorClass = () => SEVERITY_COLORS[severity()];
    const isDisabled = () => props.value === undefined || props.value <= 0;

    return (
        <button
            type="button"
            onClick={(e) => {
                e.stopPropagation();
                props.onClick?.();
            }}
            class={`
        inline-flex items-center gap-1 rounded-full font-medium
        transition-all duration-150
        ${sizeClasses[size()]}
        ${colorClass()}
        ${props.onClick ? 'cursor-pointer hover:ring-2 hover:ring-offset-1 hover:ring-blue-400 dark:hover:ring-offset-gray-900' : 'cursor-default'}
        ${props.isOverridden ? 'ring-1 ring-blue-400 dark:ring-blue-500' : ''}
        ${props.hasAlert ? 'animate-pulse ring-2 ring-red-400' : ''}
      `}
            title={
                props.hasAlert
                    ? `Active alert on ${props.metric}`
                    : props.isOverridden
                        ? `Custom threshold (default: ${formatDisplayValue(props.metric, props.defaultValue)})`
                        : `${props.metric} threshold`
            }
        >
            <Show when={props.showLabel}>
                <span class="opacity-70">{props.label || getMetricLabel(props.metric)}</span>
            </Show>
            <span class={isDisabled() ? 'italic' : ''}>
                {formatDisplayValue(props.metric, props.value)}
            </span>
        </button>
    );
};

/**
 * A group of threshold badges for a resource
 */
export interface ThresholdBadgeGroupProps {
    thresholds: Record<string, number | undefined>;
    defaults: Record<string, number | undefined>;
    metrics: string[];
    onClickMetric?: (metric: string) => void;
    hasActiveAlert?: (metric: string) => boolean;
    size?: 'sm' | 'md' | 'lg';
    maxVisible?: number;
}

export const ThresholdBadgeGroup: Component<ThresholdBadgeGroupProps> = (props) => {
    const maxVisible = () => props.maxVisible ?? 4;

    const visibleMetrics = () => {
        const metrics = props.metrics.filter((m) => {
            const value = props.thresholds[m];
            // Show if has a value or is overridden
            return value !== undefined || (props.defaults[m] !== undefined);
        });
        return metrics.slice(0, maxVisible());
    };

    const hiddenCount = () => {
        const total = props.metrics.filter((m) =>
            props.thresholds[m] !== undefined || props.defaults[m] !== undefined
        ).length;
        return Math.max(0, total - maxVisible());
    };

    return (
        <div class="flex flex-wrap items-center gap-1">
            {visibleMetrics().map((metric) => (
                <ThresholdBadge
                    metric={metric}
                    value={props.thresholds[metric] ?? props.defaults[metric]}
                    defaultValue={props.defaults[metric]}
                    isOverridden={
                        props.thresholds[metric] !== undefined &&
                        props.thresholds[metric] !== props.defaults[metric]
                    }
                    hasAlert={props.hasActiveAlert?.(metric)}
                    onClick={props.onClickMetric ? () => props.onClickMetric!(metric) : undefined}
                    size={props.size}
                    showLabel={true}
                />
            ))}
            <Show when={hiddenCount() > 0}>
                <span class="text-xs text-gray-500 dark:text-gray-400">
                    +{hiddenCount()} more
                </span>
            </Show>
        </div>
    );
};

export default ThresholdBadge;
