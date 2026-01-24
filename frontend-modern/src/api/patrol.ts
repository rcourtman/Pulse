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
export type InvestigationOutcome = 'resolved' | 'fix_queued' | 'fix_executed' | 'fix_failed' | 'needs_attention' | 'cannot_fix';
export type PatrolAutonomyLevel = 'monitor' | 'approval' | 'full';

export interface PatrolAutonomySettings {
    autonomy_level: PatrolAutonomyLevel;
    investigation_budget: number;      // Max turns per investigation (5-30)
    investigation_timeout_sec: number; // Max seconds per investigation (60-600)
    critical_require_approval: boolean; // Critical findings always require approval
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
    timestamp: string;
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
    fixed_count: number; // Number of issues auto-fixed by Patrol
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
 * Approve and execute a proposed fix
 */
export async function approveFix(approvalId: string): Promise<{ success: boolean; message: string; output?: string }> {
    return apiFetchJSON(`/api/ai/approvals/${approvalId}/approve`, {
        method: 'POST',
    });
}

/**
 * Deny/skip a proposed fix
 */
export async function denyFix(approvalId: string): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON(`/api/ai/approvals/${approvalId}/deny`, {
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
};
