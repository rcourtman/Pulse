/**
 * AI Patrol API client
 * Provides access to background AI monitoring findings and status
 */

import { apiFetchJSON } from '@/utils/apiClient';

export type FindingSeverity = 'info' | 'watch' | 'warning' | 'critical';
export type FindingCategory = 'performance' | 'capacity' | 'reliability' | 'backup' | 'security' | 'general';

export interface Finding {
    id: string;
    severity: FindingSeverity;
    category: FindingCategory;
    resource_id: string;
    resource_name: string;
    resource_type: string; // node, vm, container, docker_host, docker_container, storage, pbs, host_raid
    node?: string;
    title: string;
    description: string;
    recommendation?: string;
    evidence?: string;
    detected_at: string;
    last_seen_at: string;
    resolved_at?: string;
    auto_resolved: boolean;
    acknowledged_at?: string;
    snoozed_until?: string; // Finding hidden until this time
    alert_id?: string;
}

export interface FindingsSummary {
    critical: number;
    warning: number;
    watch: number;
    info: number;
}

export interface PatrolStatus {
    running: boolean;
    enabled: boolean;
    last_patrol_at?: string;
    last_deep_analysis_at?: string;
    next_patrol_at?: string;
    last_duration_ms: number;
    resources_checked: number;
    findings_count: number;
    error_count: number;
    healthy: boolean;
    interval_ms: number; // Patrol interval in milliseconds
    summary: FindingsSummary;
}

export interface PatrolRunRecord {
    id: string;
    started_at: string;
    completed_at: string;
    duration_ms: number;
    type: 'quick' | 'deep';
    resources_checked: number;
    // Breakdown by resource type
    nodes_checked: number;
    guests_checked: number;
    docker_checked: number;
    storage_checked: number;
    hosts_checked: number;
    pbs_checked: number;
    // Findings from this run
    new_findings: number;
    existing_findings: number;
    resolved_findings: number;
    findings_summary: string;
    finding_ids: string[];
    error_count: number;
    status: 'healthy' | 'issues_found' | 'critical' | 'error';
    // AI Analysis details
    ai_analysis?: string;    // The AI's raw response/analysis
    input_tokens?: number;   // Tokens sent to AI
    output_tokens?: number;  // Tokens received from AI
}

/**
 * Get the current AI patrol status
 */
export async function getPatrolStatus(): Promise<PatrolStatus> {
    const resp = await fetch('/api/ai/patrol/status', {
        credentials: 'include',
    });
    if (!resp.ok) {
        throw new Error(`Failed to get patrol status: ${resp.status}`);
    }
    return resp.json();
}

/**
 * Get all active findings from the patrol service
 * Optionally filter by resource ID
 */
export async function getFindings(resourceId?: string): Promise<Finding[]> {
    const url = resourceId
        ? `/api/ai/patrol/findings?resource_id=${encodeURIComponent(resourceId)}`
        : '/api/ai/patrol/findings';

    const resp = await fetch(url, {
        credentials: 'include',
    });
    if (!resp.ok) {
        throw new Error(`Failed to get findings: ${resp.status}`);
    }
    return resp.json();
}

/**
 * Trigger an immediate patrol run
 */
export async function forcePatrol(deep: boolean = false): Promise<{ success: boolean; message: string }> {
    const url = deep ? '/api/ai/patrol/run?deep=true' : '/api/ai/patrol/run';
    return apiFetchJSON(url, { method: 'POST' });
}

/**
 * Get AI findings history including resolved findings
 * @param startTime Optional ISO timestamp to filter findings from
 */
export async function getFindingsHistory(startTime?: string): Promise<Finding[]> {
    const url = startTime
        ? `/api/ai/patrol/history?start_time=${encodeURIComponent(startTime)}`
        : '/api/ai/patrol/history';
    const resp = await fetch(url, {
        method: 'GET',
        credentials: 'include',
    });
    if (!resp.ok) {
        throw new Error(`Failed to get findings history: ${resp.status}`);
    }
    return resp.json();
}

/**
 * Get the history of patrol check runs
 * @param limit Maximum number of records to return (default: 50, max: 100)
 */
export async function getPatrolRunHistory(limit?: number): Promise<PatrolRunRecord[]> {
    const url = limit
        ? `/api/ai/patrol/runs?limit=${limit}`
        : '/api/ai/patrol/runs';
    const resp = await fetch(url, {
        method: 'GET',
        credentials: 'include',
    });
    if (!resp.ok) {
        throw new Error(`Failed to get patrol run history: ${resp.status}`);
    }
    return resp.json();
}

/**
 * Acknowledge a finding (marks as seen but keeps visible, like alert acknowledgement)
 * Finding will auto-resolve when the underlying condition clears.
 */
export async function acknowledgeFinding(findingId: string): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON('/api/ai/patrol/acknowledge', {
        method: 'POST',
        body: JSON.stringify({ finding_id: findingId }),
    });
}

/**
 * Snooze a finding for a specified duration
 * @param findingId The ID of the finding to snooze
 * @param durationHours Duration in hours (e.g., 1, 24, 168 for 7 days)
 */
export async function snoozeFinding(findingId: string, durationHours: number): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON('/api/ai/patrol/snooze', {
        method: 'POST',
        body: JSON.stringify({ finding_id: findingId, duration_hours: durationHours }),
    });
}

/**
 * Manually resolve a finding (mark as fixed)
 * @param findingId The ID of the finding to resolve
 */
export async function resolveFinding(findingId: string): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON('/api/ai/patrol/resolve', {
        method: 'POST',
        body: JSON.stringify({ finding_id: findingId }),
    });
}

/**
 * Severity color mapping for UI
 */
export const severityColors: Record<FindingSeverity, { bg: string; text: string; border: string }> = {
    critical: { bg: 'rgba(220, 38, 38, 0.15)', text: '#ef4444', border: 'rgba(220, 38, 38, 0.3)' },
    warning: { bg: 'rgba(234, 179, 8, 0.15)', text: '#eab308', border: 'rgba(234, 179, 8, 0.3)' },
    watch: { bg: 'rgba(59, 130, 246, 0.15)', text: '#3b82f6', border: 'rgba(59, 130, 246, 0.3)' },
    info: { bg: 'rgba(107, 114, 128, 0.15)', text: '#9ca3af', border: 'rgba(107, 114, 128, 0.3)' },
};

/**
 * Category labels for UI
 */
export const categoryLabels: Record<FindingCategory, string> = {
    performance: 'Performance',
    capacity: 'Capacity',
    reliability: 'Reliability',
    backup: 'Backup',
    security: 'Security',
    general: 'General',
};

/**
 * Format a timestamp for display
 */
export function formatTimestamp(ts: string): string {
    const date = new Date(ts);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;
    return date.toLocaleDateString();
}

/**
 * Event types from patrol stream
 */
export interface PatrolStreamEvent {
    type: 'start' | 'content' | 'thinking' | 'phase' | 'complete' | 'error';
    content?: string;
    phase?: string;
    tokens?: number;
}

/**
 * Subscribe to live patrol stream via SSE
 * Returns an unsubscribe function
 */
export function subscribeToPatrolStream(
    onEvent: (event: PatrolStreamEvent) => void,
    onError?: (error: Error) => void
): () => void {
    const eventSource = new EventSource('/api/ai/patrol/stream', { withCredentials: true });

    eventSource.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data) as PatrolStreamEvent;
            onEvent(data);
        } catch (e) {
            console.error('Failed to parse patrol stream event:', e);
        }
    };

    eventSource.onerror = () => {
        if (onError) {
            onError(new Error('Patrol stream connection error'));
        }
        eventSource.close();
    };

    // Return unsubscribe function
    return () => {
        eventSource.close();
    };
}
