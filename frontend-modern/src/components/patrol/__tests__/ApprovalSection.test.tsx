import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { InvestigationSession } from '@/api/ai';
import type { ActionAuditRecord, PatrolActionReference } from '@/types/actionAudit';
import approvalSectionSource from '../ApprovalSection.tsx?raw';
import ApprovalSection from '../ApprovalSection';

const state = vi.hoisted(() => ({ hasAutoFix: true }));
const getInvestigationMock = vi.hoisted(() => vi.fn());
const decideActionMock = vi.hoisted(() => vi.fn());
const executeActionMock = vi.hoisted(() => vi.fn());
const loadFindingsMock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined));
const notificationSuccessMock = vi.hoisted(() => vi.fn());
const notificationWarningMock = vi.hoisted(() => vi.fn());
const notificationErrorMock = vi.hoisted(() => vi.fn());
const openMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/ai', () => ({ AIAPI: { getInvestigation: getInvestigationMock } }));
vi.mock('@/api/resourceActions', () => ({
  ResourceActionsAPI: {
    decideAction: (...args: unknown[]) => decideActionMock(...args),
    executeAction: (...args: unknown[]) => executeActionMock(...args),
  },
}));
vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: { loadFindings: (...args: unknown[]) => loadFindingsMock(...args) },
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    warning: (...args: unknown[]) => notificationWarningMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));
vi.mock('@/stores/aiChat', () => ({
  aiChatStore: { open: (...args: unknown[]) => openMock(...args) },
}));
vi.mock('@/stores/license', () => ({
  hasFeature: (feature: string) => feature === 'ai_autofix' && state.hasAutoFix,
}));

function actionReference(stateValue: PatrolActionReference['state']): PatrolActionReference {
  return {
    action_id: 'act-1',
    proposal_id: 'proposal-1',
    resource_id: 'vm:42',
    capability_name: 'restart',
    state: stateValue,
    plan: {
      actionId: 'act-1',
      requestId: 'proposal-1',
      allowed: true,
      requiresApproval: stateValue === 'pending_approval',
      approvalPolicy: stateValue === 'pending_approval' ? 'admin' : 'none',
      rollbackAvailable: true,
      message: 'Restart the unhealthy workload',
      preflight: {
        target: 'vm:42',
        currentState: 'degraded',
        intendedChange: 'Restart the workload',
        dryRunAvailable: true,
        dryRunSummary: 'Connectivity and dependency checks passed',
        safetyChecks: ['Agent is connected'],
        verificationSteps: ['Confirm the workload is healthy'],
      },
    },
  };
}

function investigation(action?: PatrolActionReference): InvestigationSession {
  return {
    id: 'investigation-1',
    finding_id: 'finding-1',
    session_id: 'session-1',
    status: 'completed',
    started_at: '2026-07-10T10:00:00Z',
    turn_count: 2,
    outcome: 'fix_queued',
    action,
  };
}

function audit(
  stateValue: ActionAuditRecord['state'],
  verification = 'unknown',
): ActionAuditRecord {
  const action = actionReference(stateValue);
  return {
    id: action.action_id,
    createdAt: '2026-07-10T10:00:00Z',
    updatedAt: '2026-07-10T10:01:00Z',
    state: stateValue,
    request: {
      requestId: 'proposal-1',
      resourceId: action.resource_id,
      capabilityName: action.capability_name,
      reason: 'Recover the unhealthy workload',
      requestedBy: 'pulse_patrol',
    },
    plan: action.plan,
    origin: {
      surface: 'patrol',
      findingId: 'finding-1',
      investigationId: 'investigation-1',
      proposalId: 'proposal-1',
    },
    verificationOutcome: { status: verification as never },
  };
}

describe('ApprovalSection typed action lifecycle', () => {
  beforeEach(() => {
    state.hasAutoFix = true;
    getInvestigationMock.mockReset();
    decideActionMock.mockReset();
    executeActionMock.mockReset();
    loadFindingsMock.mockClear();
    notificationSuccessMock.mockReset();
    notificationWarningMock.mockReset();
    notificationErrorMock.mockReset();
    openMock.mockReset();
  });

  afterEach(() => cleanup());

  it('contains no live dependency on retired command-fix approval routes or raw commands', () => {
    expect(approvalSectionSource).toContain('@/api/resourceActions');
    expect(approvalSectionSource).not.toContain('reapproveInvestigationFix');
    expect(approvalSectionSource).not.toContain('approveInvestigationFix');
    expect(approvalSectionSource).not.toContain('denyInvestigationFix');
    expect(approvalSectionSource).not.toContain('approval.command');
    expect(approvalSectionSource).not.toContain('proposed_fix.commands');
  });

  it('approves and executes a pending typed action through the canonical lifecycle', async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('pending_approval')));
    decideActionMock.mockResolvedValue({
      actionId: 'act-1',
      state: 'approved',
      approval: {
        actor: 'operator',
        method: 'api',
        timestamp: '2026-07-10T10:01:00Z',
        outcome: 'approved',
      },
      audit: audit('approved'),
    });
    executeActionMock.mockResolvedValue({
      actionId: 'act-1',
      state: 'completed',
      audit: audit('completed', 'verified'),
      result: { success: true },
    });

    render(() => <ApprovalSection findingId="finding-1" investigationOutcome="fix_queued" />);
    fireEvent.click(await screen.findByRole('button', { name: /approve and run/i }));

    await waitFor(() => {
      expect(decideActionMock).toHaveBeenCalledWith(
        'act-1',
        'approved',
        'Approved from the Patrol action review',
      );
      expect(executeActionMock).toHaveBeenCalledWith(
        'act-1',
        'Operator requested execution from the Patrol action review',
      );
    });
    expect(notificationSuccessMock).toHaveBeenCalledWith('Action completed and verified');
  });

  it('rejects without executing', async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('pending_approval')));
    decideActionMock.mockResolvedValue({
      actionId: 'act-1',
      state: 'rejected',
      approval: {
        actor: 'operator',
        method: 'api',
        timestamp: '2026-07-10T10:01:00Z',
        outcome: 'rejected',
      },
      audit: audit('rejected'),
    });

    render(() => <ApprovalSection findingId="finding-1" investigationOutcome="fix_queued" />);
    fireEvent.click(await screen.findByRole('button', { name: /^reject$/i }));

    await waitFor(() =>
      expect(decideActionMock).toHaveBeenCalledWith(
        'act-1',
        'rejected',
        'Rejected from the Patrol action review',
      ),
    );
    expect(executeActionMock).not.toHaveBeenCalled();
  });

  it('runs a no-approval plan without fabricating a decision', async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('planned')));
    executeActionMock.mockResolvedValue({
      actionId: 'act-1',
      state: 'completed',
      audit: audit('completed', 'unverified'),
      result: { success: true },
    });

    render(() => <ApprovalSection findingId="finding-1" investigationOutcome="fix_queued" />);
    fireEvent.click(await screen.findByRole('button', { name: /run action/i }));

    await waitFor(() => expect(executeActionMock).toHaveBeenCalledTimes(1));
    expect(decideActionMock).not.toHaveBeenCalled();
    expect(notificationWarningMock).toHaveBeenCalledWith(
      'Action completed, but verification was inconclusive',
    );
  });

  it('fails closed when only a legacy investigation artifact remains', async () => {
    getInvestigationMock.mockResolvedValue({
      ...investigation(),
      proposed_fix: {
        id: 'legacy-fix',
        description: 'Legacy command-shaped history',
        commands: ['rm -rf /should-never-render'],
        destructive: true,
      },
      approval_id: 'legacy-approval',
    });

    render(() => <ApprovalSection findingId="finding-1" investigationOutcome="fix_queued" />);

    expect(await screen.findByText('Action details unavailable')).toBeInTheDocument();
    expect(screen.queryByText('rm -rf /should-never-render')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /approve/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /run action/i })).not.toBeInTheDocument();
  });
});
