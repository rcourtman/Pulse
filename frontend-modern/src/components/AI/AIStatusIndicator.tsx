/**
 * AIStatusIndicator - Subtle header component showing AI patrol health
 * 
 * Design: Minimal presence when healthy, highlighted when issues detected.
 * Clicking navigates to the Alerts page where AI Insights are displayed.
 */

import { createResource, Show, createMemo, onCleanup } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { getPatrolStatus, type PatrolStatus } from '../../api/patrol';
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

    // Refetch every 30 seconds with proper cleanup
    const intervalId = setInterval(() => refetch(), 30000);
    onCleanup(() => clearInterval(intervalId));

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

    const totalFindings = createMemo(() => {
        const s = status();
        if (!s) return 0;
        return s.summary.critical + s.summary.warning + s.summary.watch;
    });

    const tooltipText = createMemo(() => {
        const s = status();
        if (!s) return 'AI Patrol status unavailable';
        if (!s.enabled) return 'AI Patrol disabled';
        if (s.license_required) {
            if (s.license_status === 'active') {
                return 'AI Patrol is not included in this license tier';
            }
            if (s.license_status === 'expired') {
                return 'AI Patrol license expired - upgrade to restore';
            }
            return 'AI Patrol requires Pulse Pro';
        }
        if (!s.running) return 'AI Patrol not running';

        const parts: string[] = [];
        if (s.summary.critical > 0) parts.push(`${s.summary.critical} critical`);
        if (s.summary.warning > 0) parts.push(`${s.summary.warning} warning`);
        if (s.summary.watch > 0) parts.push(`${s.summary.watch} watch`);

        if (parts.length === 0) return 'AI: All systems healthy';
        return `AI: ${parts.join(', ')}`;
    });

    const statusClass = createMemo(() => {
        if (hasIssues()) return 'ai-status--issues';
        if (hasWatch()) return 'ai-status--watch';
        return 'ai-status--healthy';
    });

    const handleClick = () => {
        // Navigate to Alerts page with AI Insights subtab selected
        navigate('/alerts?subtab=ai-insights');
    };

    return (
        <Show when={status()?.enabled}>
            <button
                class={`ai-status-indicator ${statusClass()}`}
                onClick={handleClick}
                title={tooltipText()}
            >
                <span class="ai-status-icon">
                    <Show when={hasIssues()} fallback={
                        <Show when={hasWatch()} fallback={
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
                <Show when={totalFindings() > 0}>
                    <span class="ai-status-count">{totalFindings()}</span>
                </Show>
            </button>
        </Show>
    );
}

export default AIStatusIndicator;
