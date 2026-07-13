import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import type { ActionAuditRecord } from '@/types/actionAudit';
import { ActionDecisionPacket } from '../ActionDecisionPacket';

afterEach(cleanup);

const audit: ActionAuditRecord = {
  id: 'action-1',
  createdAt: '2026-07-12T00:00:00Z',
  updatedAt: '2026-07-12T00:01:00Z',
  state: 'completed',
  decisionRevision: 1,
  request: {
    requestId: 'request-1',
    resourceId: 'docker:container:edge',
    capabilityName: 'restart',
    reason: 'Recover the edge proxy',
    requestedBy: 'ui:docker-page',
  },
  resource: { id: 'docker:container:edge', name: 'Edge proxy', type: 'app-container' },
  plan: {
    actionId: 'action-1',
    requestId: 'request-1',
    allowed: true,
    requiresApproval: true,
    approvalPolicy: 'admin',
    approvalRequirement: { version: 1, floor: 'admin', quorum: 1, disallowRequester: false },
    rollbackAvailable: false,
    plannedAt: '2026-07-12T00:00:00Z',
    expiresAt: '2026-07-12T00:10:00Z',
    resourceVersion: 'resource:sha256:one',
    policyVersion: 'policy:sha256:one',
    planHash: 'sha256:plan',
    policyDecision: {
      version: 1,
      status: 'resolved',
      decisionId: 'policy-decision:sha256:one',
      actionId: 'action-1',
      scope: { orgId: 'org-1', resourceId: 'docker:container:edge', capabilityName: 'restart' },
      approvalRequirement: { version: 1, floor: 'admin', quorum: 1, disallowRequester: false },
      planningAllowed: true,
      requiresApproval: true,
      authorities: [
        {
          kind: 'capability_registry',
          sourceId: 'capability-registry:restart',
          revision: 'policy:sha256:one',
          status: 'consulted',
          scope: { orgId: 'org-1', resourceId: 'docker:container:edge', capabilityName: 'restart' },
          approvalFloor: 'admin',
          reasonCodes: ['capability_approval_admin', 'capability_auto_low_risk'],
        },
        {
          kind: 'resource_operator_policy',
          sourceId: 'resource-operator-policy:docker:container:edge',
          revision: 'resource-policy:sha256:one',
          status: 'consulted',
          scope: { orgId: 'org-1', resourceId: 'docker:container:edge', capabilityName: 'restart' },
          approvalFloor: 'admin',
          reasonCodes: ['resource_capability_allowed', 'resource_window_open'],
        },
      ],
    },
  },
  result: {
    success: true,
    actionResultV2: {
      version: 2,
      execution: { status: 'succeeded', summary: 'Dispatch completed.' },
      verification: {
        status: 'confirmed',
        evidenceClass: 'independent',
        summary: 'A separate observer saw the target state.',
        evidence: [],
      },
      compensation: { support: 'unavailable', status: 'not_available' },
    },
  },
  verificationOutcome: { status: 'verified' },
};

describe('ActionDecisionPacket', () => {
  it('shows server policy provenance, expiry, and independent result evidence as separate truth', () => {
    render(() => <ActionDecisionPacket audit={audit} />);
    expect(screen.getByText('Edge proxy')).toBeInTheDocument();
    expect(screen.getByText('docker:container:edge')).toBeInTheDocument();
    expect(screen.getByText('Policy evidence')).toBeInTheDocument();
    expect(screen.getByText(/2 authorities checked at planning/)).toBeInTheDocument();
    expect(screen.getByText('Capability safety policy')).not.toBeVisible();
    fireEvent.click(screen.getByText('Policy evidence'));
    expect(screen.getByText('Capability safety policy')).toBeInTheDocument();
    expect(screen.getByText('Policy for this resource')).toBeInTheDocument();
    expect(
      within(screen.getByTestId('action-execution-truth')).getByText('Succeeded'),
    ).toBeInTheDocument();
    expect(screen.getByText('Confirmed by independent observer')).toBeInTheDocument();
    expect(screen.getByText('Source: Independent observer')).toBeInTheDocument();
  });

  it.each([
    [
      'agent-attested confirmed',
      'succeeded',
      'confirmed',
      'agent_attested',
      'Succeeded',
      'Confirmed by executing agent',
      'Source: Executing agent',
    ],
    [
      'independent confirmed',
      'succeeded',
      'confirmed',
      'independent',
      'Succeeded',
      'Confirmed by independent observer',
      'Source: Independent observer',
    ],
    [
      'succeeded plus contradicted',
      'succeeded',
      'contradicted',
      'independent',
      'Succeeded',
      'Outcome contradicted',
      'Source: Independent observer',
    ],
    [
      'failed plus confirmed',
      'failed',
      'confirmed',
      'agent_attested',
      'Failed',
      'Confirmed by executing agent',
      'Source: Executing agent',
    ],
    [
      'not run plus not attempted',
      'not_run',
      'not_attempted',
      'none',
      'Did not run',
      'Outcome not verified',
      'Source: No evidence source',
    ],
    [
      'inconclusive plus confirmed',
      'inconclusive',
      'confirmed',
      'independent',
      'Inconclusive',
      'Confirmed by independent observer',
      'Source: Independent observer',
    ],
    [
      'confirmed without evidence source',
      'succeeded',
      'confirmed',
      'none',
      'Succeeded',
      'Confirmation lacks an evidence source',
      'Source: No evidence source',
    ],
  ] as const)(
    'keeps execution, verification source, and recovery separate for %s',
    (
      _name,
      execution,
      verification,
      evidenceClass,
      executionLabel,
      verificationLabel,
      sourceLabel,
    ) => {
      const variant: ActionAuditRecord = {
        ...audit,
        result: {
          success: execution === 'succeeded',
          actionResultV2: {
            version: 2,
            execution: { status: execution },
            verification: { status: verification, evidenceClass },
            compensation: {
              support: 'declared',
              status: 'not_attempted',
              strategy: 'restart previous container',
            },
          },
        },
      };
      render(() => <ActionDecisionPacket audit={variant} />);
      expect(
        within(screen.getByTestId('action-execution-truth')).getByText(executionLabel),
      ).toBeInTheDocument();
      expect(
        within(screen.getByTestId('action-verification-truth')).getByText(verificationLabel),
      ).toBeInTheDocument();
      expect(
        within(screen.getByTestId('action-verification-truth')).getByText(sourceLabel),
      ).toBeInTheDocument();
      expect(
        within(screen.getByTestId('action-compensation-truth')).getByText('Not Attempted'),
      ).toBeInTheDocument();
      cleanup();
    },
  );

  it('shows bounded APT facts, agent attestation, recovery, and one durable receipt without a reboot control', () => {
    const aptAudit: ActionAuditRecord = {
      ...audit,
      request: {
        ...audit.request,
        resourceId: 'proxmox:node:pve-1',
        capabilityName: 'install_os_updates',
        params: {},
      },
      plan: {
        ...audit.plan,
        policyDecision: {
          ...audit.plan.policyDecision!,
          scope: {
            ...audit.plan.policyDecision!.scope,
            resourceId: 'proxmox:node:pve-1',
            capabilityName: 'install_os_updates',
          },
          authorities: audit.plan.policyDecision!.authorities.map((authority) => ({
            ...authority,
            scope: {
              ...authority.scope,
              resourceId: 'proxmox:node:pve-1',
              capabilityName: 'install_os_updates',
            },
            reasonCodes: ['capability_approval_admin', 'capability_auto_elevated'],
          })),
        },
      },
      result: {
        success: true,
        actionResultV2: {
          version: 2,
          execution: {
            status: 'succeeded',
            summary:
              'APT package updates: phase=complete; 6 pending before, 0 pending after; package manager health: healthy; recovery required: false; reboot required: true',
          },
          verification: {
            status: 'confirmed',
            evidenceClass: 'agent_attested',
            summary: 'The executing agent observed the canonical postcondition.',
            evidence: [
              {
                version: 1,
                id: 'evidence-1',
                observerId: 'agent:pve-1',
                observerKind: 'agent',
                observerTrustDomain: 'host:pve-1',
                executorTrustDomain: 'host:pve-1',
                method: 'typed_read_after_write',
                subjectId: 'proxmox:node:pve-1',
                observedAt: '2026-07-12T00:01:00Z',
                receivedAt: '2026-07-12T00:05:00Z',
                digest: 'sha256:evidence',
              },
            ],
          },
          compensation: {
            support: 'unavailable',
            status: 'not_available',
            summary: 'No rollback is available.',
          },
        },
      },
    };
    render(() => (
      <ActionDecisionPacket
        audit={aptAudit}
        detail={{
          audit: aptAudit,
          events: [],
          attempt: {
            id: 'attempt-1',
            actionId: aptAudit.id,
            state: 'receipt_recorded',
            createdAt: aptAudit.createdAt,
            updatedAt: aptAudit.updatedAt,
            dispatchCount: 1,
          },
          receipt: {
            attemptId: 'attempt-1',
            actionId: aptAudit.id,
            transportRequestId: 'transport-1',
            receivedAt: '2026-07-12T00:05:00Z',
          },
        }}
      />
    ));
    expect(
      within(screen.getByTestId('apt-action-facts')).getByText(
        'Yes — fact only; no reboot was authorized',
      ),
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId('action-verification-truth')).getByText(
        'Confirmed by executing agent',
      ),
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId('action-compensation-truth')).getByText(
        'No rollback is available.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('One agent receipt is recorded for this action.')).toBeInTheDocument();
    expect(screen.getAllByTestId('action-delivery-truth')).toHaveLength(1);
    expect(screen.queryByRole('button', { name: /reboot/i })).toBeNull();
    expect(screen.queryByText('APT package updates:', { exact: false })).toBeNull();
  });
});
