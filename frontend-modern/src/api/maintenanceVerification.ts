import { apiFetchJSON } from '@/utils/apiClient';

/**
 * Maintenance Verification Report — the durable summary Pulse writes
 * when a maintenance window ends for a resource. Mirrors the Go
 * `unified.LoopReport` projected through
 * `internal/api/maintenance_verification.go`.
 *
 * Surfaced as "Maintenance Verification Report" in the product copy.
 * The status enum is small on purpose so the UI can render each value
 * with a stable visual treatment:
 *   - healthy             → green check / quiet
 *   - needs_review        → amber prompt for an operator look
 *   - failed_verification → red surface, operator action expected
 *   - pending             → in-flight; the sentinel hasn't decided
 */
export type MaintenanceVerificationStatus =
  | 'pending'
  | 'healthy'
  | 'needs_review'
  | 'failed_verification';

export type MaintenanceVerificationUserOutcome = 'reviewed' | '';

export interface MaintenanceVerificationMetricRecovery {
  metricsObserved?: string[];
  samplesAfterEnd: number;
  trend?: 'improving' | 'stable' | 'degrading' | 'unknown' | '';
  note?: string;
}

export interface MaintenanceVerificationEvidence {
  operatorStateSummary?: string;
  activeCriticalAlerts: number;
  activeWarningAlerts: number;
  activeCriticalFindings: number;
  activeWarningFindings: number;
  failedActionsSinceWindowStart: number;
  metricRecovery?: MaintenanceVerificationMetricRecovery;
  /**
   * Breadcrumb set by the sentinel when deterministic evidence was
   * ambiguous and a scoped Patrol run would have helped, but
   * triggering one was not safe in this build. Empty when the report
   * was unambiguous.
   */
  patrolRunTodo?: string;
}

export interface MaintenanceVerificationReport {
  id: string;
  resourceId: string;
  trigger: string;
  goal?: string;
  status: MaintenanceVerificationStatus;
  startedAt: string;
  completedAt: string;
  windowStartedAt?: string;
  windowEndedAt?: string;
  evidence: MaintenanceVerificationEvidence;
  linkedFindingIds: string[];
  linkedAlertIds: string[];
  linkedActionIds: string[];
  linkedPatrolRunId?: string;
  recommendation?: string;
  userOutcome?: MaintenanceVerificationUserOutcome;
  reviewedAt?: string;
  reviewedBy?: string;
  reviewNote?: string;
}

export interface MaintenanceVerificationListResponse {
  data: MaintenanceVerificationReport[];
  meta: {
    resourceId: string;
    limit: number;
    total: number;
  };
}

/**
 * Fetch recent Maintenance Verification Reports for a resource,
 * newest first.
 */
export async function listMaintenanceVerificationsForResource(
  resourceId: string,
  limit = 25,
): Promise<MaintenanceVerificationListResponse> {
  const params = new URLSearchParams();
  if (limit && limit > 0) {
    params.set('limit', String(limit));
  }
  const query = params.toString();
  const path = `/api/resources/${encodeURIComponent(resourceId)}/maintenance-verifications${
    query ? `?${query}` : ''
  }`;
  return apiFetchJSON<MaintenanceVerificationListResponse>(path, { cache: 'no-store' });
}

/**
 * Mark a report as reviewed by the operator. The report's status and
 * evidence stay immutable — only the user verdict and review fields
 * update.
 */
export async function reviewMaintenanceVerification(
  reportId: string,
  note?: string,
): Promise<MaintenanceVerificationReport> {
  return apiFetchJSON<MaintenanceVerificationReport>(
    `/api/maintenance-verifications/${encodeURIComponent(reportId)}/review`,
    {
      method: 'POST',
      body: JSON.stringify({ note: note ?? '' }),
      headers: { 'Content-Type': 'application/json' },
    },
  );
}

/**
 * Re-run the deterministic verification immediately. The new report
 * is persisted with a `-rerun-N` suffix on its id so the review
 * history is preserved.
 */
export async function rerunMaintenanceVerification(
  resourceId: string,
): Promise<MaintenanceVerificationReport> {
  return apiFetchJSON<MaintenanceVerificationReport>(
    `/api/resources/${encodeURIComponent(resourceId)}/maintenance-verifications/rerun`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    },
  );
}
