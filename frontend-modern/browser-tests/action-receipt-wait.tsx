// Mount the production action dialog with a synthetic, recent receipt wait.
// The browser runner supplies only bounded read-only API responses.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { ActionReviewDialog } from '../src/features/actions/ActionReviewDialog';
import type { ActionDetailResponse } from '../src/types/actionAudit';
import '../src/index.css';

const now = Date.now();
const initial: ActionDetailResponse = {
  audit: {
    id: 'action-fixture-1',
    createdAt: new Date(now - 6 * 60_000).toISOString(),
    updatedAt: new Date(now - 5 * 60_000).toISOString(),
    state: 'executing',
    decisionRevision: 1,
    request: {
      requestId: 'request-fixture-1',
      resourceId: 'docker:container:edge',
      capabilityName: 'update',
      reason: 'Update edge to its latest image.',
      requestedBy: 'operator',
    },
    plan: {
      actionId: 'action-fixture-1',
      requestId: 'request-fixture-1',
      allowed: true,
      requiresApproval: false,
      approvalPolicy: 'none',
      approvalRequirement: {
        version: 1,
        floor: 'none',
        quorum: 0,
        disallowRequester: false,
      },
      rollbackAvailable: false,
      expiresAt: new Date(now + 30 * 60_000).toISOString(),
      planHash: 'sha256:fixture-plan',
      policyDecision: {
        version: 1,
        status: 'resolved',
        decisionId: 'fixture-policy',
        actionId: 'action-fixture-1',
        scope: { orgId: 'org-fixture', resourceId: 'docker:container:edge', capabilityName: 'update' },
        authorities: [],
        approvalRequirement: {
          version: 1,
          floor: 'none',
          quorum: 0,
          disallowRequester: false,
        },
        planningAllowed: true,
        requiresApproval: false,
      },
    },
    verificationOutcome: { status: 'unknown' },
  },
  events: [],
  attempt: {
    id: 'attempt-fixture-1',
    actionId: 'action-fixture-1',
    state: 'receipt_pending',
    createdAt: new Date(now - 5 * 60_000).toISOString(),
    updatedAt: new Date(now - 5 * 60_000).toISOString(),
    dispatchCount: 1,
  },
};

const [selected, setSelected] = createSignal(initial);
render(
  () => <ActionReviewDialog detail={selected()} onClose={() => {}} onChanged={setSelected} />,
  document.getElementById('root')!,
);
