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
import { acknowledgeFinding, snoozeFinding, dismissFinding } from '@/api/patrol';
import type {
  RemediationPlan,
  CircuitBreakerStatus,
  UnifiedFindingRecord,
} from '@/api/ai';
import { logger } from '@/utils/logger';

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
  investigationOutcome?: 'resolved' | 'fix_queued' | 'needs_attention' | 'cannot_fix';
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
          source: (item.source as UnifiedFinding['source']) || 'ai-patrol',
          resourceId: item.resource_id,
          resourceName: item.resource_name || item.resource_id,
          resourceType: item.resource_type || 'unknown',
          alertId: item.alert_id,
          isThreshold: Boolean(item.is_threshold || item.source === 'threshold'),
          category: item.category || 'general',
          severity: (item.severity as UnifiedFinding['severity']) || 'info',
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
  remediationPlansSignal: remediationPlans,

  async loadRemediationPlans() {
    setPlansLoading(true);
    try {
      const resp = await AIAPI.getRemediationPlans();
      setRemediationPlans(resp.plans || []);
    } catch (e) {
      logger.error('Failed to load remediation plans:', e);
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
    ]);
  },

  // Refresh all data
  async refresh() {
    await this.initialize();
  },
};

export default aiIntelligenceStore;
