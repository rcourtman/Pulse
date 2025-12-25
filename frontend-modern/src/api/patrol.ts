/**
 * AI Patrol API client
 * Provides access to background AI monitoring findings and status
 */

import { apiFetchJSON } from '@/utils/apiClient';
import { logger } from '@/utils/logger';

export type FindingSeverity = 'info' | 'watch' | 'warning' | 'critical';
export type FindingCategory = 'performance' | 'capacity' | 'reliability' | 'backup' | 'security' | 'general';

export interface Finding {
    id: string;
    key?: string;
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
    // User feedback fields (LLM memory system)
    dismissed_reason?: 'not_an_issue' | 'expected_behavior' | 'will_fix_later';
    user_note?: string;
    times_raised: number;
    suppressed: boolean;
}

export interface RunbookInfo {
    id: string;
    title: string;
    description: string;
    risk: 'low' | 'medium' | 'high';
}

export interface RunbookStepResult {
    name: string;
    command: string;
    output: string;
    success: boolean;
}

export interface RunbookExecutionResult {
    runbook_id: string;
    outcome: 'resolved' | 'partial' | 'failed' | 'unknown';
    message: string;
    steps: RunbookStepResult[];
    verification?: RunbookStepResult;
    resolved: boolean;
    executed_at: string;
    finding_id: string;
    finding_key?: string;
}

export interface FindingsSummary {
    critical: number;
    warning: number;
    watch: number;
    info: number;
}

export type LicenseStatus = 'none' | 'active' | 'expired' | 'grace_period';

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
    license_required?: boolean;
    license_status?: LicenseStatus;
    upgrade_url?: string;
    summary: FindingsSummary;
}

export interface PatrolRunRecord {
    id: string;
    started_at: string;
    completed_at: string;
    duration_ms: number;
    type: string; // Always 'patrol' now (kept for backwards compat with old records)
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
    auto_fix_count?: number;
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
    return apiFetchJSON<PatrolStatus>('/api/ai/patrol/status');
}

/**
 * Get all active findings from the patrol service
 * Optionally filter by resource ID
 */
export async function getFindings(resourceId?: string): Promise<Finding[]> {
    const url = resourceId
        ? `/api/ai/patrol/findings?resource_id=${encodeURIComponent(resourceId)}`
        : '/api/ai/patrol/findings';

    return apiFetchJSON<Finding[]>(url);
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
    return apiFetchJSON<Finding[]>(url);
}

/**
 * Get the history of patrol check runs
 * @param limit Maximum number of records to return (default: 50, max: 100)
 */
export async function getPatrolRunHistory(limit?: number): Promise<PatrolRunRecord[]> {
    const url = limit
        ? `/api/ai/patrol/runs?limit=${limit}`
        : '/api/ai/patrol/runs';
    return apiFetchJSON<PatrolRunRecord[]>(url);
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

export async function getRunbooksForFinding(findingId: string): Promise<RunbookInfo[]> {
    const url = `/api/ai/runbooks?finding_id=${encodeURIComponent(findingId)}`;
    return apiFetchJSON<RunbookInfo[]>(url);
}

export async function executeRunbook(findingId: string, runbookId: string): Promise<RunbookExecutionResult> {
    return apiFetchJSON('/api/ai/runbooks/execute', {
        method: 'POST',
        body: JSON.stringify({ finding_id: findingId, runbook_id: runbookId }),
    });
}

/**
 * Dismiss a finding with a reason (LLM memory feature)
 * The LLM will be told not to re-raise this issue in future patrols.
 * @param findingId The ID of the finding to dismiss
 * @param reason One of: "not_an_issue", "expected_behavior", "will_fix_later"
 * @param note Optional freeform explanation
 */
export async function dismissFinding(
    findingId: string,
    reason: 'not_an_issue' | 'expected_behavior' | 'will_fix_later',
    note?: string
): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON('/api/ai/patrol/dismiss', {
        method: 'POST',
        body: JSON.stringify({ finding_id: findingId, reason, note }),
    });
}

/**
 * Permanently suppress a finding type (LLM memory feature)
 * The LLM will never re-raise this type of finding for this resource.
 * @param findingId The ID of the finding to suppress
 */
export async function suppressFinding(findingId: string): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON('/api/ai/patrol/suppress', {
        method: 'POST',
        body: JSON.stringify({ finding_id: findingId }),
    });
}

/**
 * Clear all AI findings
 * Removes all accumulated findings from the store.
 * Useful for users who want to start fresh or who accumulated findings
 * before AI was properly configured.
 */
export async function clearAllFindings(): Promise<{ success: boolean; cleared: number; message: string }> {
    return apiFetchJSON('/api/ai/patrol/findings?confirm=true', {
        method: 'DELETE',
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
            logger.error('Failed to parse patrol stream event:', e);
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

// === Suppression Rules ===

export interface SuppressionRule {
    id: string;
    resource_id?: string;   // Empty means "any resource"
    resource_name?: string; // Human-readable name
    category?: FindingCategory; // Empty means "any category"
    description: string;    // User's reason
    created_at: string;
    created_from: 'finding' | 'manual';
    finding_id?: string;    // Original finding ID if created from dismissal
}

/**
 * Get all suppression rules (both manual and from dismissed findings)
 */
export async function getSuppressionRules(): Promise<SuppressionRule[]> {
    return apiFetchJSON<SuppressionRule[]>('/api/ai/patrol/suppressions');
}

/**
 * Create a new manual suppression rule
 * @param resourceId Resource ID (empty for "any resource")
 * @param resourceName Human-readable name for display
 * @param category Category (empty for "any category")
 * @param description User's reason for the rule
 */
export async function addSuppressionRule(
    resourceId: string,
    resourceName: string,
    category: FindingCategory | '',
    description: string
): Promise<{ success: boolean; message: string; rule: SuppressionRule }> {
    return apiFetchJSON('/api/ai/patrol/suppressions', {
        method: 'POST',
        body: JSON.stringify({
            resource_id: resourceId,
            resource_name: resourceName,
            category: category,
            description: description,
        }),
    });
}

/**
 * Delete a suppression rule
 * @param ruleId The ID of the rule to delete
 */
export async function deleteSuppressionRule(ruleId: string): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON(`/api/ai/patrol/suppressions/${encodeURIComponent(ruleId)}`, {
        method: 'DELETE',
    });
}

/**
 * Get all dismissed/suppressed findings
 */
export async function getDismissedFindings(): Promise<Finding[]> {
    return apiFetchJSON<Finding[]>('/api/ai/patrol/dismissed');
}
