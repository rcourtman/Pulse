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
const openWithPromptMock = vi.hoisted(() => vi.fn());

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
    openWithPrompt: (...args: unknown[]) => openWithPromptMock(...args),
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
    openWithPromptMock.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('keeps fix approvals out of commercial trial prompts', () => {
    expect(approvalSectionSource).not.toContain('canStartCommercialTrial');
    expect(approvalSectionSource).not.toContain('runStartProTrialAction');
    expect(approvalSectionSource).not.toContain('start a free 14-day trial');
    expect(approvalSectionSource).not.toContain('commercialPosture');
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

    expect(openWithPromptMock).toHaveBeenCalledWith(
      expect.stringContaining('Patrol queued a governed fix for CPU saturation on node-1'),
      {
        targetType: 'host',
        targetId: 'host-1',
        findingId: 'finding-1',
        briefing: expect.objectContaining({
          sourceLabel: 'Pulse Patrol',
          title: 'Operator briefing attached',
          subject: 'CPU saturation on node-1',
          statusLabel: 'Fix Queued',
          detailLines: expect.arrayContaining([
            expect.stringContaining('fix queued for governed review'),
            expect.stringContaining('Recover or regenerate the governed approval before execution'),
          ]),
          suggestedPrompts: [
            'Review approval risk and next step',
            'Explain current finding status',
            'List approval prerequisites before action',
          ],
        }),
        autonomousMode: false,
      },
    );
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

    expect(openWithPromptMock).toHaveBeenCalledTimes(1);
    const [prompt, context] = openWithPromptMock.mock.calls[0];
    expect(prompt).toContain('queued a governed fix for review');
    expect(prompt).toContain('**Approval:** approval-1');
    expect(prompt).toContain('**Approval status:** pending');
    expect(prompt).toContain('**Risk level:** high');
    expect(prompt).not.toContain('systemctl restart nginx');
    expect(prompt).not.toContain('Please execute this fix');
    expect(context).toEqual({
      targetType: 'agent',
      targetId: 'agent-1',
      findingId: 'finding-1',
      briefing: expect.objectContaining({
        sourceLabel: 'Pulse Patrol',
        title: 'Operator briefing attached',
        subject: 'CPU saturation on node-1',
        statusLabel: 'Pending approval · High risk',
        detailLines: expect.arrayContaining([
          expect.stringContaining('live approval pending'),
          expect.stringContaining('Review live governed approval approval-1 before execution'),
        ]),
        actionLabel: 'Approval approval-1',
        safetyNote: 'Execution requires the governed approval flow.',
      }),
      autonomousMode: false,
    });
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

    expect(openWithPromptMock).toHaveBeenCalledTimes(1);
    const [prompt, context] = openWithPromptMock.mock.calls[0];
    expect(prompt).toContain('queued a governed fix for review');
    expect(prompt).toContain('**Proposed fix:** Restart the workload service');
    expect(prompt).toContain('**Target:** node-1');
    expect(prompt).toContain('**Risk level:** high');
    expect(prompt).not.toContain('systemctl restart nginx');
    expect(context).toEqual({
      targetType: 'agent',
      targetId: 'agent-1',
      findingId: 'finding-1',
      briefing: expect.objectContaining({
        sourceLabel: 'Pulse Patrol',
        title: 'Operator briefing attached',
        subject: 'CPU saturation on node-1',
        statusLabel: 'Fix Queued',
        detailLines: expect.arrayContaining([
          expect.stringContaining('fix queued for governed review'),
          expect.stringContaining('Proposed fix: Restart the workload service'),
          expect.stringContaining('Recover or regenerate the governed approval before execution'),
        ]),
        actionLabel: 'Restart the workload service',
        commandSummary: '1 command recorded for approval context',
        safetyNote:
          'Command details stay in approval context; destructive actions require governed approval.',
        suggestedPrompts: [
          'Review approval risk and next step',
          'Explain current finding status',
          'Summarize remediation without command text',
        ],
      }),
      autonomousMode: false,
    });
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

    expect(await screen.findAllByRole('button', { name: /re-approve & execute/i })).toHaveLength(1);
    fireEvent.click(screen.getByRole('button', { name: /re-approve & execute/i }));

    await waitFor(() => {
      expect(reapproveInvestigationFixMock).toHaveBeenCalledWith('finding-2');
      expect(approveInvestigationFixMock).toHaveBeenCalledWith('approval-2');
    });
    expect(notificationSuccessMock).toHaveBeenCalledWith('Fix executed successfully');
  });
});
