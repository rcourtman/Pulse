import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ApprovalExecutionResult, ApprovalRequest, InvestigationSession } from '@/api/ai';
import approvalSectionSource from '../ApprovalSection.tsx?raw';
import ApprovalSection from '../ApprovalSection';

const state = vi.hoisted(() => ({
  pendingApprovals: [] as ApprovalRequest[],
  hasAutoFix: false,
  entitlements: {
    subscription_state: 'expired',
    trial_eligible: false,
  } as { subscription_state: string; trial_eligible?: boolean } | null,
}));

const getInvestigationMock = vi.hoisted(() => vi.fn<() => Promise<InvestigationSession | null>>());
const reapproveInvestigationFixMock = vi.hoisted(() =>
  vi.fn<() => Promise<{ approval_id: string; message: string }>>(),
);
const approveInvestigationFixMock = vi.hoisted(() => vi.fn());
const denyInvestigationFixMock = vi.hoisted(() => vi.fn());
const notificationSuccessMock = vi.hoisted(() => vi.fn());
const notificationErrorMock = vi.hoisted(() => vi.fn());
const openMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getInvestigation: getInvestigationMock,
    reapproveInvestigationFix: reapproveInvestigationFixMock,
  },
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    get pendingApprovals() {
      return state.pendingApprovals;
    },
    get patrolPendingApprovals() {
      return state.pendingApprovals;
    },
    approveInvestigationFix: (...args: unknown[]) => approveInvestigationFixMock(...args),
    denyInvestigationFix: (...args: unknown[]) => denyInvestigationFixMock(...args),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

vi.mock('@/stores/aiChat', () => ({
  aiChatStore: {
    open: (...args: unknown[]) => openMock(...args),
  },
}));

vi.mock('@/stores/license', () => ({
  hasFeature: (feature: string) => feature === 'ai_autofix' && state.hasAutoFix,
}));

vi.mock('@/stores/licenseCommercial', () => ({
  canStartCommercialTrial: () => false,
}));

vi.mock('../RemediationStatus', () => ({
  RemediationStatus: (props: { result: ApprovalExecutionResult }) => (
    <div>{props.result.message}</div>
  ),
}));

describe('ApprovalSection', () => {
  beforeEach(() => {
    state.pendingApprovals = [];
    state.hasAutoFix = false;
    state.entitlements = {
      subscription_state: 'expired',
      trial_eligible: false,
    };

    getInvestigationMock.mockReset();
    reapproveInvestigationFixMock.mockReset();
    approveInvestigationFixMock.mockReset();
    denyInvestigationFixMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    openMock.mockReset();
  });

  it('keeps approval action loading indicators on the shared LoadingSpinner primitive', () => {
    expect(approvalSectionSource).toContain('LoadingSpinner');
  });

  it('keeps approval action controls on the shared Button primitive', () => {
    expect(approvalSectionSource).toContain('@/components/shared/Button');
    expect(approvalSectionSource).toContain('<Button');
    expect(approvalSectionSource).toContain('variant="success"');
    expect(approvalSectionSource).toContain('variant="warningSolid"');
    expect(approvalSectionSource).toContain('variant="primary"');
    expect(approvalSectionSource).toContain('variant="ghost"');
    expect(approvalSectionSource).not.toContain('px-3 py-1.5 bg-green-600 hover:bg-green-700');
    expect(approvalSectionSource).not.toContain('px-3 py-1.5 bg-amber-600 hover:bg-amber-700');
    expect(approvalSectionSource).not.toContain('px-3 py-1.5 bg-blue-600 hover:bg-blue-700');
    expect(approvalSectionSource).not.toContain(
      'px-3 py-1.5 hover:bg-surface-hover disabled:opacity-50 text-muted',
    );
  });

  it('demotes raw commands behind technical details instead of making them the approval body', () => {
    expect(approvalSectionSource).toContain('renderTechnicalCommandDetails');
    expect(approvalSectionSource).toContain('Technical details');
    expect(approvalSectionSource).toContain('{renderTechnicalCommandDetails(approval.command)}');
    expect(approvalSectionSource).toContain('{renderTechnicalCommandDetails(fix.commands)}');
    expect(approvalSectionSource).toContain('{renderTechnicalCommandDetails(fix().commands)}');
    expect(approvalSectionSource).not.toContain('{approval.command}</div>');
    expect(approvalSectionSource).not.toContain('{fix.commands![0]}</div>');
    expect(approvalSectionSource).not.toContain('{fix().commands![0]}</div>');
  });

  afterEach(() => {
    cleanup();
  });

  it('uses governed decision wording for live approvals', async () => {
    state.hasAutoFix = true;
    state.pendingApprovals = [
      {
        id: 'approval-1',
        toolId: 'investigation_fix',
        command: 'systemctl restart nginx',
        targetType: 'investigation',
        targetId: 'finding-1',
        targetName: 'node-1',
        context: 'Restart the workload service',
        riskLevel: 'high',
        status: 'pending',
        requestedAt: new Date().toISOString(),
        expiresAt: new Date(Date.now() + 5 * 60_000).toISOString(),
      },
    ];

    render(() => <ApprovalSection findingId="finding-1" investigationOutcome="fix_queued" />);

    expect(await screen.findByRole('button', { name: /approve fix/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /reject/i })).toBeInTheDocument();
    expect(screen.queryByText(/approve & execute/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/^deny$/i)).not.toBeInTheDocument();
  });

  it('keeps fix approvals out of commercial trial prompts', () => {
    expect(approvalSectionSource).not.toContain('canStartCommercialTrial');
    expect(approvalSectionSource).not.toContain('runStartProTrialAction');
    expect(approvalSectionSource).not.toContain('start a free 14-day trial');
    expect(approvalSectionSource).not.toContain('commercialPosture');
  });

  it('keeps approval metadata chips on the shared MetadataBadge primitive', () => {
    expect(approvalSectionSource).toContain('MetadataBadge');
    expect(approvalSectionSource).toContain('APPROVAL_SECTION_BADGE_PROPS');
    expect(approvalSectionSource).toContain('approvalRisk.badgeTone');
    expect(approvalSectionSource).toContain('fixRisk.badgeTone');
    expect(approvalSectionSource).not.toContain('approvalRisk.badgeClass');
    expect(approvalSectionSource).not.toContain('fixRisk.badgeClass');
    expect(approvalSectionSource).not.toMatch(/px-1\.5 py-0\.5 text-\[10px\] font-medium rounded/);
  });

  it('keeps fix_queued findings actionable when approval and investigation details are unavailable', async () => {
    getInvestigationMock.mockResolvedValue(null);

    render(() => (
      <ApprovalSection
        findingId="finding-1"
        investigationOutcome="fix_queued"
        findingTitle="CPU saturation"
        resourceName="node-1"
        resourceType="host"
        resourceId="host-1"
      />
    ));

    expect(await screen.findByText('Fix Pending Approval')).toBeInTheDocument();
    expect(screen.getAllByText('Fix Pending Approval')).toHaveLength(1);
    expect(screen.getByText('details unavailable')).toBeInTheDocument();
    expect(
      screen.getByText(/original approval details are no longer available/i),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /discuss with assistant/i }));

    expect(openMock).toHaveBeenCalledTimes(1);
    const [context] = openMock.mock.calls[0];
    expect(context).toEqual(
      expect.objectContaining({
        targetType: 'host',
        targetId: 'host-1',
        findingId: 'finding-1',
        briefing: expect.objectContaining({
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol finding attached',
          subject: 'CPU saturation on node-1',
          statusLabel: expect.stringContaining('Fix Queued'),
        }),
        autonomousMode: false,
        handoffResources: [{ id: 'host-1', name: 'node-1', node: undefined, type: 'host' }],
        handoffActions: undefined,
        context: expect.objectContaining({
          source: 'pulse-patrol-finding',
          findingId: 'finding-1',
          resourceId: 'host-1',
          resourceName: 'node-1',
          resourceType: 'host',
          actionReferenceCount: 0,
        }),
      }),
    );
    expect(context.handoffContext).toContain('[Patrol Finding Context]');
    expect(context.handoffContext).toContain('Finding ID: finding-1');
    expect(context.handoffContext).toContain(
      'Recover or regenerate the governed approval before execution',
    );
    expect(context.handoffContext).toContain('Command Boundary:');
  });

  it('opens Assistant from a pending Patrol approval without carrying raw command text', async () => {
    state.pendingApprovals = [
      {
        id: 'approval-1',
        toolId: 'investigation_fix',
        command: 'systemctl restart nginx',
        targetType: 'investigation',
        targetId: 'finding-1',
        targetName: 'node-1',
        context: 'Restart the workload service',
        riskLevel: 'high',
        status: 'pending',
        requestedAt: new Date().toISOString(),
        expiresAt: new Date(Date.now() + 5 * 60_000).toISOString(),
      },
    ];

    render(() => (
      <ApprovalSection
        findingId="finding-1"
        investigationOutcome="fix_queued"
        findingTitle="CPU saturation"
        resourceName="node-1"
        resourceType="agent"
        resourceId="agent-1"
      />
    ));

    fireEvent.click(await screen.findByRole('button', { name: /fix with assistant/i }));

    expect(openMock).toHaveBeenCalledTimes(1);
    const [context] = openMock.mock.calls[0];
    expect(JSON.stringify(context)).not.toContain('systemctl restart nginx');
    expect(JSON.stringify(context)).not.toContain('Please execute this fix');
    expect(context).toEqual(
      expect.objectContaining({
        targetType: 'agent',
        targetId: 'agent-1',
        findingId: 'finding-1',
        briefing: expect.objectContaining({
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol finding attached',
          subject: 'CPU saturation on node-1',
          statusLabel: expect.stringContaining('Pending'),
          commandSummary: '1 command recorded for approval context',
          safetyNote: expect.stringContaining('Command details stay in approval context'),
        }),
        autonomousMode: false,
        handoffResources: [{ id: 'agent-1', name: 'node-1', node: undefined, type: 'agent' }],
        handoffActions: [
          expect.objectContaining({
            findingId: 'finding-1',
            approvalId: 'approval-1',
            approvalStatus: 'pending',
            actionRequiresApproval: true,
            description: 'Restart the workload service',
            riskLevel: 'high',
            destructive: false,
            targetHost: 'node-1',
            targetResourceId: 'agent-1',
            targetResourceName: 'node-1',
            targetResourceType: 'agent',
          }),
        ],
        context: expect.objectContaining({
          source: 'pulse-patrol-finding',
          findingId: 'finding-1',
          resourceId: 'agent-1',
          resourceName: 'node-1',
          resourceType: 'agent',
          pendingApprovalId: 'approval-1',
          actionReferenceCount: 1,
        }),
      }),
    );
    expect(context.handoffContext).toContain('[Patrol Finding Context]');
    expect(context.handoffContext).toContain('Approval: approval-1');
    expect(context.handoffContext).toContain('Approval Status: pending');
    expect(context.handoffContext).toContain('Command Boundary:');
    expect(JSON.stringify(context.handoffActions)).not.toContain('systemctl restart nginx');
    expect(JSON.stringify(context.briefing)).not.toContain('systemctl restart nginx');
  });

  it('opens Assistant from an expired approval with safe proposed-fix briefing metadata', async () => {
    getInvestigationMock.mockResolvedValue({
      id: 'session-1',
      finding_id: 'finding-1',
      session_id: 'session-1',
      status: 'completed',
      started_at: '2026-05-06T12:00:00Z',
      turn_count: 1,
      outcome: 'fix_queued',
      proposed_fix: {
        id: 'fix-1',
        description: 'Restart the workload service',
        commands: ['systemctl restart nginx'],
        risk_level: 'high',
        destructive: true,
        target_host: 'node-1',
        rationale: 'Service is wedged after IO pressure.',
      },
    });

    render(() => (
      <ApprovalSection
        findingId="finding-1"
        investigationOutcome="fix_queued"
        findingTitle="CPU saturation"
        resourceName="node-1"
        resourceType="agent"
        resourceId="agent-1"
      />
    ));

    expect(await screen.findByText('approval expired')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /fix with assistant/i }));

    expect(openMock).toHaveBeenCalledTimes(1);
    const [context] = openMock.mock.calls[0];
    expect(JSON.stringify(context)).not.toContain('systemctl restart nginx');
    expect(context).toEqual(
      expect.objectContaining({
        targetType: 'agent',
        targetId: 'agent-1',
        findingId: 'finding-1',
        briefing: expect.objectContaining({
          sourceLabel: 'Pulse Patrol',
          title: 'Patrol finding attached',
          subject: 'CPU saturation on node-1',
          statusLabel: expect.stringContaining('Fix Queued'),
          commandSummary: '1 command recorded for approval context',
          safetyNote: expect.stringContaining('destructive actions require governed approval'),
        }),
        autonomousMode: false,
        handoffResources: [{ id: 'agent-1', name: 'node-1', node: undefined, type: 'agent' }],
        handoffActions: [
          expect.objectContaining({
            findingId: 'finding-1',
            approvalId: undefined,
            actionRequiresApproval: false,
            description: 'Restart the workload service',
            riskLevel: 'high',
            destructive: true,
            targetHost: 'node-1',
            targetResourceId: 'agent-1',
            targetResourceName: 'node-1',
            targetResourceType: 'agent',
          }),
        ],
        context: expect.objectContaining({
          source: 'pulse-patrol-finding',
          findingId: 'finding-1',
          resourceId: 'agent-1',
          resourceName: 'node-1',
          resourceType: 'agent',
          pendingApprovalId: undefined,
          actionReferenceCount: 1,
        }),
      }),
    );
    expect(context.handoffContext).toContain('[Patrol Finding Context]');
    expect(context.handoffContext).toContain('Existing Action Artifact:');
    expect(context.handoffContext).toContain('Command Boundary:');
    expect(JSON.stringify(context.handoffActions)).not.toContain('systemctl restart nginx');
    expect(JSON.stringify(context.briefing)).not.toContain('systemctl restart nginx');
  });

  it('recreates and executes a queued fix when autofix is available', async () => {
    state.hasAutoFix = true;
    getInvestigationMock.mockResolvedValue(null);
    reapproveInvestigationFixMock.mockResolvedValue({
      approval_id: 'approval-2',
      message: 'Approval recreated',
    });
    approveInvestigationFixMock.mockResolvedValue({
      approved: true,
      executed: true,
      success: true,
      output: 'ok',
      exit_code: 0,
      finding_id: 'finding-2',
      message: 'Fix executed successfully',
    } satisfies ApprovalExecutionResult);

    render(() => <ApprovalSection findingId="finding-2" investigationOutcome="fix_queued" />);

    expect(await screen.findAllByRole('button', { name: /re-approve fix/i })).toHaveLength(1);
    fireEvent.click(screen.getByRole('button', { name: /re-approve fix/i }));

    await waitFor(() => {
      expect(reapproveInvestigationFixMock).toHaveBeenCalledWith('finding-2');
      expect(approveInvestigationFixMock).toHaveBeenCalledWith('approval-2');
    });
    expect(notificationSuccessMock).toHaveBeenCalledWith('Fix executed successfully');
  });
});
