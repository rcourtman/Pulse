import { Show, createMemo, For } from 'solid-js';
import type { AnomalyReport, AnomalySeverity } from '@/types/aiIntelligence';

interface AnomalyCellProps {
    anomalies: AnomalyReport[];
}

// Severity priority for sorting (higher = more severe)
const severityPriority: Record<AnomalySeverity, number> = {
    critical: 4,
    high: 3,
    medium: 2,
    low: 1,
};

// Config for each severity level
const severityConfig: Record<AnomalySeverity, { bg: string; text: string; border: string }> = {
    critical: {
        bg: 'bg-red-100 dark:bg-red-900/40',
        text: 'text-red-700 dark:text-red-300',
        border: 'border-red-300 dark:border-red-700',
    },
    high: {
        bg: 'bg-orange-100 dark:bg-orange-900/40',
        text: 'text-orange-700 dark:text-orange-300',
        border: 'border-orange-300 dark:border-orange-700',
    },
    medium: {
        bg: 'bg-yellow-100 dark:bg-yellow-900/40',
        text: 'text-yellow-700 dark:text-yellow-300',
        border: 'border-yellow-300 dark:border-yellow-700',
    },
    low: {
        bg: 'bg-blue-100 dark:bg-blue-900/40',
        text: 'text-blue-700 dark:text-blue-300',
        border: 'border-blue-300 dark:border-blue-700',
    },
};

/**
 * Cell component that displays anomaly indicators for a guest.
 * Shows metric badges with severity colors and deviation info.
 */
export function AnomalyCell(props: AnomalyCellProps) {
    // Sort by severity (most severe first)
    const sortedAnomalies = createMemo(() =>
        [...props.anomalies].sort(
            (a, b) => severityPriority[b.severity] - severityPriority[a.severity]
        )
    );

    // Format ratio for display
    const formatRatio = (anomaly: AnomalyReport): string => {
        if (anomaly.baseline_mean === 0) return '↑';
        const ratio = anomaly.current_value / anomaly.baseline_mean;
        if (ratio >= 2) return `${ratio.toFixed(1)}x`;
        if (ratio >= 1.5) return '↑↑';
        if (ratio > 1) return '↑';
        if (ratio <= 0.5) return '↓↓';
        return '↓';
    };

    // Metric labels
    const metricLabels: Record<string, string> = {
        cpu: 'CPU',
        memory: 'MEM',
        disk: 'DSK',
    };

    return (
        <Show when={props.anomalies.length > 0}>
            <div class="flex items-center gap-0.5 flex-wrap justify-center">
                <For each={sortedAnomalies().slice(0, 3)}>
                    {(anomaly) => {
                        const config = severityConfig[anomaly.severity];
                        return (
                            <span
                                class={`inline-flex items-center px-1 py-0.5 rounded text-[8px] font-bold ${config.bg} ${config.text} border ${config.border}`}
                                title={anomaly.description}
                            >
                                <span>{metricLabels[anomaly.metric] || anomaly.metric.toUpperCase()}</span>
                                <span class="ml-0.5 opacity-75">{formatRatio(anomaly)}</span>
                            </span>
                        );
                    }}
                </For>
                <Show when={sortedAnomalies().length > 3}>
                    <span
                        class="text-[8px] text-gray-500 dark:text-gray-400"
                        title={`${sortedAnomalies().length - 3} more anomalies`}
                    >
                        +{sortedAnomalies().length - 3}
                    </span>
                </Show>
            </div>
        </Show>
    );
}

/**
 * Small dot indicator showing if a guest has any anomalies.
 * Used for compact views where space is limited.
 */
export function AnomalyDot(props: { hasAnomalies: boolean; severity?: AnomalySeverity; title?: string }) {
    const dotColor = createMemo(() => {
        if (!props.hasAnomalies) return '';
        switch (props.severity) {
            case 'critical':
                return 'bg-red-500';
            case 'high':
                return 'bg-orange-500';
            case 'medium':
                return 'bg-yellow-500';
            case 'low':
                return 'bg-blue-400';
            default:
                return 'bg-gray-400';
        }
    });

    return (
        <Show when={props.hasAnomalies}>
            <span
                class={`inline-block w-1.5 h-1.5 rounded-full ${dotColor()} animate-pulse`}
                title={props.title || 'Baseline anomaly detected'}
            />
        </Show>
    );
}
