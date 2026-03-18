import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ApprovalExecutionResult, ApprovalRequest, InvestigationSession } from '@/api/ai';
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
const startProTrialMock = vi.hoisted(() => vi.fn());

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
  licenseStatus: () => state.entitlements,
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
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
    startProTrialMock.mockReset();
  });

  afterEach(() => {
    cleanup();
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
      expect.stringContaining('Patrol queued a fix for a finding'),
      {
        targetType: 'host',
        targetId: 'host-1',
        findingId: 'finding-1',
      },
    );
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
