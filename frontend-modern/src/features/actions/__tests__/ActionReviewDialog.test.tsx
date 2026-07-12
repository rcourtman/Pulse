import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ActionAuditRecord, ActionDetailResponse } from '@/types/actionAudit';
import { ActionReviewDialog } from '../ActionReviewDialog';

vi.mock('@/api/resourceActions', () => ({ ResourceActionsAPI: { getAction: vi.fn(), decideAction: vi.fn(), executeAction: vi.fn() } }));
vi.mock('@/stores/notifications', () => ({ notificationStore: { success: vi.fn(), error: vi.fn(), warning: vi.fn() } }));

afterEach(() => { cleanup(); vi.useRealTimers(); });

const makeAudit = (status: 'resolved' | 'legacy_unknown', expiresAt: string): ActionAuditRecord => ({
  id: 'action-1', createdAt: '2026-07-12T00:00:00Z', updatedAt: '2026-07-12T00:00:00Z', state: 'pending_approval', decisionRevision: 0,
  request: { requestId: 'request-1', resourceId: 'docker:container:edge', capabilityName: 'restart', reason: 'Recover edge', requestedBy: 'operator' },
  plan: {
    actionId: 'action-1', requestId: 'request-1', allowed: true, requiresApproval: true, approvalPolicy: 'admin', approvalRequirement: { version: 1, floor: 'admin', quorum: 1, disallowRequester: false }, rollbackAvailable: false, expiresAt,
    policyDecision: status === 'resolved'
      ? { version: 1, status, decisionId: 'decision-1', actionId: 'action-1', scope: { orgId: 'org-1', resourceId: 'docker:container:edge', capabilityName: 'restart' }, authorities: [{ kind: 'capability_registry', sourceId: 'capability-registry:restart', status: 'consulted', scope: { orgId: 'org-1', resourceId: 'docker:container:edge', capabilityName: 'restart' }, approvalFloor: 'admin', reasonCodes: ['capability_approval_admin'] }], approvalRequirement: { version: 1, floor: 'admin', quorum: 1, disallowRequester: false }, planningAllowed: true, requiresApproval: true }
      : { version: 0, status, scope: { orgId: '', resourceId: '', capabilityName: '' }, authorities: [], approvalRequirement: { version: 0, floor: 'admin', quorum: 1, disallowRequester: false }, planningAllowed: false, requiresApproval: true },
  },
  verificationOutcome: { status: 'unknown' },
});
const detail = (audit: ActionAuditRecord): ActionDetailResponse => ({ audit, events: [] });

describe('ActionReviewDialog trust gates', () => {
  it('offers no approve or run control for legacy provenance', () => {
    render(() => <ActionReviewDialog detail={detail(makeAudit('legacy_unknown', '2099-01-01T00:00:00Z'))} onClose={vi.fn()} />);
    expect(screen.getByTestId('action-review-invalid')).toHaveTextContent('no current server policy provenance');
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Run action' })).toBeNull();
  });

  it('removes decision controls when expiry passes while the dialog remains open', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-07-12T00:00:00Z'));
    render(() => <ActionReviewDialog detail={detail(makeAudit('resolved', '2026-07-12T00:00:00.500Z'))} onClose={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Approve' })).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(1000);
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
    expect(screen.getByTestId('action-review-invalid')).toHaveTextContent('review expired');
  });
});
