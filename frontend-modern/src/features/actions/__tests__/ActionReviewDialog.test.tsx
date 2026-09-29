import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { SecurityAPI } from '@/api/security';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import type { SecurityStatus } from '@/types/config';
import type { ActionAuditRecord, ActionDetailResponse } from '@/types/actionAudit';
import { ActionReviewDialog } from '../ActionReviewDialog';

vi.mock('@/api/resourceActions', () => ({
  ResourceActionsAPI: {
    getAction: vi.fn(),
    refreshAction: vi.fn(),
    decideAction: vi.fn(),
    executeAction: vi.fn(),
    forceFailAction: vi.fn(),
  },
}));
vi.mock('@/api/security', () => ({ SecurityAPI: { getStatus: vi.fn() } }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

beforeEach(() => {
  vi.mocked(SecurityAPI.getStatus).mockResolvedValue({
    hasAuthentication: true,
    requiresAuth: true,
    settingsCapabilities: { authenticationWrite: true },
  } as SecurityStatus);
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  syncSessionPresentationPolicy(null);
  vi.clearAllMocks();
});

const makeAudit = (
  status: 'resolved' | 'legacy_unknown',
  expiresAt: string,
): ActionAuditRecord => ({
  id: 'action-1',
  createdAt: '2026-07-12T00:00:00Z',
  updatedAt: '2026-07-12T00:00:00Z',
  state: 'pending_approval',
  decisionRevision: 0,
  request: {
    requestId: 'request-1',
    resourceId: 'docker:container:edge',
    capabilityName: 'restart',
    reason: 'Recover edge',
    requestedBy: 'operator',
  },
  plan: {
    actionId: 'action-1',
    requestId: 'request-1',
    allowed: true,
    requiresApproval: true,
    approvalPolicy: 'admin',
    approvalRequirement: { version: 1, floor: 'admin', quorum: 1, disallowRequester: false },
    rollbackAvailable: false,
    expiresAt,
    planHash: 'sha256:reviewed-plan',
    policyDecision:
      status === 'resolved'
        ? {
            version: 1,
            status,
            decisionId: 'decision-1',
            actionId: 'action-1',
            scope: {
              orgId: 'org-1',
              resourceId: 'docker:container:edge',
              capabilityName: 'restart',
            },
            authorities: [
              {
                kind: 'capability_registry',
                sourceId: 'capability-registry:restart',
                status: 'consulted',
                scope: {
                  orgId: 'org-1',
                  resourceId: 'docker:container:edge',
                  capabilityName: 'restart',
                },
                approvalFloor: 'admin',
                reasonCodes: ['capability_approval_admin'],
              },
            ],
            approvalRequirement: {
              version: 1,
              floor: 'admin',
              quorum: 1,
              disallowRequester: false,
            },
            planningAllowed: true,
            requiresApproval: true,
          }
        : {
            version: 0,
            status,
            scope: { orgId: '', resourceId: '', capabilityName: '' },
            authorities: [],
            approvalRequirement: {
              version: 0,
              floor: 'admin',
              quorum: 1,
              disallowRequester: false,
            },
            planningAllowed: false,
            requiresApproval: true,
          },
  },
  verificationOutcome: { status: 'unknown' },
});
const detail = (audit: ActionAuditRecord): ActionDetailResponse => ({
  audit,
  events: [],
  readiness: {
    ready: true,
    code: 'ready',
    message: 'Action is ready for approval and dispatch.',
    refreshable: false,
    checkedAt: '2026-07-12T00:00:00Z',
  },
});

const waitingDetail = (updatedAt = '2026-07-12T00:02:00Z'): ActionDetailResponse => {
  const current = detail(makeAudit('resolved', '2026-07-12T00:10:00Z'));
  current.audit.state = 'executing';
  current.attempt = {
    id: 'attempt-1',
    actionId: current.audit.id,
    state: 'receipt_pending',
    createdAt: '2026-07-12T00:01:00Z',
    updatedAt,
    dispatchCount: 1,
  };
  return current;
};

describe('ActionReviewDialog trust gates', () => {
  it('lets an admin close an aged receipt wait only after a direct-check reason and acknowledgement', async () => {
    const current = waitingDetail();
    const terminalAudit: ActionAuditRecord = {
      ...current.audit,
      state: 'failed',
      result: {
        success: false,
        actionResultV2: {
          version: 2,
          execution: { status: 'inconclusive', reasonCode: 'operator_force_failed' },
          verification: { status: 'inconclusive', evidenceClass: 'none' },
          compensation: { support: 'unavailable', status: 'not_attempted' },
        },
      },
    };
    const terminal = { ...current, audit: terminalAudit };
    vi.mocked(ResourceActionsAPI.getAction)
      .mockResolvedValueOnce(current)
      .mockResolvedValueOnce(terminal);
    vi.mocked(ResourceActionsAPI.forceFailAction).mockResolvedValue({
      actionId: terminalAudit.id,
      state: 'failed',
      audit: terminalAudit,
      result: terminalAudit.result,
    });
    const [selected, setSelected] = createSignal(current);
    const onChanged = vi.fn((next: ActionDetailResponse) => {
      setSelected(next);
    });
    render(() => (
      <ActionReviewDialog detail={selected()} onClose={vi.fn()} onChanged={onChanged} />
    ));

    fireEvent.click(await screen.findByRole('button', { name: 'Close stuck audit record…' }));
    expect(screen.getByText(/does not cancel the agent operation/)).toBeVisible();
    const close = screen.getByRole('button', { name: 'Close audit as inconclusive' });
    expect(close).toBeDisabled();
    fireEvent.input(screen.getByLabelText('What did you verify directly?'), {
      target: { value: 'Checked running container edge; image is already updated.' },
    });
    expect(close).toBeDisabled();
    fireEvent.click(
      screen.getByRole('checkbox', {
        name: /I checked the actual resource and understand/,
      }),
    );
    expect(close).toBeEnabled();
    fireEvent.click(close);

    await waitFor(() => {
      expect(ResourceActionsAPI.getAction).toHaveBeenCalledTimes(2);
      expect(ResourceActionsAPI.forceFailAction).toHaveBeenCalledWith(
        'action-1',
        'Checked running container edge; image is already updated.',
      );
      expect(onChanged).toHaveBeenCalledWith(terminal);
    });
    expect(screen.getByText('Outcome unknown')).toBeVisible();
    expect(screen.getByTestId('action-execution-truth')).toHaveTextContent('Inconclusive');
    expect(screen.getByText(/audit was closed without an agent receipt/)).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Close audit as inconclusive' })).toBeNull();
  });

  it('does not expose override for fresh, settled, read-only, or non-admin receipt records', async () => {
    const fresh = waitingDetail(new Date(Date.now() - 5 * 60 * 1000).toISOString());
    const freshView = render(() => <ActionReviewDialog detail={fresh} onClose={vi.fn()} />);
    await waitFor(() => expect(SecurityAPI.getStatus).toHaveBeenCalled());
    expect(screen.queryByTestId('action-stuck-recovery')).toBeNull();
    freshView.unmount();

    const settled = waitingDetail();
    settled.audit.state = 'failed';
    const settledView = render(() => <ActionReviewDialog detail={settled} onClose={vi.fn()} />);
    expect(screen.queryByTestId('action-stuck-recovery')).toBeNull();
    settledView.unmount();

    const readOnly = { ...waitingDetail(), readOnly: true };
    const readOnlyView = render(() => <ActionReviewDialog detail={readOnly} onClose={vi.fn()} />);
    expect(screen.queryByTestId('action-stuck-recovery')).toBeNull();
    readOnlyView.unmount();

    vi.mocked(SecurityAPI.getStatus).mockResolvedValue({
      hasAuthentication: true,
      requiresAuth: true,
      settingsCapabilities: { authenticationWrite: false },
    } as SecurityStatus);
    render(() => <ActionReviewDialog detail={waitingDetail()} onClose={vi.fn()} />);
    await waitFor(() => expect(SecurityAPI.getStatus).toHaveBeenCalled());
    expect(screen.queryByTestId('action-stuck-recovery')).toBeNull();
    expect(ResourceActionsAPI.forceFailAction).not.toHaveBeenCalled();
  });

  it('re-reads the receipt before mutation and refuses a stale override', async () => {
    const current = waitingDetail();
    const settled = { ...current, audit: { ...current.audit, state: 'completed' as const } };
    vi.mocked(ResourceActionsAPI.getAction).mockResolvedValueOnce(settled);
    const onChanged = vi.fn();
    render(() => <ActionReviewDialog detail={current} onClose={vi.fn()} onChanged={onChanged} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Close stuck audit record…' }));
    fireEvent.input(screen.getByLabelText('What did you verify directly?'), {
      target: { value: 'I checked the running container and its current image.' },
    });
    fireEvent.click(
      screen.getByRole('checkbox', {
        name: /I checked the actual resource and understand/,
      }),
    );
    fireEvent.click(screen.getByRole('button', { name: 'Close audit as inconclusive' }));
    await waitFor(() => expect(onChanged).toHaveBeenCalledWith(settled));
    expect(ResourceActionsAPI.forceFailAction).not.toHaveBeenCalled();
    expect(screen.getByRole('alert')).toHaveTextContent('No audit override was sent');
  });
  it('keeps a rejected action outcome visible without offering execution', () => {
    const audit = makeAudit('resolved', '2026-07-12T00:10:00Z');
    audit.state = 'rejected';
    render(() => <ActionReviewDialog detail={detail(audit)} onClose={vi.fn()} />);
    expect(screen.getByText('Rejected', { exact: true })).toBeVisible();
    expect(
      screen.queryByRole('button', { name: /approve|run|refresh plan/i }),
    ).not.toBeInTheDocument();
  });

  it('links a trusted Patrol action back to its exact operational record', () => {
    const audit = makeAudit('resolved', '2099-01-01T00:00:00Z');
    audit.origin = {
      surface: 'operational_trust_attention',
      operationalRecordId: 'record/one',
    };
    render(() => (
      <Router>
        <Route
          path="*"
          component={() => <ActionReviewDialog detail={detail(audit)} onClose={vi.fn()} />}
        />
      </Router>
    ));
    expect(screen.getByRole('link', { name: 'Open Patrol record' })).toHaveAttribute(
      'href',
      '/patrol?attention=record%2Fone',
    );
  });

  it('returns older Patrol actions to Patrol without claiming an exact record link', () => {
    const audit = makeAudit('resolved', '2099-01-01T00:00:00Z');
    audit.origin = { surface: 'operational_trust_attention' };
    render(() => (
      <Router>
        <Route
          path="*"
          component={() => <ActionReviewDialog detail={detail(audit)} onClose={vi.fn()} />}
        />
      </Router>
    ));
    expect(screen.getByRole('link', { name: 'Open Patrol' })).toHaveAttribute('href', '/patrol');
    expect(screen.queryByRole('link', { name: 'Open Patrol record' })).not.toBeInTheDocument();
  });

  it('does not treat another origin surface as Patrol even when it carries a record id', () => {
    const audit = makeAudit('resolved', '2099-01-01T00:00:00Z');
    audit.origin = {
      surface: 'pulse_assistant',
      operationalRecordId: 'record/one',
    };
    render(() => <ActionReviewDialog detail={detail(audit)} onClose={vi.fn()} />);
    expect(screen.queryByRole('link', { name: 'Open Patrol record' })).not.toBeInTheDocument();
  });

  it('offers no approve or run control for legacy provenance', () => {
    render(() => (
      <ActionReviewDialog
        detail={detail(makeAudit('legacy_unknown', '2099-01-01T00:00:00Z'))}
        onClose={vi.fn()}
      />
    ));
    expect(screen.getByTestId('action-review-invalid')).toHaveTextContent(
      'no current server policy provenance',
    );
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Run action' })).toBeNull();
  });

  it('removes decision controls when expiry passes while the dialog remains open', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-07-12T00:00:00Z'));
    render(() => (
      <ActionReviewDialog
        detail={detail(makeAudit('resolved', '2026-07-12T00:00:00.500Z'))}
        onClose={vi.fn()}
      />
    ));
    expect(screen.getByRole('button', { name: 'Approve' })).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(1000);
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
    expect(screen.getByTestId('action-review-invalid')).toHaveTextContent('review expired');
  });

  it('offers no decision or run control when a typed APT action carries parameters', () => {
    const audit = makeAudit('resolved', '2099-01-01T00:00:00Z');
    audit.request = {
      ...audit.request,
      capabilityName: 'install_os_updates',
      params: { package: 'curl' },
    };
    audit.plan.policyDecision!.scope.capabilityName = 'install_os_updates';
    render(() => <ActionReviewDialog detail={detail(audit)} onClose={vi.fn()} />);
    expect(screen.getByTestId('action-review-invalid')).toHaveTextContent(
      'unexpected operator-selected parameters',
    );
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Run action' })).toBeNull();
  });

  it('blocks approval with the exact live-readiness unblock while keeping rejection available', () => {
    const current = detail(makeAudit('resolved', '2099-01-01T00:00:00Z'));
    current.readiness = {
      ready: false,
      code: 'command_agent_disconnected',
      message: 'Connect the command agent for web-42.',
      remediation: 'Reconnect the agent, then refresh this check.',
      refreshable: false,
      checkedAt: '2026-07-12T00:00:00Z',
    };
    render(() => <ActionReviewDialog detail={current} onClose={vi.fn()} />);
    expect(screen.getByTestId('action-review-invalid')).toHaveTextContent(
      'Connect the command agent for web-42. Reconnect the agent, then refresh this check.',
    );
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Reject' })).toBeInTheDocument();
  });

  it('refreshes a drifted plan and hands the replacement back for review', async () => {
    const current = detail(makeAudit('resolved', '2099-01-01T00:00:00Z'));
    current.readiness = {
      ready: false,
      code: 'action_plan_drift',
      message: 'The resource changed after this plan was created.',
      remediation: 'Refresh the plan and review the replacement.',
      refreshable: true,
      checkedAt: '2026-07-12T00:00:00Z',
    };
    const replacementAudit = { ...current.audit, id: 'action-2' };
    replacementAudit.plan = {
      ...current.audit.plan,
      actionId: 'action-2',
      planHash: 'sha256:replacement',
    };
    const replacement = detail(replacementAudit);
    vi.mocked(ResourceActionsAPI.refreshAction).mockResolvedValue(replacement);
    const onChanged = vi.fn();
    render(() => <ActionReviewDialog detail={current} onClose={vi.fn()} onChanged={onChanged} />);
    fireEvent.click(screen.getByRole('button', { name: 'Refresh plan' }));
    await waitFor(() => {
      expect(ResourceActionsAPI.refreshAction).toHaveBeenCalledWith(
        'action-1',
        'sha256:reviewed-plan',
      );
      expect(onChanged).toHaveBeenCalledWith(replacement);
    });
  });

  it('keeps mock and other read-only sessions inspectable without mutation controls', () => {
    syncSessionPresentationPolicy({
      presentationPolicy: {
        demoMode: true,
        readOnly: true,
        hideCommercial: true,
        hideUpgrade: true,
      },
    });
    render(() => (
      <ActionReviewDialog
        detail={detail(makeAudit('resolved', '2099-01-01T00:00:00Z'))}
        onClose={vi.fn()}
      />
    ));
    expect(screen.getByTestId('action-review-invalid')).toHaveTextContent('session is read-only');
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Run action' })).toBeNull();
  });

  it('honors the action API read-only projection when the broader session remains writable', () => {
    const readOnlyDetail = {
      ...detail(makeAudit('resolved', '2099-01-01T00:00:00Z')),
      readOnly: true,
    };
    render(() => <ActionReviewDialog detail={readOnlyDetail} onClose={vi.fn()} />);
    expect(screen.getByTestId('action-review-invalid')).toHaveTextContent('session is read-only');
    expect(screen.queryByRole('button', { name: 'Reject' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
  });

  it('does not describe a settled historical action as an expired review', () => {
    const audit = makeAudit('resolved', '2026-07-12T00:10:00Z');
    audit.state = 'completed';
    render(() => <ActionReviewDialog detail={detail(audit)} onClose={vi.fn()} />);
    expect(screen.queryByTestId('action-review-invalid')).toBeNull();
  });

  it('binds an approval to the plan shown in the dialog', async () => {
    const audit = makeAudit('resolved', '2099-01-01T00:00:00Z');
    vi.mocked(ResourceActionsAPI.decideAction).mockResolvedValue({
      actionId: audit.id,
      state: 'approved',
      approval: {
        actor: 'operator',
        method: 'api',
        timestamp: audit.updatedAt,
        outcome: 'approved',
      },
      audit: { ...audit, state: 'approved' },
    });
    vi.mocked(ResourceActionsAPI.getAction).mockResolvedValue(
      detail({ ...audit, state: 'approved' }),
    );
    render(() => <ActionReviewDialog detail={detail(audit)} onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }));
    await waitFor(() =>
      expect(ResourceActionsAPI.decideAction).toHaveBeenCalledWith(
        'action-1',
        'approved',
        'sha256:reviewed-plan',
        'Operator approved from Actions review.',
      ),
    );
  });

  it('collapses low-risk capabilities to a single Approve and run confirmation', async () => {
    const audit = makeAudit('resolved', '2099-01-01T00:00:00Z');
    audit.capabilityAutoAuthorization = 'low_risk';
    vi.mocked(ResourceActionsAPI.decideAction).mockResolvedValue({
      actionId: audit.id,
      state: 'approved',
      approval: {
        actor: 'operator',
        method: 'api',
        timestamp: audit.updatedAt,
        outcome: 'approved',
      },
      audit: { ...audit, state: 'approved' },
    });
    vi.mocked(ResourceActionsAPI.executeAction).mockResolvedValue({
      actionId: audit.id,
      state: 'executing',
      audit: { ...audit, state: 'executing' },
    });
    vi.mocked(ResourceActionsAPI.getAction).mockResolvedValue(
      detail({ ...audit, state: 'executing' }),
    );
    render(() => <ActionReviewDialog detail={detail(audit)} onClose={vi.fn()} />);
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Reject' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Approve and run' }));
    await waitFor(() => {
      expect(ResourceActionsAPI.decideAction).toHaveBeenCalledWith(
        'action-1',
        'approved',
        'sha256:reviewed-plan',
        'Operator approved from Actions review.',
      );
      expect(ResourceActionsAPI.executeAction).toHaveBeenCalledWith(
        'action-1',
        'sha256:reviewed-plan',
        'Operator confirmed execution from Actions review.',
      );
    });
  });

  it('keeps the two-phase Approve for capabilities without the low-risk class', () => {
    const audit = makeAudit('resolved', '2099-01-01T00:00:00Z');
    render(() => <ActionReviewDialog detail={detail(audit)} onClose={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Approve' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Approve and run' })).toBeNull();
  });

  it('offers no action controls when the reviewed plan identity is missing', () => {
    const audit = makeAudit('resolved', '2099-01-01T00:00:00Z');
    delete audit.plan.planHash;
    render(() => <ActionReviewDialog detail={detail(audit)} onClose={vi.fn()} />);
    expect(screen.getByTestId('action-review-invalid')).toHaveTextContent(
      'no reviewed plan identity',
    );
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Run action' })).toBeNull();
  });
});
