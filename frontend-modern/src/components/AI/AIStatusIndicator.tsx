/**
 * AIStatusIndicator - Subtle header component showing Pulse patrol health and anomalies
 * 
 * Design: Minimal presence when healthy, highlighted when issues or anomalies detected.
 * Clicking navigates to Pulse Patrol.
 */

import { createResource, Show, createMemo, onCleanup } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { getPatrolStatus, type PatrolStatus } from '../../api/patrol';
import { useAllAnomalies } from '@/hooks/useAnomalies';
import { useLearningStatus } from '@/hooks/useLearningStatus';
import './AIStatusIndicator.css';


export function AIStatusIndicator() {
    const navigate = useNavigate();

    // Poll patrol status every 30 seconds
    const [status, { refetch }] = createResource<PatrolStatus>(
        async () => {
            try {
                return await getPatrolStatus();
            } catch {
                return null as unknown as PatrolStatus;
            }
        },
        { initialValue: undefined }
    );

    // Get anomaly data (also polls every 30 seconds via the hook)
    const anomalyData = useAllAnomalies();

    // Get learning status (polls every 60 seconds)
    const learningStatus = useLearningStatus();

    // Refetch patrol status every 30 seconds with proper cleanup
    const intervalId = setInterval(() => refetch(), 30000);
    onCleanup(() => clearInterval(intervalId));


    // Count anomalies by severity
    const anomalyCounts = createMemo(() => {
        const anomalies = anomalyData.anomalies();
        const counts = { critical: 0, high: 0, medium: 0, low: 0 };
        for (const a of anomalies) {
            counts[a.severity]++;
        }
        return counts;
    });

    const totalAnomalies = createMemo(() => anomalyData.count());

    const hasIssues = createMemo(() => {
        const s = status();
        if (!s) return false;
        return s.summary.critical > 0 || s.summary.warning > 0;
    });

    const hasWatch = createMemo(() => {
        const s = status();
        if (!s) return false;
        return s.summary.watch > 0 && !hasIssues();
    });

    const hasAnomalies = createMemo(() => {
        const counts = anomalyCounts();
        return counts.critical > 0 || counts.high > 0;
    });

    const hasMildAnomalies = createMemo(() => {
        const counts = anomalyCounts();
        return !hasAnomalies() && (counts.medium > 0 || counts.low > 0);
    });

    const totalFindings = createMemo(() => {
        const s = status();
        if (!s) return 0;
        return s.summary.critical + s.summary.warning + s.summary.watch;
    });

    const tooltipText = createMemo(() => {
        const parts: string[] = [];

        // Patrol status
        const s = status();
        if (s?.enabled && s?.running) {
            if (s.summary.critical > 0) parts.push(`${s.summary.critical} critical findings`);
            if (s.summary.warning > 0) parts.push(`${s.summary.warning} warnings`);
            if (s.summary.watch > 0) parts.push(`${s.summary.watch} watching`);
        }

        // Anomaly status
        const counts = anomalyCounts();
        const anomalyTotal = totalAnomalies();
        if (anomalyTotal > 0) {
            const anomalyParts: string[] = [];
            if (counts.critical > 0) anomalyParts.push(`${counts.critical} critical`);
            if (counts.high > 0) anomalyParts.push(`${counts.high} high`);
            if (counts.medium > 0) anomalyParts.push(`${counts.medium} medium`);
            if (counts.low > 0) anomalyParts.push(`${counts.low} low`);
            parts.push(`Anomalies: ${anomalyParts.join(', ')}`);
        }

        // Learning status - show when healthy to indicate AI is working
        const resourceCount = learningStatus.resourceCount();
        if (parts.length === 0 && resourceCount > 0) {
            // Show learning progress when healthy
            return `Pulse: All healthy • ${resourceCount} resources baselined`;
        }

        if (parts.length === 0) {
            if (!s?.enabled) {
                // Show baseline info even when patrol disabled
                if (resourceCount > 0) {
                    return `Pulse Learning: ${resourceCount} resources baselined`;
                }
                return 'Pulse Baseline Learning active';
            }
            if (s?.license_required) {
                if (s.license_status === 'active') {
                    return 'Pulse Patrol is not included in this license tier';
                }
                if (s.license_status === 'expired') {
                    return 'Pulse Patrol license expired';
                }
                return 'Pulse Patrol requires Pulse Pro';
            }
            return 'Pulse: All systems healthy';
        }

        return `Pulse Patrol: ${parts.join(' | ')}`;
    });


    const statusClass = createMemo(() => {
        if (hasIssues() || hasAnomalies()) return 'ai-status--issues';
        if (hasWatch() || hasMildAnomalies()) return 'ai-status--watch';
        return 'ai-status--healthy';
    });

    const handleClick = () => {
        navigate('/ai');
    };

    // Combined total for badge
    const badgeCount = createMemo(() => totalFindings() + totalAnomalies());

    // Show indicator if patrol is enabled, we have anomalies, OR we have learned baselines
    // This makes the AI presence visible even before Patrol is configured
    const showIndicator = createMemo(() => {
        const s = status();
        const hasBaselines = learningStatus.resourceCount() > 0;
        return s?.enabled || totalAnomalies() > 0 || hasBaselines;
    });

    return (
        <Show when={showIndicator()}>
            <button
                class={`ai-status-indicator ${statusClass()}`}
                onClick={handleClick}
                title={tooltipText()}
            >
                <span class="ai-status-icon">
                    <Show when={hasIssues() || hasAnomalies()} fallback={
                        <Show when={hasWatch() || hasMildAnomalies()} fallback={
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <path d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10z" />
                                <path d="M9 12l2 2 4-4" />
                            </svg>
                        }>
                            {/* Watch icon - eye */}
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                                <circle cx="12" cy="12" r="3" />
                            </svg>
                        </Show>
                    }>
                        {/* Issues icon - alert */}
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                            <line x1="12" y1="9" x2="12" y2="13" />
                            <line x1="12" y1="17" x2="12.01" y2="17" />
                        </svg>
                    </Show>
                </span>
                <Show when={badgeCount() > 0}>
                    <span class="ai-status-count">{badgeCount()}</span>
                </Show>
            </button>
        </Show>
    );
}

export default AIStatusIndicator;
