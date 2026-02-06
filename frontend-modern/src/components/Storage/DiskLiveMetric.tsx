import { createMemo, createEffect, onCleanup, createSignal } from 'solid-js';
import { getLatestDiskMetric, getDiskMetricsVersion } from '@/stores/diskMetricsHistory';
import { formatBytes } from '@/utils/format';

interface DiskLiveMetricProps {
    resourceId: string;
    type: 'read' | 'write' | 'ioTime';
}

export function DiskLiveMetric(props: DiskLiveMetricProps) {
    const [version, setVersion] = createSignal(getDiskMetricsVersion());

    const updateHandler = () => setVersion(getDiskMetricsVersion());

    const latestMetric = createMemo(() => {
        version(); // dependency
        return getLatestDiskMetric(props.resourceId);
    });

    // Poll for UI updates (keep consistent with sparkline timing)
    createEffect(() => {
        const timer = setInterval(updateHandler, 2000);
        onCleanup(() => clearInterval(timer));
    });

    const value = createMemo(() => {
        const m = latestMetric();
        if (!m) return 0;

        if (props.type === 'read') return m.readBps;
        if (props.type === 'write') return m.writeBps;
        return m.ioTime;
    });

    const formatted = createMemo(() => {
        const v = value();
        if (props.type === 'ioTime') return `${Math.round(v)}%`;
        return `${formatBytes(v)}/s`;
    });

    const colorClass = createMemo(() => {
        const v = value();
        // Highlight interesting values
        if (props.type === 'ioTime') {
            if (v > 90) return 'text-red-600 dark:text-red-400 font-bold';
            if (v > 50) return 'text-yellow-600 dark:text-yellow-400 font-semibold';
        } else {
            // > 100MB/s
            if (v > 100 * 1024 * 1024) return 'text-blue-600 dark:text-blue-400 font-semibold';
        }
        return 'text-gray-600 dark:text-gray-400';
    });

    return (
        <span class={`font-mono text-[11px] sm:text-xs ${colorClass()}`}>
            {formatted()}
        </span>
    );
}
