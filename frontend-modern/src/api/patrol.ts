/**
 * Pulse Patrol API client
 * Provides access to background AI monitoring findings and status
 */

import { apiFetchJSON } from '@/utils/apiClient';

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
    // Investigation fields (Patrol Autonomy)
    investigation_session_id?: string;
    investigation_status?: InvestigationStatus;
    investigation_outcome?: InvestigationOutcome;
    last_investigated_at?: string;
    investigation_attempts: number;
}

export type InvestigationStatus = 'pending' | 'running' | 'completed' | 'failed' | 'needs_attention';
export type InvestigationOutcome =
    | 'resolved'
    | 'fix_queued'
    | 'fix_executed'
    | 'fix_failed'
    | 'needs_attention'
    | 'cannot_fix'
    | 'timed_out'
    | 'fix_verified'
    | 'fix_verification_failed'
    | 'fix_verification_unknown';
export type PatrolAutonomyLevel = 'monitor' | 'approval' | 'assisted' | 'full';

export interface PatrolAutonomySettings {
    autonomy_level: PatrolAutonomyLevel;
    full_mode_unlocked: boolean;       // User has acknowledged Full mode risks
    investigation_budget: number;      // Max turns per investigation (5-30)
    investigation_timeout_sec: number; // Max seconds per investigation (60-600)
}

export interface Investigation {
    id: string;
    finding_id: string;
    session_id: string;
    status: InvestigationStatus;
    started_at: string;
    completed_at?: string;
    turn_count: number;
    outcome?: InvestigationOutcome;
    tools_available?: string[];
    tools_used?: string[];
    evidence_ids?: string[];
    summary?: string;
    error?: string;
    proposed_fix?: ProposedFix;
    approval_id?: string;
}

export interface ProposedFix {
    id: string;
    description: string;
    commands?: string[];
    risk_level?: string;
    destructive: boolean;
    target_host?: string;
    rationale?: string;
}

export interface InvestigationMessages {
    investigation_id: string;
    session_id: string;
    messages: ChatMessage[];
}

export interface ChatMessage {
    id: string;
    role: 'user' | 'assistant' | 'system';
    content: string;
    reasoning_content?: string;
    tool_calls?: ChatToolCall[];
    tool_result?: ChatToolResult;
    timestamp: string;
}

export interface ChatToolCall {
    id: string;
    name: string;
    input: Record<string, unknown>;
}

export interface ChatToolResult {
    tool_use_id: string;
    content: string;
    is_error?: boolean;
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
    next_patrol_at?: string;
    last_duration_ms: number;
    resources_checked: number;
    findings_count: number;
    error_count: number;
    healthy: boolean;
    interval_ms: number; // Patrol interval in milliseconds
    fixed_count: number; // Number of issues auto-fixed by Patrol
    blocked_reason?: string;
    blocked_at?: string;
    license_required?: boolean;
    license_status?: LicenseStatus;
    upgrade_url?: string;
    summary: FindingsSummary;
}

/**
 * Get the current Pulse Patrol status
 */
export async function getPatrolStatus(): Promise<PatrolStatus> {
    return apiFetchJSON<PatrolStatus>('/api/ai/patrol/status');
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
 * Set or update a user note on a finding
 * Notes provide context that Patrol sees on future runs.
 * @param findingId The ID of the finding
 * @param note The note text (empty string to clear)
 */
export async function setFindingNote(findingId: string, note: string): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON('/api/ai/patrol/findings/note', {
        method: 'POST',
        body: JSON.stringify({ finding_id: findingId, note }),
    });
}

// =============================================================================
// Patrol Autonomy APIs
// =============================================================================

/**
 * Get current patrol autonomy settings
 */
export async function getPatrolAutonomySettings(): Promise<PatrolAutonomySettings> {
    return apiFetchJSON<PatrolAutonomySettings>('/api/ai/patrol/autonomy');
}

/**
 * Update patrol autonomy settings
 */
export async function updatePatrolAutonomySettings(settings: PatrolAutonomySettings): Promise<{ success: boolean; settings: PatrolAutonomySettings }> {
    return apiFetchJSON('/api/ai/patrol/autonomy', {
        method: 'PUT',
        body: JSON.stringify(settings),
    });
}

/**
 * Get investigation details for a finding
 */
export async function getInvestigation(findingId: string): Promise<Investigation> {
    return apiFetchJSON<Investigation>(`/api/ai/findings/${encodeURIComponent(findingId)}/investigation`);
}

/**
 * Get chat messages for an investigation
 */
export async function getInvestigationMessages(findingId: string): Promise<InvestigationMessages> {
    return apiFetchJSON<InvestigationMessages>(`/api/ai/findings/${encodeURIComponent(findingId)}/investigation/messages`);
}

/**
 * Trigger re-investigation of a finding
 */
export async function reinvestigateFinding(findingId: string): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON(`/api/ai/findings/${encodeURIComponent(findingId)}/reinvestigate`, {
        method: 'POST',
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
 * Format a timestamp for display
 */
export function formatTimestamp(ts: string): string {
    const date = new Date(ts);
    if (!ts || Number.isNaN(date.getTime())) {
        return '';
    }
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
 * Investigation status labels for UI
 */
export const investigationStatusLabels: Record<InvestigationStatus, string> = {
    pending: 'Pending',
    running: 'Investigating...',
    completed: 'Completed',
    failed: 'Failed',
    needs_attention: 'Needs Attention',
};

/**
 * Investigation outcome labels for UI
 */
export const investigationOutcomeLabels: Record<InvestigationOutcome, string> = {
    resolved: 'Resolved',
    fix_queued: 'Fix Queued',
    fix_executed: 'Fix Executed',
    fix_failed: 'Fix Failed',
    needs_attention: 'Needs Attention',
    cannot_fix: 'Cannot Auto-Fix',
    timed_out: 'Timed Out — Will Retry',
    fix_verified: 'Fix Verified',
    fix_verification_failed: 'Verification Failed',
    fix_verification_unknown: 'Verification Inconclusive',
};

/**
 * Investigation outcome badge colors for UI
 */
export const investigationOutcomeColors: Record<InvestigationOutcome, string> = {
    resolved: 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
    fix_queued: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900 dark:text-blue-300',
    fix_executed: 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
    fix_failed: 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
    needs_attention: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
    cannot_fix: 'border-slate-200 bg-slate-50 text-slate-600',
    timed_out: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
    fix_verified: 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900 dark:text-green-300',
    fix_verification_failed: 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300',
    fix_verification_unknown: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-300',
};

// =============================================================================
// Patrol Run History APIs
// =============================================================================

export type PatrolRunStatus = 'healthy' | 'issues_found' | 'critical' | 'error';

export interface ToolCallRecord {
    id: string;
    tool_name: string;
    input: string;
    output: string;
    success: boolean;
    start_time: number;
    end_time: number;
    duration_ms: number;
}

export interface PatrolRunRecord {
    id: string;
    started_at: string;
    completed_at: string;
    duration_ms: number;
    type: string;
    trigger_reason?: string;
    scope_resource_ids?: string[];
    scope_resource_types?: string[];
    scope_context?: string;
    alert_id?: string;
    finding_id?: string;
    resources_checked: number;
    nodes_checked: number;
    guests_checked: number;
    docker_checked: number;
    storage_checked: number;
    hosts_checked: number;
    pbs_checked: number;
    pmg_checked: number;
    kubernetes_checked: number;
    new_findings: number;
    existing_findings: number;
    rejected_findings: number;
    resolved_findings: number;
    auto_fix_count: number;
    findings_summary: string;
    finding_ids: string[];
    error_count: number;
    status: PatrolRunStatus;
    ai_analysis?: string;
    input_tokens?: number;
    output_tokens?: number;
    tool_calls?: ToolCallRecord[];
    tool_call_count: number;
}

function normalizeHistoryLimit(limit: number): number {
    if (!Number.isFinite(limit)) {
        return 30;
    }
    const normalized = Math.floor(limit);
    if (normalized < 1) {
        return 1;
    }
    return normalized;
}

/**
 * Get patrol run history
 * @param limit Maximum number of runs to return (default: 30)
 */
export async function getPatrolRunHistory(limit: number = 30): Promise<PatrolRunRecord[]> {
    const search = new URLSearchParams({
        limit: String(normalizeHistoryLimit(limit)),
    });
    const runs = await apiFetchJSON<PatrolRunRecord[]>(`/api/ai/patrol/runs?${search.toString()}`);
    return runs || [];
}

/**
 * Get patrol run history with tool calls included
 * @param limit Maximum number of runs to return (default: 30)
 */
export async function getPatrolRunHistoryWithToolCalls(limit: number = 30): Promise<PatrolRunRecord[]> {
    const search = new URLSearchParams({
        include: 'tool_calls',
        limit: String(normalizeHistoryLimit(limit)),
    });
    const runs = await apiFetchJSON<PatrolRunRecord[]>(`/api/ai/patrol/runs?${search.toString()}`);
    return runs || [];
}

/** SSE event from /api/ai/patrol/stream */
export interface PatrolStreamEvent {
    // Meta (best-effort; present when backend supports it)
    run_id?: string;
    seq?: number;
    ts_ms?: number;
    resync_reason?: 'late_joiner' | 'stale_last_event_id' | 'buffer_rotated';
    buffer_start_seq?: number;
    buffer_end_seq?: number;
    content_truncated?: boolean;

    type: 'snapshot' | 'start' | 'content' | 'phase' | 'thinking' | 'complete' | 'error' | 'tool_start' | 'tool_end';
    content?: string;
    phase?: string;
    tokens?: number;
    tool_id?: string;
    tool_name?: string;
    tool_input?: string;
    tool_raw_input?: string;
    tool_output?: string;
    tool_success?: boolean;
}

/**
 * Trigger a manual patrol run
 */
export async function triggerPatrolRun(): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON('/api/ai/patrol/run', {
        method: 'POST',
    });
}
