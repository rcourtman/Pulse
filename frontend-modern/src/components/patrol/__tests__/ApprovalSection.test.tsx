import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { Suspense } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { InvestigationSession } from '@/api/ai';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { PatrolActionReference } from '@/types/actionAudit';
import approvalSectionSource from '../ApprovalSection.tsx?raw';
import ApprovalSection from '../ApprovalSection';

const getInvestigationMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/ai', () => ({ AIAPI: { getInvestigation: getInvestigationMock } }));

function actionReference(state: PatrolActionReference['state']): PatrolActionReference {
  return {
    action_id: 'act-1',
    proposal_id: 'proposal-1',
    resource_id: 'vm:42',
    capability_name: 'restart',
    state,
    plan: {
      actionId: 'act-1',
      requestId: 'proposal-1',
      allowed: true,
      requiresApproval: state === 'pending_approval',
      approvalPolicy: state === 'pending_approval' ? 'admin' : 'none',
      rollbackAvailable: true,
      planHash: 'sha256:reviewed-plan',
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
    outcome: action?.state === 'completed' ? 'fix_verified' : 'fix_queued',
    action,
  };
}

describe('ApprovalSection typed action handoff', () => {
  beforeEach(() => {
    resetCreateNonSuspendingQueryCacheForTest();
    getInvestigationMock.mockReset();
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
  });

  const renderSection = (investigationOutcome: string, hasAction = false) =>
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <ApprovalSection
              findingId="finding-1"
              hasAction={hasAction}
              investigationOutcome={investigationOutcome}
            />
          )}
        />
      </Router>
    ));

  it('keeps Patrol contextual while routing mutations to the canonical Actions review', () => {
    expect(approvalSectionSource).toContain('@/features/actions/actionRouting');
    expect(approvalSectionSource).toContain('buildActionReviewPath');
    expect(approvalSectionSource).not.toContain('@/api/resourceActions');
    expect(approvalSectionSource).not.toContain('decideAction');
    expect(approvalSectionSource).not.toContain('executeAction');
    expect(approvalSectionSource).not.toContain('approveInvestigationFix');
    expect(approvalSectionSource).not.toContain('proposed_fix.commands');
  });

  it('reads the governed action through the non-suspending query', () => {
    // The nearest Suspense boundary above a review is the app's route
    // boundary, whose fallback replaces the page and loses the reader's place.
    expect(approvalSectionSource).not.toContain('createResource');
    expect(approvalSectionSource).toContain('createNonSuspendingQuery');
  });

  it('keeps the page around a finding review while the governed action loads', async () => {
    let finishRead: (value: InvestigationSession) => void = () => {};
    getInvestigationMock.mockReturnValue(
      new Promise<InvestigationSession>((resolve) => {
        finishRead = resolve;
      }),
    );

    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <Suspense fallback={<div>Loading view...</div>}>
              <ApprovalSection findingId="finding-1" investigationOutcome="fix_queued" />
            </Suspense>
          )}
        />
      </Router>
    ));

    expect(await screen.findByText('Loading governed action…')).toBeInTheDocument();
    expect(screen.queryByText('Loading view...')).not.toBeInTheDocument();

    finishRead(investigation(actionReference('pending_approval')));
    expect(await screen.findByRole('link', { name: /review in actions/i })).toBeInTheDocument();
    expect(screen.queryByText('Loading view...')).not.toBeInTheDocument();
  });

  it('reads the action again on its own after a failed read', async () => {
    // Findings refreshes no longer remount the review, so it retries itself.
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    try {
      getInvestigationMock.mockRejectedValueOnce(new Error('Pulse is restarting'));
      getInvestigationMock.mockResolvedValueOnce(
        investigation(actionReference('pending_approval')),
      );

      renderSection('fix_queued');
      expect(await screen.findByText('Action details unavailable')).toBeInTheDocument();

      vi.advanceTimersByTime(30_000);
      expect(await screen.findByRole('link', { name: /review in actions/i })).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it('deep-links a pending action to its exact governed review', async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('pending_approval')));

    renderSection('fix_queued');

    const link = await screen.findByRole('link', { name: /review in actions/i });
    expect(link).toHaveAttribute('href', '/actions?action=act-1');
    expect(screen.getByText('Approval required')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /approve/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^reject$/i })).not.toBeInTheDocument();
  });

  it('routes terminal action history to the exact recorded outcome', async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('completed')));

    renderSection('fix_verified');

    expect(await screen.findByRole('link', { name: /view outcome in actions/i })).toHaveAttribute(
      'href',
      '/actions?action=act-1',
    );
    expect(screen.getByText('Outcome verified')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /discuss with assistant/i })).toBeNull();
  });

  it('keeps missing plan identity visible while leaving replan guidance to Actions', async () => {
    const action = actionReference('pending_approval');
    delete action.plan.planHash;
    getInvestigationMock.mockResolvedValue(investigation(action));

    renderSection('fix_queued');

    expect(await screen.findByRole('alert')).toHaveTextContent('no reviewed plan identity');
    expect(screen.getByRole('link', { name: /review in actions/i })).toHaveAttribute(
      'href',
      '/actions?action=act-1',
    );
  });

  it('retains an expired action when the issue needs attention', async () => {
    getInvestigationMock.mockResolvedValue({
      ...investigation(actionReference('expired')),
      outcome: 'needs_attention',
    });

    renderSection('needs_attention', true);

    expect(await screen.findByRole('link', { name: /view outcome in actions/i })).toHaveAttribute(
      'href',
      '/actions?action=act-1',
    );
    expect(screen.getByText('Expired')).toBeInTheDocument();
    expect(screen.queryByText('Outcome verified')).not.toBeInTheDocument();
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

    renderSection('fix_queued');

    expect(await screen.findByText('Action details unavailable')).toBeInTheDocument();
    expect(screen.queryByText('rm -rf /should-never-render')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /actions/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button')).toBeNull();
  });
});
