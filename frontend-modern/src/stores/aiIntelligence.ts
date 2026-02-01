/**
 * AI Intelligence Store
 *
 * Central store for managing AI intelligence state:
 * - Unified findings (alerts + AI findings)
 * - Remediation plans
 * - Circuit breaker status
 */

import { createSignal } from 'solid-js';
import { AIAPI } from '@/api/ai';
import { acknowledgeFinding, snoozeFinding, dismissFinding, setFindingNote } from '@/api/patrol';
import type {
  RemediationPlan,
  CircuitBreakerStatus,
  UnifiedFindingRecord,
  ApprovalRequest,
  ApprovalExecutionResult,
} from '@/api/ai';
import { logger } from '@/utils/logger';

// ============================================
// Enum validation helpers
// ============================================

const VALID_INVESTIGATION_STATUSES = new Set<string>(['pending', 'running', 'completed', 'failed', 'needs_attention']);
const VALID_INVESTIGATION_OUTCOMES = new Set<string>([
  'resolved', 'fix_queued', 'fix_executed', 'fix_failed',
  'needs_attention', 'cannot_fix', 'timed_out', 'fix_verified', 'fix_verification_failed',
]);
const VALID_SEVERITIES = new Set<string>(['critical', 'warning', 'info', 'watch']);
const VALID_SOURCES = new Set<string>(['threshold', 'ai-patrol', 'ai-chat', 'anomaly', 'correlation', 'forecast']);

function validateInvestigationStatus(value: string | undefined): UnifiedFinding['investigationStatus'] {
  if (!value) return undefined;
  return VALID_INVESTIGATION_STATUSES.has(value) ? value as UnifiedFinding['investigationStatus'] : undefined;
}

function validateInvestigationOutcome(value: string | undefined): UnifiedFinding['investigationOutcome'] {
  if (!value) return undefined;
  return VALID_INVESTIGATION_OUTCOMES.has(value) ? value as UnifiedFinding['investigationOutcome'] : undefined;
}

function validateSeverity(value: string | undefined): UnifiedFinding['severity'] {
  if (value && VALID_SEVERITIES.has(value)) return value as UnifiedFinding['severity'];
  return 'info';
}

function validateSource(value: string | undefined): UnifiedFinding['source'] {
  if (value && VALID_SOURCES.has(value)) return value as UnifiedFinding['source'];
  return 'ai-patrol';
}

// ============================================
// Unified Findings
// ============================================

export interface UnifiedFinding {
  id: string;
  source: 'threshold' | 'ai-patrol' | 'ai-chat' | 'anomaly' | 'correlation' | 'forecast';
  resourceId: string;
  resourceName: string;
  resourceType: string;
  alertId?: string;
  alertType?: string;
  isThreshold?: boolean;
  category: string;
  severity: 'critical' | 'warning' | 'info' | 'watch';
  title: string;
  description: string;
  recommendation?: string;
  detectedAt: string;
  resolvedAt?: string;
  acknowledgedAt?: string;
  snoozedUntil?: string;
  dismissedReason?: string;
  userNote?: string;
  status: 'active' | 'resolved' | 'dismissed' | 'snoozed';
  correlatedFindingIds?: string[];
  remediationPlanId?: string;
  // Investigation fields (Patrol Autonomy)
  investigationSessionId?: string;
  investigationStatus?: 'pending' | 'running' | 'completed' | 'failed' | 'needs_attention';
  investigationOutcome?: 'resolved' | 'fix_queued' | 'fix_executed' | 'fix_failed' | 'needs_attention' | 'cannot_fix' | 'timed_out' | 'fix_verified' | 'fix_verification_failed';
  lastInvestigatedAt?: string;
  investigationAttempts?: number;
}

const [unifiedFindings, setUnifiedFindings] = createSignal<UnifiedFinding[]>([]);
const [findingsLoading, setFindingsLoading] = createSignal(false);
const [findingsError, setFindingsError] = createSignal<string | null>(null);

// ============================================
// Remediation Plans
// ============================================

const [remediationPlans, setRemediationPlans] = createSignal<RemediationPlan[]>([]);
const [plansLoading, setPlansLoading] = createSignal(false);
const [plansError, setPlansError] = createSignal<string | null>(null);

// ============================================
// Pending Approvals
// ============================================

const [pendingApprovals, setPendingApprovals] = createSignal<ApprovalRequest[]>([]);
const [approvalsError, setApprovalsError] = createSignal<string | null>(null);

// ============================================
// Circuit Breaker
// ============================================

const [circuitBreakerStatus, setCircuitBreakerStatus] = createSignal<CircuitBreakerStatus | null>(null);

// ============================================
// Store API
// ============================================

export const aiIntelligenceStore = {
  // Unified Findings
  get findings() { return unifiedFindings(); },
  get findingsLoading() { return findingsLoading(); },
  get findingsError() { return findingsError(); },
  findingsSignal: unifiedFindings,

  async loadFindings() {
    setFindingsLoading(true);
    setFindingsError(null);
    try {
      const resp = await AIAPI.getUnifiedFindings({ includeResolved: true });
      if (!resp) return;
      const now = Date.now();

      const findings = (resp.findings || []).map((item: UnifiedFindingRecord): UnifiedFinding => {
        let status = item.status as UnifiedFinding['status'] | undefined;
        if (!status) {
          if (item.resolved_at) {
            status = 'resolved';
          } else if (item.snoozed_until && new Date(item.snoozed_until).getTime() > now) {
            status = 'snoozed';
          } else if (item.dismissed_reason || item.suppressed) {
            status = 'dismissed';
          } else {
            status = 'active';
          }
        }

        return {
          id: item.id,
          source: validateSource(item.source),
          resourceId: item.resource_id,
          resourceName: item.resource_name || item.resource_id,
          resourceType: item.resource_type || 'unknown',
          alertId: item.alert_id,
          isThreshold: Boolean(item.is_threshold || item.source === 'threshold'),
          category: item.category || 'general',
          severity: validateSeverity(item.severity),
          title: item.title,
          description: item.description,
          recommendation: item.recommendation,
          detectedAt: item.detected_at,
          resolvedAt: item.resolved_at,
          acknowledgedAt: item.acknowledged_at,
          snoozedUntil: item.snoozed_until,
          dismissedReason: item.dismissed_reason,
          userNote: item.user_note,
          status,
          correlatedFindingIds: item.correlated_ids,
          remediationPlanId: item.remediation_id,
          investigationSessionId: item.investigation_session_id || '',
          investigationStatus: validateInvestigationStatus(item.investigation_status),
          investigationOutcome: validateInvestigationOutcome(item.investigation_outcome),
          lastInvestigatedAt: item.last_investigated_at || undefined,
          investigationAttempts: item.investigation_attempts || 0,
        };
      });

      setUnifiedFindings(findings);
    } catch (e) {
      logger.error('Failed to load unified findings:', e);
      setFindingsError(e instanceof Error ? e.message : 'Failed to load findings');
    } finally {
      setFindingsLoading(false);
    }
  },

  // Remediation Plans
  get remediationPlans() { return remediationPlans(); },
  get plansLoading() { return plansLoading(); },
  get plansError() { return plansError(); },
  remediationPlansSignal: remediationPlans,

  async loadRemediationPlans() {
    setPlansLoading(true);
    setPlansError(null);
    try {
      const resp = await AIAPI.getRemediationPlans();
      setRemediationPlans(resp?.plans || []);
    } catch (e) {
      logger.error('Failed to load remediation plans:', e);
      setPlansError(e instanceof Error ? e.message : 'Failed to load remediation plans');
    } finally {
      setPlansLoading(false);
    }
  },

  async approvePlan(planId: string) {
    try {
      await AIAPI.approveRemediationPlan(planId);
      await this.loadRemediationPlans();
      return true;
    } catch (e) {
      logger.error('Failed to approve plan:', e);
      return false;
    }
  },

  async executePlan(planId: string) {
    try {
      const result = await AIAPI.executeRemediationPlan(planId);
      await this.loadRemediationPlans();
      return result;
    } catch (e) {
      logger.error('Failed to execute plan:', e);
      throw e;
    }
  },

  async rollbackPlan(executionId: string) {
    try {
      await AIAPI.rollbackRemediationPlan(executionId);
      await this.loadRemediationPlans();
      return true;
    } catch (e) {
      logger.error('Failed to rollback plan:', e);
      return false;
    }
  },

  async acknowledgeFinding(findingId: string) {
    try {
      await acknowledgeFinding(findingId);
      await this.loadFindings();
      return true;
    } catch (e) {
      logger.error('Failed to acknowledge finding:', e);
      return false;
    }
  },

  async snoozeFinding(findingId: string, durationHours: number) {
    try {
      await snoozeFinding(findingId, durationHours);
      await this.loadFindings();
      return true;
    } catch (e) {
      logger.error('Failed to snooze finding:', e);
      return false;
    }
  },

  async dismissFinding(
    findingId: string,
    reason: 'not_an_issue' | 'expected_behavior' | 'will_fix_later',
    note?: string,
  ) {
    try {
      await dismissFinding(findingId, reason, note);
      await this.loadFindings();
      return true;
    } catch (e) {
      logger.error('Failed to dismiss finding:', e);
      return false;
    }
  },

  async setFindingNote(findingId: string, note: string) {
    try {
      await setFindingNote(findingId, note);
      // Update local state immediately for responsiveness
      setUnifiedFindings(prev =>
        prev.map(f => f.id === findingId ? { ...f, userNote: note } : f),
      );
      return true;
    } catch (e) {
      logger.error('Failed to set finding note:', e);
      return false;
    }
  },

  // Pending Approvals
  get pendingApprovals() { return pendingApprovals(); },
  get approvalsError() { return approvalsError(); },
  pendingApprovalsSignal: pendingApprovals,

  get pendingApprovalCount() {
    return pendingApprovals().filter(a => a.status === 'pending').length;
  },

  get findingsWithPendingApprovals() {
    const approvals = pendingApprovals().filter(a => a.status === 'pending');
    const findingIds = new Set(approvals.filter(a => a.toolId === 'investigation_fix').map(a => a.targetId));
    return unifiedFindings().filter(f => findingIds.has(f.id));
  },

  get findingsNeedingAttention() {
    const actionableOutcomes = new Set(['fix_verification_failed', 'fix_failed', 'timed_out', 'needs_attention', 'cannot_fix']);
    return unifiedFindings().filter(f =>
      f.status === 'active' && f.investigationOutcome && actionableOutcomes.has(f.investigationOutcome)
    );
  },

  get needsAttentionCount() {
    return this.findingsNeedingAttention.length;
  },

  async loadPendingApprovals() {
    setApprovalsError(null);
    try {
      const approvals = await AIAPI.getPendingApprovals();
      setPendingApprovals(approvals);
    } catch (e) {
      logger.error('Failed to load pending approvals:', e);
      setApprovalsError(e instanceof Error ? e.message : 'Failed to load approvals');
    }
  },

  async approveInvestigationFix(approvalId: string): Promise<ApprovalExecutionResult | null> {
    try {
      const result = await AIAPI.approveInvestigationFix(approvalId);
      await this.loadPendingApprovals();
      await this.loadFindings();
      return result;
    } catch (e) {
      logger.error('Failed to approve fix:', e);
      return null;
    }
  },

  async denyInvestigationFix(approvalId: string, reason?: string) {
    try {
      await AIAPI.denyInvestigationFix(approvalId, reason);
      await this.loadPendingApprovals();
      await this.loadFindings();
      return true;
    } catch (e) {
      logger.error('Failed to deny fix:', e);
      return false;
    }
  },

  // Circuit Breaker
  get circuitBreakerStatus() { return circuitBreakerStatus(); },
  circuitBreakerStatusSignal: circuitBreakerStatus,

  async loadCircuitBreakerStatus() {
    try {
      const status = await AIAPI.getCircuitBreakerStatus();
      setCircuitBreakerStatus(status);
    } catch (e) {
      logger.error('Failed to load circuit breaker status:', e);
    }
  },

  // Initialize - load all data
  async initialize() {
    await Promise.all([
      this.loadFindings(),
      this.loadRemediationPlans(),
      this.loadCircuitBreakerStatus(),
      this.loadPendingApprovals(),
    ]);
  },

  // Refresh all data
  async refresh() {
    await this.initialize();
  },
};

export default aiIntelligenceStore;
