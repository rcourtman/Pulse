import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import type { ActionAuditRecord, ActionDetailResponse } from '@/types/actionAudit';
import { ActionReviewDialog } from '../ActionReviewDialog';

vi.mock('@/api/resourceActions', () => ({
  ResourceActionsAPI: { getAction: vi.fn(), decideAction: vi.fn(), executeAction: vi.fn() },
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  syncSessionPresentationPolicy(null);
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
const detail = (audit: ActionAuditRecord): ActionDetailResponse => ({ audit, events: [] });

describe('ActionReviewDialog trust gates', () => {
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
