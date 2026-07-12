import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import type { ActionAuditRecord } from '@/types/actionAudit';
import { ActionDecisionPacket } from '../ActionDecisionPacket';

afterEach(cleanup);

const audit: ActionAuditRecord = {
  id: 'action-1', createdAt: '2026-07-12T00:00:00Z', updatedAt: '2026-07-12T00:01:00Z', state: 'completed', decisionRevision: 1,
  request: { requestId: 'request-1', resourceId: 'docker:container:edge', capabilityName: 'restart', reason: 'Recover the edge proxy', requestedBy: 'ui:docker-page' },
  plan: {
    actionId: 'action-1', requestId: 'request-1', allowed: true, requiresApproval: true, approvalPolicy: 'admin', approvalRequirement: { version: 1, floor: 'admin', quorum: 1, disallowRequester: false }, rollbackAvailable: false,
    plannedAt: '2026-07-12T00:00:00Z', expiresAt: '2026-07-12T00:10:00Z', resourceVersion: 'resource:sha256:one', policyVersion: 'policy:sha256:one', planHash: 'sha256:plan',
    policyDecision: { version: 1, status: 'resolved', decisionId: 'policy-decision:sha256:one', actionId: 'action-1', scope: { orgId: 'org-1', resourceId: 'docker:container:edge', capabilityName: 'restart' }, approvalRequirement: { version: 1, floor: 'admin', quorum: 1, disallowRequester: false }, planningAllowed: true, requiresApproval: true, authorities: [
      { kind: 'capability_registry', sourceId: 'capability-registry:restart', revision: 'policy:sha256:one', status: 'consulted', scope: { orgId: 'org-1', resourceId: 'docker:container:edge', capabilityName: 'restart' }, approvalFloor: 'admin', reasonCodes: ['capability_approval_admin', 'capability_auto_low_risk'] },
      { kind: 'resource_operator_policy', sourceId: 'resource-operator-policy:docker:container:edge', revision: 'resource-policy:sha256:one', status: 'consulted', scope: { orgId: 'org-1', resourceId: 'docker:container:edge', capabilityName: 'restart' }, approvalFloor: 'admin', reasonCodes: ['resource_capability_allowed', 'resource_window_open'] },
    ] },
  },
  result: { success: true, actionResultV2: { version: 2, execution: { status: 'succeeded', summary: 'Dispatch completed.' }, verification: { status: 'confirmed', evidenceClass: 'independent', summary: 'A separate observer saw the target state.', evidence: [] }, compensation: { support: 'unavailable', status: 'not_available' } } },
  verificationOutcome: { status: 'verified' },
};

describe('ActionDecisionPacket', () => {
  it('shows server policy provenance, expiry, and independent result evidence as separate truth', () => {
    render(() => <ActionDecisionPacket audit={audit} />);
    expect(screen.getByText('Why Pulse allows this review')).toBeInTheDocument();
    expect(screen.getByText('Capability safety policy')).toBeInTheDocument();
    expect(screen.getByText('Policy for this resource')).toBeInTheDocument();
    expect(within(screen.getByTestId('action-execution-truth')).getByText('Succeeded')).toBeInTheDocument();
    expect(screen.getByText('Confirmed by independent observer')).toBeInTheDocument();
    expect(screen.getByText('Source: Independent observer')).toBeInTheDocument();
  });

  it.each([
    ['agent-attested confirmed', 'succeeded', 'confirmed', 'agent_attested', 'Succeeded', 'Confirmed by executing agent', 'Source: Executing agent'],
    ['independent confirmed', 'succeeded', 'confirmed', 'independent', 'Succeeded', 'Confirmed by independent observer', 'Source: Independent observer'],
    ['succeeded plus contradicted', 'succeeded', 'contradicted', 'independent', 'Succeeded', 'Outcome contradicted', 'Source: Independent observer'],
    ['failed plus confirmed', 'failed', 'confirmed', 'agent_attested', 'Failed', 'Confirmed by executing agent', 'Source: Executing agent'],
    ['not run plus not attempted', 'not_run', 'not_attempted', 'none', 'Did not run', 'Outcome not verified', 'Source: No evidence source'],
    ['inconclusive plus confirmed', 'inconclusive', 'confirmed', 'independent', 'Inconclusive', 'Confirmed by independent observer', 'Source: Independent observer'],
    ['confirmed without evidence source', 'succeeded', 'confirmed', 'none', 'Succeeded', 'Confirmation lacks an evidence source', 'Source: No evidence source'],
  ] as const)('keeps execution, verification source, and recovery separate for %s', (_name, execution, verification, evidenceClass, executionLabel, verificationLabel, sourceLabel) => {
    const variant: ActionAuditRecord = {
      ...audit,
      result: {
        success: execution === 'succeeded',
        actionResultV2: {
          version: 2,
          execution: { status: execution },
          verification: { status: verification, evidenceClass },
          compensation: { support: 'declared', status: 'not_attempted', strategy: 'restart previous container' },
        },
      },
    };
    render(() => <ActionDecisionPacket audit={variant} />);
    expect(within(screen.getByTestId('action-execution-truth')).getByText(executionLabel)).toBeInTheDocument();
    expect(within(screen.getByTestId('action-verification-truth')).getByText(verificationLabel)).toBeInTheDocument();
    expect(within(screen.getByTestId('action-verification-truth')).getByText(sourceLabel)).toBeInTheDocument();
    expect(within(screen.getByTestId('action-compensation-truth')).getByText('Not Attempted')).toBeInTheDocument();
    cleanup();
  });
});
