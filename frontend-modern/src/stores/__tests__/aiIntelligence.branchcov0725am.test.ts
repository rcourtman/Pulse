import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getUnifiedFindings: vi.fn(),
    getRemediationPlans: vi.fn(),
    approveRemediationPlan: vi.fn(),
    executeRemediationPlan: vi.fn(),
    rollbackRemediationPlan: vi.fn(),
    getPendingApprovals: vi.fn(),
    approvePendingApproval: vi.fn(),
    denyPendingApproval: vi.fn(),
    getIntelligenceSummary: vi.fn(),
    getCorrelations: vi.fn(),
  },
}));

vi.mock('@/api/patrol', () => ({
  getPatrolFindings: vi.fn(),
  acknowledgeFinding: vi.fn(),
  snoozeFinding: vi.fn(),
  dismissFinding: vi.fn(),
  resolveFinding: vi.fn(),
  setFindingNote: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsDemoMode: vi.fn(() => false),
}));

import { AIAPI } from '@/api/ai';
import {
  acknowledgeFinding as patrolAcknowledgeFinding,
  snoozeFinding as patrolSnoozeFinding,
  dismissFinding as patrolDismissFinding,
  setFindingNote as patrolSetFindingNote,
  getPatrolFindings,
} from '@/api/patrol';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { presentationPolicyIsDemoMode } from '@/stores/sessionPresentationPolicy';

function approvalExpiryInMinutes(minutesFromNow: number): string {
  return new Date(Date.now() + minutesFromNow * 60_000).toISOString();
}

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

describe('aiIntelligenceStore branch coverage (0725am)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
    vi.mocked(presentationPolicyIsDemoMode).mockReturnValue(false);
    vi.mocked(getPatrolFindings).mockResolvedValue([]);
  });

  // ---------------------------------------------------------------
  // Loading / error getters (TRUE and FALSE states)
  // ---------------------------------------------------------------

  it('exposes findingsLoading true while loadFindings is in flight and false once settled', async () => {
    const { promise, resolve } = deferred<{ findings: unknown[] }>();
    vi.mocked(AIAPI.getUnifiedFindings).mockReturnValueOnce(promise as never);

    const pending = aiIntelligenceStore.loadFindings();
    expect(aiIntelligenceStore.findingsLoading).toBe(true);
    expect(aiIntelligenceStore.findingsError).toBeNull();

    resolve({ findings: [] });
    await pending;

    expect(aiIntelligenceStore.findingsLoading).toBe(false);
  });

  it('surfaces an Error.message through findingsError when loadFindings fails', async () => {
    vi.mocked(AIAPI.getUnifiedFindings).mockRejectedValueOnce(new Error('upstream 503'));

    await aiIntelligenceStore.loadFindings();

    expect(aiIntelligenceStore.findingsError).toBe('upstream 503');
    expect(aiIntelligenceStore.findingsLoading).toBe(false);
  });

  it('falls back to the canonical literal through findingsError for a non-Error rejection', async () => {
    vi.mocked(AIAPI.getUnifiedFindings).mockRejectedValueOnce('network down');

    await aiIntelligenceStore.loadFindings();

    expect(aiIntelligenceStore.findingsError).toBe('Failed to load findings');
  });

  it('returns early without populating findings when the unified findings response is null', async () => {
    // Covers the `if (!resp) return;` arm: no findings written, no error,
    // loading settled false.
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce(null as never);

    await aiIntelligenceStore.loadFindings();

    expect(aiIntelligenceStore.findings).toEqual([]);
    expect(aiIntelligenceStore.findingsError).toBeNull();
    expect(aiIntelligenceStore.findingsLoading).toBe(false);
  });

  it('exposes patrolFindingsLoading true while loadPatrolFindings is in flight and false once settled', async () => {
    const { promise, resolve } = deferred<unknown[]>();
    vi.mocked(getPatrolFindings).mockReturnValueOnce(promise as never);

    const pending = aiIntelligenceStore.loadPatrolFindings();
    expect(aiIntelligenceStore.patrolFindingsLoading).toBe(true);
    expect(aiIntelligenceStore.patrolFindingsError).toBeNull();

    resolve([]);
    await pending;

    expect(aiIntelligenceStore.patrolFindingsLoading).toBe(false);
  });

  it('surfaces an Error.message through patrolFindingsError when loadPatrolFindings fails', async () => {
    vi.mocked(getPatrolFindings).mockRejectedValueOnce(new Error('patrol 500'));

    await aiIntelligenceStore.loadPatrolFindings();

    expect(aiIntelligenceStore.patrolFindingsError).toBe('patrol 500');
    expect(aiIntelligenceStore.patrolFindingsLoading).toBe(false);
  });

  it('falls back to the canonical literal through patrolFindingsError for a non-Error rejection', async () => {
    vi.mocked(getPatrolFindings).mockRejectedValueOnce(42);

    await aiIntelligenceStore.loadPatrolFindings();

    expect(aiIntelligenceStore.patrolFindingsError).toBe('Failed to load Patrol findings');
  });

  it('exposes plansLoading true while loadRemediationPlans is in flight and false once settled', async () => {
    const { promise, resolve } = deferred<{ plans: unknown[] }>();
    vi.mocked(AIAPI.getRemediationPlans).mockReturnValueOnce(promise as never);

    const pending = aiIntelligenceStore.loadRemediationPlans();
    expect(aiIntelligenceStore.plansLoading).toBe(true);

    resolve({ plans: [] });
    await pending;

    expect(aiIntelligenceStore.plansLoading).toBe(false);
  });

  it('surfaces an Error.message through plansError when loadRemediationPlans fails', async () => {
    vi.mocked(AIAPI.getRemediationPlans).mockRejectedValueOnce(new Error('plans 502'));

    await aiIntelligenceStore.loadRemediationPlans();

    expect(aiIntelligenceStore.plansError).toBe('plans 502');
    expect(aiIntelligenceStore.plansLoading).toBe(false);
  });

  it('falls back to the canonical literal through plansError for a non-Error rejection', async () => {
    vi.mocked(AIAPI.getRemediationPlans).mockRejectedValueOnce('timeout');

    await aiIntelligenceStore.loadRemediationPlans();

    expect(aiIntelligenceStore.plansError).toBe('Failed to load remediation plans');
  });

  // ---------------------------------------------------------------
  // Remediation plan actions: approvePlan / executePlan / rollbackPlan
  // ---------------------------------------------------------------

  it('approvePlan returns true on success and rehydrates the plan list into the store', async () => {
    vi.mocked(AIAPI.approveRemediationPlan).mockResolvedValueOnce({ success: true });
    vi.mocked(AIAPI.getRemediationPlans).mockResolvedValueOnce({
      plans: [
        {
          id: 'plan-1',
          finding_id: 'finding-1',
          resource_id: 'vm-1',
          title: 'Restart service',
          description: 'Restart the agent',
          steps: [],
          risk_level: 'low',
          status: 'approved',
          created_at: '2026-07-01T00:00:00Z',
        },
      ],
    } as never);

    const ok = await aiIntelligenceStore.approvePlan('plan-1');

    expect(ok).toBe(true);
    expect(AIAPI.approveRemediationPlan).toHaveBeenCalledWith('plan-1');
    expect(aiIntelligenceStore.remediationPlans).toHaveLength(1);
    expect(aiIntelligenceStore.remediationPlans[0]).toMatchObject({
      id: 'plan-1',
      status: 'approved',
    });
  });

  it('approvePlan returns false and skips the plan reload when approval fails', async () => {
    vi.mocked(AIAPI.approveRemediationPlan).mockRejectedValueOnce(new Error('not allowed'));

    const ok = await aiIntelligenceStore.approvePlan('plan-x');

    expect(ok).toBe(false);
    expect(AIAPI.getRemediationPlans).not.toHaveBeenCalled();
  });

  it('executePlan returns the execution result and rehydrates the plan list on success', async () => {
    vi.mocked(AIAPI.executeRemediationPlan).mockResolvedValueOnce({
      execution_id: 'exec-1',
      plan_id: 'plan-1',
      status: 'success',
      steps_completed: 2,
    });
    vi.mocked(AIAPI.getRemediationPlans).mockResolvedValueOnce({
      plans: [
        {
          id: 'plan-1',
          finding_id: 'finding-1',
          resource_id: 'vm-1',
          title: 'Restart service',
          description: 'Restart the agent',
          steps: [],
          risk_level: 'low',
          status: 'completed',
          created_at: '2026-07-01T00:00:00Z',
        },
      ],
    } as never);

    const result = await aiIntelligenceStore.executePlan('exec-1');

    expect(result).toMatchObject({ execution_id: 'exec-1', status: 'success' });
    expect(AIAPI.executeRemediationPlan).toHaveBeenCalledWith('exec-1');
    expect(aiIntelligenceStore.remediationPlans[0]).toMatchObject({
      id: 'plan-1',
      status: 'completed',
    });
  });

  it('executePlan rethrows and does not reload plans when execution fails', async () => {
    vi.mocked(AIAPI.executeRemediationPlan).mockRejectedValueOnce(new Error('exec blew up'));

    await expect(aiIntelligenceStore.executePlan('exec-bad')).rejects.toThrow('exec blew up');
    expect(AIAPI.getRemediationPlans).not.toHaveBeenCalled();
  });

  it('rollbackPlan returns true and rehydrates the plan list on success', async () => {
    vi.mocked(AIAPI.rollbackRemediationPlan).mockResolvedValueOnce({ success: true });
    vi.mocked(AIAPI.getRemediationPlans).mockResolvedValueOnce({
      plans: [
        {
          id: 'plan-1',
          finding_id: 'finding-1',
          resource_id: 'vm-1',
          title: 'Restart service',
          description: 'Restart the agent',
          steps: [],
          risk_level: 'low',
          status: 'rolled_back',
          created_at: '2026-07-01T00:00:00Z',
        },
      ],
    } as never);

    const ok = await aiIntelligenceStore.rollbackPlan('exec-1');

    expect(ok).toBe(true);
    expect(AIAPI.rollbackRemediationPlan).toHaveBeenCalledWith('exec-1');
    expect(aiIntelligenceStore.remediationPlans[0]).toMatchObject({
      id: 'plan-1',
      status: 'rolled_back',
    });
  });

  it('rollbackPlan returns false and skips the plan reload when rollback fails', async () => {
    vi.mocked(AIAPI.rollbackRemediationPlan).mockRejectedValueOnce(new Error('no rollback'));

    const ok = await aiIntelligenceStore.rollbackPlan('exec-bad');

    expect(ok).toBe(false);
    expect(AIAPI.getRemediationPlans).not.toHaveBeenCalled();
  });

  // ---------------------------------------------------------------
  // Finding lifecycle actions
  // ---------------------------------------------------------------

  it('acknowledgeFinding returns true and refreshes the patrol finding status into the store', async () => {
    vi.mocked(patrolAcknowledgeFinding).mockResolvedValueOnce({ success: true, message: 'ok' });
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({ findings: [] } as never);
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([
      {
        id: 'finding-ack',
        severity: 'warning',
        category: 'reliability',
        resource_id: 'instance:node:1',
        resource_name: 'node-1',
        resource_type: 'host',
        title: 'Disk pressure',
        description: 'Patrol flagged disk pressure.',
        detected_at: '2026-07-01T00:00:00Z',
        last_seen_at: '2026-07-01T00:05:00Z',
        resolved_at: '2026-07-01T00:10:00Z',
        auto_resolved: false,
        times_raised: 1,
        suppressed: false,
        investigation_attempts: 0,
      },
    ]);

    const ok = await aiIntelligenceStore.acknowledgeFinding('finding-ack');

    expect(ok).toBe(true);
    expect(patrolAcknowledgeFinding).toHaveBeenCalledWith('finding-ack');
    expect(aiIntelligenceStore.patrolFindings).toHaveLength(1);
    expect(aiIntelligenceStore.patrolFindings[0]).toMatchObject({
      id: 'finding-ack',
      status: 'resolved',
    });
  });

  it('acknowledgeFinding returns false and performs no reload when the patrol call fails', async () => {
    vi.mocked(patrolAcknowledgeFinding).mockRejectedValueOnce(new Error('conflict'));

    const ok = await aiIntelligenceStore.acknowledgeFinding('finding-ack');

    expect(ok).toBe(false);
    expect(AIAPI.getUnifiedFindings).not.toHaveBeenCalled();
    expect(getPatrolFindings).not.toHaveBeenCalled();
  });

  it('snoozeFinding returns true and refreshes both finding sources on success', async () => {
    vi.mocked(patrolSnoozeFinding).mockResolvedValueOnce({ success: true, message: 'snoozed' });
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({
      findings: [
        {
          id: 'finding-snooze',
          source: 'ai-patrol',
          severity: 'warning',
          category: 'reliability',
          resource_id: 'instance:node:2',
          resource_name: 'node-2',
          resource_type: 'host',
          title: 'Disk pressure',
          description: 'Snoozed by operator.',
          detected_at: '2026-07-01T00:00:00Z',
          snoozed_until: '2026-07-02T00:00:00Z',
          status: 'snoozed',
        },
      ],
    } as never);
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([]);

    const ok = await aiIntelligenceStore.snoozeFinding('finding-snooze', 24);

    expect(ok).toBe(true);
    expect(patrolSnoozeFinding).toHaveBeenCalledWith('finding-snooze', 24);
    expect(aiIntelligenceStore.findings).toHaveLength(1);
    expect(aiIntelligenceStore.findings[0]).toMatchObject({
      id: 'finding-snooze',
      status: 'snoozed',
    });
  });

  it('snoozeFinding returns false and performs no reload when the patrol call fails', async () => {
    vi.mocked(patrolSnoozeFinding).mockRejectedValueOnce(new Error('nope'));

    const ok = await aiIntelligenceStore.snoozeFinding('finding-snooze', 24);

    expect(ok).toBe(false);
    expect(AIAPI.getUnifiedFindings).not.toHaveBeenCalled();
  });

  it('dismissFinding returns true and refreshes both finding sources on success', async () => {
    vi.mocked(patrolDismissFinding).mockResolvedValueOnce({ success: true, message: 'dismissed' });
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({ findings: [] } as never);
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([
      {
        id: 'finding-dismiss',
        severity: 'info',
        category: 'reliability',
        resource_id: 'instance:node:3',
        resource_name: 'node-3',
        resource_type: 'host',
        title: 'Noisy check',
        description: 'Operator dismissed as expected.',
        detected_at: '2026-07-01T00:00:00Z',
        last_seen_at: '2026-07-01T00:05:00Z',
        auto_resolved: false,
        times_raised: 1,
        suppressed: false,
        investigation_attempts: 0,
        dismissed_reason: 'expected_behavior',
      },
    ]);

    const ok = await aiIntelligenceStore.dismissFinding(
      'finding-dismiss',
      'expected_behavior',
      'routine',
    );

    expect(ok).toBe(true);
    expect(patrolDismissFinding).toHaveBeenCalledWith(
      'finding-dismiss',
      'expected_behavior',
      'routine',
    );
    expect(aiIntelligenceStore.patrolFindings[0]).toMatchObject({
      id: 'finding-dismiss',
      status: 'dismissed',
      dismissedReason: 'expected_behavior',
    });
  });

  it('dismissFinding returns false and performs no reload when the patrol call fails', async () => {
    vi.mocked(patrolDismissFinding).mockRejectedValueOnce(new Error('deny'));

    const ok = await aiIntelligenceStore.dismissFinding('finding-dismiss', 'not_an_issue');

    expect(ok).toBe(false);
    expect(getPatrolFindings).not.toHaveBeenCalled();
  });

  it('setFindingNote optimistically updates the note on both unified and patrol findings on success', async () => {
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({
      findings: [
        {
          id: 'finding-note',
          source: 'ai-patrol',
          severity: 'warning',
          category: 'reliability',
          resource_id: 'instance:node:4',
          resource_name: 'node-4',
          resource_type: 'host',
          title: 'Disk pressure',
          description: 'Add a note.',
          detected_at: '2026-07-01T00:00:00Z',
        },
      ],
    } as never);
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([
      {
        id: 'finding-note',
        severity: 'warning',
        category: 'reliability',
        resource_id: 'instance:node:4',
        resource_name: 'node-4',
        resource_type: 'host',
        title: 'Disk pressure',
        description: 'Add a note.',
        detected_at: '2026-07-01T00:00:00Z',
        last_seen_at: '2026-07-01T00:05:00Z',
        auto_resolved: false,
        times_raised: 1,
        suppressed: false,
        investigation_attempts: 0,
      },
    ]);
    vi.mocked(patrolSetFindingNote).mockResolvedValueOnce({ success: true, message: 'saved' });

    await aiIntelligenceStore.loadFindings();
    await aiIntelligenceStore.loadPatrolFindings();
    expect(aiIntelligenceStore.findings[0].userNote).toBeUndefined();

    const ok = await aiIntelligenceStore.setFindingNote('finding-note', 'operator context');

    expect(ok).toBe(true);
    expect(patrolSetFindingNote).toHaveBeenCalledWith('finding-note', 'operator context');
    // No reload performed — the optimistic update is the only state change.
    expect(AIAPI.getUnifiedFindings).toHaveBeenCalledTimes(1);
    expect(getPatrolFindings).toHaveBeenCalledTimes(1);
    expect(aiIntelligenceStore.findings[0].userNote).toBe('operator context');
    expect(aiIntelligenceStore.patrolFindings[0].userNote).toBe('operator context');
  });

  it('setFindingNote returns false and leaves the local note unchanged when the patrol call fails', async () => {
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({
      findings: [
        {
          id: 'finding-note',
          source: 'ai-patrol',
          severity: 'warning',
          category: 'reliability',
          resource_id: 'instance:node:4',
          resource_name: 'node-4',
          resource_type: 'host',
          title: 'Disk pressure',
          description: 'Add a note.',
          detected_at: '2026-07-01T00:00:00Z',
        },
      ],
    } as never);
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([]);
    vi.mocked(patrolSetFindingNote).mockRejectedValueOnce(new Error('write failed'));

    await aiIntelligenceStore.loadFindings();
    await aiIntelligenceStore.loadPatrolFindings();

    const ok = await aiIntelligenceStore.setFindingNote('finding-note', 'never persisted');

    expect(ok).toBe(false);
    expect(aiIntelligenceStore.findings[0].userNote).toBeUndefined();
  });

  // ---------------------------------------------------------------
  // Investigation-fix approval actions
  // ---------------------------------------------------------------

  it('approvePendingApproval returns the decision result and clears the live approval after reload', async () => {
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([
      {
        id: 'approval-1',
        toolId: 'investigation_fix',
        command: 'systemctl restart pulse-agent',
        targetType: 'host',
        targetId: 'finding-queued',
        targetName: 'node-1',
        context: 'Restart the agent',
        riskLevel: 'medium',
        status: 'pending',
        requestedAt: '2026-07-01T00:01:00Z',
        expiresAt: approvalExpiryInMinutes(6),
      },
    ]);
    await aiIntelligenceStore.loadPendingApprovals();
    expect(aiIntelligenceStore.pendingApprovalCount).toBe(1);

    vi.mocked(AIAPI.approvePendingApproval).mockResolvedValueOnce({
      approved: true,
      executed: true,
      success: true,
      message: 'executed',
    });
    // Reloads after the decision return an empty approval set and empty findings.
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([]);
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({ findings: [] } as never);
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([]);

    const result = await aiIntelligenceStore.approvePendingApproval('approval-1');

    expect(result).toMatchObject({ approved: true, success: true, message: 'executed' });
    expect(AIAPI.approvePendingApproval).toHaveBeenCalledWith('approval-1');
    expect(aiIntelligenceStore.pendingApprovalCount).toBe(0);
  });

  it('approvePendingApproval returns null when the API rejects and leaves the approval live', async () => {
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([
      {
        id: 'approval-1',
        toolId: 'investigation_fix',
        command: 'systemctl restart pulse-agent',
        targetType: 'host',
        targetId: 'finding-queued',
        targetName: 'node-1',
        context: 'Restart the agent',
        riskLevel: 'medium',
        status: 'pending',
        requestedAt: '2026-07-01T00:01:00Z',
        expiresAt: approvalExpiryInMinutes(6),
      },
    ]);
    await aiIntelligenceStore.loadPendingApprovals();

    vi.mocked(AIAPI.approvePendingApproval).mockRejectedValueOnce(new Error('server refused'));

    const result = await aiIntelligenceStore.approvePendingApproval('approval-1');

    expect(result).toBeNull();
    // No reload ran, so the live approval is still counted.
    expect(aiIntelligenceStore.pendingApprovalCount).toBe(1);
  });

  it('approveInvestigationFix delegates to approvePendingApproval and casts the result on success', async () => {
    vi.mocked(AIAPI.approvePendingApproval).mockResolvedValueOnce({
      approved: true,
      executed: true,
      success: true,
      output: 'restarted',
      exit_code: 0,
      finding_id: 'finding-queued',
      message: 'executed',
    });
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([]);
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({ findings: [] } as never);
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([]);

    const result = await aiIntelligenceStore.approveInvestigationFix('approval-1');

    expect(result).not.toBeNull();
    expect(result).toMatchObject({
      approved: true,
      executed: true,
      success: true,
      exit_code: 0,
      finding_id: 'finding-queued',
    });
    expect(AIAPI.approvePendingApproval).toHaveBeenCalledWith('approval-1');
  });

  it('approveInvestigationFix returns null when the underlying approval fails', async () => {
    vi.mocked(AIAPI.approvePendingApproval).mockRejectedValueOnce(new Error('nope'));

    const result = await aiIntelligenceStore.approveInvestigationFix('approval-bad');

    expect(result).toBeNull();
  });

  it('denyPendingApproval returns true and clears the live approval after reload', async () => {
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([
      {
        id: 'approval-1',
        toolId: 'investigation_fix',
        command: 'systemctl restart pulse-agent',
        targetType: 'host',
        targetId: 'finding-queued',
        targetName: 'node-1',
        context: 'Restart the agent',
        riskLevel: 'medium',
        status: 'pending',
        requestedAt: '2026-07-01T00:01:00Z',
        expiresAt: approvalExpiryInMinutes(6),
      },
    ]);
    await aiIntelligenceStore.loadPendingApprovals();
    expect(aiIntelligenceStore.pendingApprovalCount).toBe(1);

    vi.mocked(AIAPI.denyPendingApproval).mockResolvedValueOnce({
      id: 'approval-1',
      status: 'denied',
    } as never);
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([]);
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({ findings: [] } as never);
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([]);

    const ok = await aiIntelligenceStore.denyPendingApproval('approval-1', 'too risky');

    expect(ok).toBe(true);
    expect(AIAPI.denyPendingApproval).toHaveBeenCalledWith('approval-1', 'too risky');
    expect(aiIntelligenceStore.pendingApprovalCount).toBe(0);
  });

  it('denyPendingApproval returns false and leaves the approval live when the API rejects', async () => {
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([
      {
        id: 'approval-1',
        toolId: 'investigation_fix',
        command: 'systemctl restart pulse-agent',
        targetType: 'host',
        targetId: 'finding-queued',
        targetName: 'node-1',
        context: 'Restart the agent',
        riskLevel: 'medium',
        status: 'pending',
        requestedAt: '2026-07-01T00:01:00Z',
        expiresAt: approvalExpiryInMinutes(6),
      },
    ]);
    await aiIntelligenceStore.loadPendingApprovals();

    vi.mocked(AIAPI.denyPendingApproval).mockRejectedValueOnce(new Error('deny failed'));

    const ok = await aiIntelligenceStore.denyPendingApproval('approval-1');

    expect(ok).toBe(false);
    expect(aiIntelligenceStore.pendingApprovalCount).toBe(1);
  });

  it('denyInvestigationFix delegates to denyPendingApproval and reports success', async () => {
    vi.mocked(AIAPI.denyPendingApproval).mockResolvedValueOnce({
      id: 'approval-1',
      status: 'denied',
    } as never);
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([]);
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({ findings: [] } as never);
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([]);

    const ok = await aiIntelligenceStore.denyInvestigationFix('approval-1', 'wrong call');

    expect(ok).toBe(true);
    expect(AIAPI.denyPendingApproval).toHaveBeenCalledWith('approval-1', 'wrong call');
  });

  it('denyInvestigationFix delegates the failure path and reports false', async () => {
    vi.mocked(AIAPI.denyPendingApproval).mockRejectedValueOnce(new Error('nope'));

    const ok = await aiIntelligenceStore.denyInvestigationFix('approval-bad');

    expect(ok).toBe(false);
  });

  // ---------------------------------------------------------------
  // Patrol-scoped derived getters + counts
  // ---------------------------------------------------------------

  it('patrolFindingsWithPendingApprovals mirrors live investigation_fix approvals scoped to patrol findings', async () => {
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([
      {
        id: 'patrol-queued',
        severity: 'warning',
        category: 'reliability',
        resource_id: 'instance:node:9',
        resource_name: 'node-9',
        resource_type: 'host',
        title: 'Queued Patrol fix',
        description: 'Patrol needs approval.',
        detected_at: '2026-07-01T00:00:00Z',
        last_seen_at: '2026-07-01T00:05:00Z',
        auto_resolved: false,
        times_raised: 1,
        suppressed: false,
        investigation_attempts: 0,
        investigation_outcome: 'fix_queued',
      },
    ]);
    await aiIntelligenceStore.loadPatrolFindings();

    // No live approval yet: nothing surfaces.
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([]);
    await aiIntelligenceStore.loadPendingApprovals();
    expect(aiIntelligenceStore.patrolFindingsWithPendingApprovals).toEqual([]);

    // A live investigation_fix approval targeting the patrol finding promotes it.
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([
      {
        id: 'approval-patrol',
        toolId: 'investigation_fix',
        command: 'systemctl restart pulse-agent',
        targetType: 'host',
        targetId: 'patrol-queued',
        targetName: 'node-9',
        context: 'Restart the agent',
        riskLevel: 'medium',
        status: 'pending',
        requestedAt: '2026-07-01T00:01:00Z',
        expiresAt: approvalExpiryInMinutes(6),
      },
    ]);
    await aiIntelligenceStore.loadPendingApprovals();

    expect(aiIntelligenceStore.patrolFindingsWithPendingApprovals.map((f) => f.id)).toEqual([
      'patrol-queued',
    ]);
  });

  it('patrolFindingsNeedingAttention surfaces queued-without-approval patrol findings and clears once approved', async () => {
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([
      {
        id: 'patrol-queued',
        severity: 'warning',
        category: 'reliability',
        resource_id: 'instance:node:9',
        resource_name: 'node-9',
        resource_type: 'host',
        title: 'Queued Patrol fix',
        description: 'Patrol needs approval.',
        detected_at: '2026-07-01T00:00:00Z',
        last_seen_at: '2026-07-01T00:05:00Z',
        auto_resolved: false,
        times_raised: 1,
        suppressed: false,
        investigation_attempts: 0,
        investigation_outcome: 'fix_queued',
      },
    ]);
    await aiIntelligenceStore.loadPatrolFindings();

    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([]);
    await aiIntelligenceStore.loadPendingApprovals();
    expect(aiIntelligenceStore.patrolFindingsNeedingAttention.map((f) => f.id)).toEqual([
      'patrol-queued',
    ]);

    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([
      {
        id: 'approval-patrol',
        toolId: 'investigation_fix',
        command: 'systemctl restart pulse-agent',
        targetType: 'host',
        targetId: 'patrol-queued',
        targetName: 'node-9',
        context: 'Restart the agent',
        riskLevel: 'medium',
        status: 'pending',
        requestedAt: '2026-07-01T00:01:00Z',
        expiresAt: approvalExpiryInMinutes(6),
      },
    ]);
    await aiIntelligenceStore.loadPendingApprovals();
    expect(aiIntelligenceStore.patrolFindingsNeedingAttention).toEqual([]);
  });

  it('needsAttentionCount tracks the unified needing-attention list length', async () => {
    // FALSE arm: empty unified findings -> zero needing attention.
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({ findings: [] } as never);
    await aiIntelligenceStore.loadFindings();
    expect(aiIntelligenceStore.needsAttentionCount).toBe(0);

    // TRUE arm: two active findings with attention outcomes.
    vi.mocked(AIAPI.getUnifiedFindings).mockResolvedValueOnce({
      findings: [
        {
          id: 'unified-failed-1',
          source: 'ai-patrol',
          severity: 'warning',
          category: 'performance',
          resource_id: 'instance:node:10',
          resource_name: 'node-10',
          resource_type: 'host',
          title: 'Fix failed',
          description: 'Patrol fix failed.',
          detected_at: '2026-07-01T00:00:00Z',
          status: 'active',
          investigation_outcome: 'fix_failed',
        },
        {
          id: 'unified-failed-2',
          source: 'ai-patrol',
          severity: 'critical',
          category: 'performance',
          resource_id: 'instance:node:11',
          resource_name: 'node-11',
          resource_type: 'host',
          title: 'Verification failed',
          description: 'Patrol could not verify the fix.',
          detected_at: '2026-07-01T00:00:00Z',
          status: 'active',
          investigation_outcome: 'fix_verification_failed',
        },
      ],
    } as never);
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([]);

    await aiIntelligenceStore.loadFindings();
    await aiIntelligenceStore.loadPendingApprovals();

    expect(aiIntelligenceStore.needsAttentionCount).toBe(2);
    expect(aiIntelligenceStore.needsAttentionCount).toBe(
      aiIntelligenceStore.findingsNeedingAttention.length,
    );
  });

  it('patrolNeedsAttentionCount tracks the patrol needing-attention list length', async () => {
    // FALSE arm: reset patrol findings to empty.
    await aiIntelligenceStore.loadPatrolFindings();
    expect(aiIntelligenceStore.patrolNeedsAttentionCount).toBe(0);

    // TRUE arm: one active patrol finding with a failed fix outcome.
    vi.mocked(getPatrolFindings).mockResolvedValueOnce([
      {
        id: 'patrol-attention',
        severity: 'warning',
        category: 'reliability',
        resource_id: 'instance:node:12',
        resource_name: 'node-12',
        resource_type: 'host',
        title: 'Fix failed',
        description: 'Patrol fix failed.',
        detected_at: '2026-07-01T00:00:00Z',
        last_seen_at: '2026-07-01T00:05:00Z',
        auto_resolved: false,
        times_raised: 1,
        suppressed: false,
        investigation_attempts: 1,
        investigation_outcome: 'fix_failed',
      },
    ]);
    vi.mocked(AIAPI.getPendingApprovals).mockResolvedValueOnce([]);

    await aiIntelligenceStore.loadPatrolFindings();
    await aiIntelligenceStore.loadPendingApprovals();

    expect(aiIntelligenceStore.patrolNeedsAttentionCount).toBe(1);
    expect(aiIntelligenceStore.patrolNeedsAttentionCount).toBe(
      aiIntelligenceStore.patrolFindingsNeedingAttention.length,
    );
  });
});
