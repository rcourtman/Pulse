import { Show, createMemo } from 'solid-js';
import type { AnomalyReport, AnomalySeverity } from '@/types/aiIntelligence';

interface AnomalyIndicatorProps {
    anomaly: AnomalyReport | null;
    compact?: boolean;  // Only show icon, no text
}

/**
 * Displays an anomaly indicator badge for a metric.
 * Shows severity level and how much above/below baseline the current value is.
 */
export function AnomalyIndicator(props: AnomalyIndicatorProps) {
    const severityConfig = createMemo(() => {
        if (!props.anomaly) return null;

        const configs: Record<AnomalySeverity, { bg: string; text: string; icon: string }> = {
            critical: {
                bg: 'bg-red-500',
                text: 'text-white',
                icon: '🔴',
            },
            high: {
                bg: 'bg-orange-500',
                text: 'text-white',
                icon: '🟠',
            },
            medium: {
                bg: 'bg-yellow-500',
                text: 'text-gray-800',
                icon: '🟡',
            },
            low: {
                bg: 'bg-blue-400',
                text: 'text-white',
                icon: '🔵',
            },
        };

        return configs[props.anomaly.severity] || configs.medium;
    });

    // Calculate how many times above baseline
    const multiplier = createMemo(() => {
        if (!props.anomaly || props.anomaly.baseline_mean === 0) return null;
        const ratio = props.anomaly.current_value / props.anomaly.baseline_mean;
        if (ratio >= 2) {
            return `${ratio.toFixed(1)}x`;
        }
        return null;
    });

    // Simplified label for compact display
    const compactLabel = createMemo(() => {
        const m = multiplier();
        if (m) return m;
        if (props.anomaly) {
            const zAbs = Math.abs(props.anomaly.z_score);
            if (zAbs >= 4) return 'CRIT';
            if (zAbs >= 3) return 'HIGH';
            if (zAbs >= 2.5) return 'MED';
            return 'LOW';
        }
        return '';
    });

    return (
        <Show when={props.anomaly && severityConfig()}>
            <div
                class={`inline-flex items-center gap-0.5 px-1 py-0.5 rounded text-[9px] font-bold ${severityConfig()!.bg
                    } ${severityConfig()!.text} animate-pulse`}
                title={props.anomaly!.description}
            >
                <Show when={!props.compact}>
                    <span>{severityConfig()!.icon}</span>
                </Show>
                <span>{compactLabel()}</span>
            </div>
        </Show>
    );
}

/**
 * Small inline indicator for use within metric bars.
 * Just shows an icon and optionally the multiplier.
 */
export function AnomalyBadge(props: { anomaly: AnomalyReport | null }) {
    const multiplier = createMemo(() => {
        if (!props.anomaly || props.anomaly.baseline_mean === 0) return null;
        const ratio = props.anomaly.current_value / props.anomaly.baseline_mean;
        if (ratio >= 1.5) {
            return `${ratio.toFixed(1)}x`;
        }
        return null;
    });

    const severityColor = createMemo(() => {
        if (!props.anomaly) return '';
        switch (props.anomaly.severity) {
            case 'critical':
                return 'text-red-400';
            case 'high':
                return 'text-orange-400';
            case 'medium':
                return 'text-yellow-400';
            case 'low':
                return 'text-blue-400';
            default:
                return 'text-gray-400';
        }
    });

    return (
        <Show when={props.anomaly}>
            <span
                class={`ml-1 ${severityColor()} font-bold text-[9px] animate-pulse`}
                title={props.anomaly!.description}
            >
                <Show when={multiplier()} fallback="⚠">
                    {multiplier()}↑
                </Show>
            </span>
        </Show>
    );
}
