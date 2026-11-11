import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { Suspense } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { InvestigationSession } from '@/api/ai';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { PatrolActionReference } from '@/types/actionAudit';
import ApprovalSection from '../ApprovalSection';

const getInvestigationMock = vi.hoisted(() => vi.fn());
const getResourceMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/ai', () => ({ AIAPI: { getInvestigation: getInvestigationMock } }));
vi.mock('@/api/resources', () => ({ ResourceAPI: { getResource: getResourceMock } }));

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
    getResourceMock.mockReset();
    // By default the registry cannot name the action's resource.
    getResourceMock.mockRejectedValue(new Error('not found'));
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
  });

  const renderSection = (
    investigationOutcome: string,
    hasAction = false,
    finding: {
      resourceId?: string;
      resourceName?: string;
      onReviewAction?: (actionId: string) => Promise<void> | void;
    } = {},
  ) =>
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <ApprovalSection
              findingId="finding-1"
              hasAction={hasAction}
              investigationOutcome={investigationOutcome}
              resourceId={finding.resourceId}
              resourceName={finding.resourceName}
              onReviewAction={finding.onReviewAction}
            />
          )}
        />
      </Router>
    ));

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

    expect(await screen.findByText('Loading the proposed action…')).toBeInTheDocument();
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
    // The section never decides or runs an action itself.
    expect(screen.queryByRole('button', { name: /^approve$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^reject$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /run/i })).not.toBeInTheDocument();
  });

  it('asks its host to open the shared review where the reader is', async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('pending_approval')));
    const onReviewAction = vi.fn().mockResolvedValue(undefined);

    renderSection('fix_queued', false, { onReviewAction });

    fireEvent.click(await screen.findByRole('button', { name: 'Review and approve' }));
    expect(onReviewAction).toHaveBeenCalledWith('act-1');
    // The link to the Actions page stays beside it.
    expect(screen.getByRole('link', { name: /review in actions/i })).toBeInTheDocument();
  });

  it('labels the button for an approved action as a run', async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('approved')));

    renderSection('fix_queued', false, { onReviewAction: vi.fn() });

    expect(await screen.findByRole('button', { name: 'Review and run' })).toBeInTheDocument();
    expect(screen.getByText('Nothing has changed yet. It is ready to run.')).toBeInTheDocument();
  });

  it('only links to Actions when no host can open the review', async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('pending_approval')));

    renderSection('fix_queued');

    await screen.findByRole('link', { name: /review in actions/i });
    expect(screen.queryByRole('button', { name: 'Review and approve' })).not.toBeInTheDocument();
  });

  it('says so when the action cannot be opened', async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('pending_approval')));
    const onReviewAction = vi.fn().mockRejectedValue(new Error('Pulse is restarting'));

    renderSection('fix_queued', false, { onReviewAction });

    fireEvent.click(await screen.findByRole('button', { name: 'Review and approve' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Pulse is restarting');
  });

  it('names the resource the action acts on, not the finding it came from', async () => {
    // The finding is on the app that broke; the fix starts its database.
    getInvestigationMock.mockResolvedValue(investigation(actionReference('pending_approval')));
    getResourceMock.mockResolvedValue({ id: 'vm:42', name: 'nextcloud-db', displayName: '' });

    renderSection('fix_queued', false, {
      resourceId: 'app-container-1',
      resourceName: 'nextcloud',
    });

    const target = await screen.findByTestId('patrol-action-target');
    await vi.waitFor(() => expect(target).toHaveTextContent('Restart nextcloud-db'));
    expect(target).not.toHaveTextContent('nextcloud nextcloud');
    expect(
      screen.getByText('Nothing has changed yet. It runs only after you approve it.'),
    ).toBeInTheDocument();
  });

  it("falls back to the finding's name only when the action is on the same resource", async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('pending_approval')));

    const same = renderSection('fix_queued', false, { resourceId: 'vm:42', resourceName: 'web' });
    expect(await screen.findByTestId('patrol-action-target')).toHaveTextContent('Restart web');
    same.unmount();
    resetCreateNonSuspendingQueryCacheForTest();

    renderSection('fix_queued', false, { resourceId: 'vm:7', resourceName: 'web' });
    expect(await screen.findByTestId('patrol-action-target')).toHaveTextContent('Restart vm:42');
  });

  it('offers no in-place review once the action has run', async () => {
    getInvestigationMock.mockResolvedValue(investigation(actionReference('completed')));

    renderSection('fix_verified', false, { onReviewAction: vi.fn() });

    await screen.findByRole('link', { name: /view outcome in actions/i });
    expect(screen.queryByRole('button', { name: /review and/i })).toBeNull();
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
