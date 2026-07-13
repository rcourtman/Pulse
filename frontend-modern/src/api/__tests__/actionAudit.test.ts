import { createHash } from 'node:crypto';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { ActionAuditAPI } from '@/api/actionAudit';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { apiFetchJSON } from '@/utils/apiClient';
import type {
  ActionAuditRecord,
  ActionEvidence,
  ActionDecisionResponse,
  ActionExecutionResponse,
  ActionResultV2,
  ResourceActionRequest,
} from '@/types/actionAudit';

const canonicalEvidenceDigest = (evidence: ActionEvidence): string => {
  const envelope = {
    version: evidence.version,
    id: evidence.id,
    observerId: evidence.observerId,
    observerKind: evidence.observerKind,
    observerTrustDomain: evidence.observerTrustDomain,
    executorTrustDomain: evidence.executorTrustDomain,
    method: evidence.method,
    subjectId: evidence.subjectId,
    observedAt: evidence.observedAt,
    receivedAt: evidence.receivedAt,
    ...(evidence.reasonCode ? { reasonCode: evidence.reasonCode } : {}),
    ...(evidence.summary ? { summary: evidence.summary } : {}),
    ...(evidence.refs?.length ? { refs: evidence.refs } : {}),
    digest: '',
  };
  return `sha256:${createHash('sha256').update(JSON.stringify(envelope)).digest('hex')}`;
};

const makeAgentEvidence = (id: string): ActionEvidence => {
  const envelope: ActionEvidence = {
    version: 1,
    id,
    observerId: 'agent:pve-1',
    observerKind: 'unified_agent',
    observerTrustDomain: 'agent:pve-1',
    executorTrustDomain: 'agent:pve-1',
    method: 'typed_read_after_write',
    subjectId: 'proxmox:node:pve-1',
    observedAt: '2026-07-12T10:01:00Z',
    receivedAt: '2026-07-12T10:05:00Z',
    digest: '',
  };
  return { ...envelope, digest: canonicalEvidenceDigest(envelope) };
};

const legacyVerificationStatus = (
  truth: ActionResultV2,
): 'verified' | 'failed' | 'unverified' | 'unknown' =>
  truth.verification.status === 'confirmed'
    ? 'verified'
    : truth.verification.status === 'contradicted'
      ? 'failed'
      : truth.verification.status === 'inconclusive'
        ? 'unverified'
        : 'unknown';

const isTask10FixtureValid = (truth: ActionResultV2): boolean => {
  if (truth.execution.status !== 'succeeded' && !truth.execution.reasonCode?.trim()) return false;
  const evidenceCount = truth.verification.evidence?.length ?? 0;
  if (truth.verification.status === 'inconclusive' && !truth.verification.reasonCode?.trim())
    return false;
  if (
    truth.verification.status === 'not_attempted' &&
    (truth.verification.evidenceClass !== 'none' || evidenceCount !== 0)
  )
    return false;
  if (
    (truth.verification.status === 'confirmed' || truth.verification.status === 'contradicted') &&
    (truth.verification.evidenceClass === 'none' || evidenceCount === 0)
  )
    return false;
  if (truth.verification.evidenceClass === 'none' && evidenceCount !== 0) return false;
  if (truth.verification.evidenceClass !== 'none' && evidenceCount === 0) return false;
  if (
    (truth.verification.evidence ?? []).some((evidence) => {
      const observedAt = new Date(evidence.observedAt).valueOf();
      const receivedAt = new Date(evidence.receivedAt).valueOf();
      return (
        !Number.isFinite(observedAt) ||
        !Number.isFinite(receivedAt) ||
        receivedAt < observedAt ||
        evidence.digest !== canonicalEvidenceDigest(evidence)
      );
    })
  )
    return false;
  if (
    truth.verification.evidenceClass === 'independent' &&
    (truth.verification.evidence ?? []).some(
      (evidence) => evidence.observerTrustDomain === evidence.executorTrustDomain,
    )
  )
    return false;
  return true;
};

describe('ActionAuditAPI', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('loads the canonical oldest-first pending action queue', async () => {
    const response = {
      actions: [
        {
          id: 'action-1',
          createdAt: '2026-07-10T18:00:00Z',
          updatedAt: '2026-07-10T18:00:00Z',
          state: 'pending_approval' as const,
          request: {
            requestId: 'proposal-1',
            resourceId: 'docker:container:web',
            capabilityName: 'restart',
            reason: 'Health checks failed',
            requestedBy: 'pulse_patrol',
          },
          plan: {
            actionId: 'action-1',
            requestId: 'proposal-1',
            allowed: true,
            requiresApproval: true,
            approvalPolicy: 'admin',
            rollbackAvailable: true,
          },
        },
      ],
      count: 1,
    };
    apiFetchJSONMock.mockResolvedValueOnce(response);

    await expect(ResourceActionsAPI.listPendingActions()).resolves.toEqual(response);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/actions/pending');
  });

  it('hydrates durable inbox views and detail with policy provenance and independent result axes', async () => {
    const scope = {
      orgId: 'org-1',
      resourceId: 'docker:container:web',
      capabilityName: 'restart',
    };
    const approvalRequirement = {
      version: 1,
      floor: 'admin' as const,
      quorum: 1,
      disallowRequester: false,
    };
    const policyDecision = {
      version: 1,
      status: 'resolved' as const,
      decisionId: 'policy-decision:sha256:one',
      actionId: 'action/one',
      scope,
      authorities: [
        {
          kind: 'resource_operator_policy' as const,
          sourceId: 'resource-policy:docker:container:web',
          revision: 'resource-policy:sha256:one',
          status: 'consulted' as const,
          scope,
          approvalFloor: 'admin' as const,
          reasonCodes: ['resource_capability_allowed' as const, 'resource_window_open' as const],
        },
      ],
      approvalRequirement,
      planningAllowed: true,
      requiresApproval: true,
    };
    const audit: ActionAuditRecord = {
      id: 'action/one',
      createdAt: '2026-07-12T00:00:00Z',
      updatedAt: '2026-07-12T00:01:00Z',
      state: 'completed',
      request: {
        requestId: 'request-1',
        resourceId: scope.resourceId,
        capabilityName: scope.capabilityName,
        reason: 'Recover the edge proxy',
        requestedBy: 'pulse_patrol',
      },
      plan: {
        actionId: 'action/one',
        requestId: 'request-1',
        allowed: true,
        requiresApproval: true,
        approvalPolicy: 'admin',
        approvalRequirement,
        rollbackAvailable: true,
        expiresAt: '2026-07-12T00:10:00Z',
        policyDecision,
      },
      result: {
        success: true,
        actionResultV2: {
          version: 2,
          execution: { status: 'succeeded', summary: 'Dispatch completed.' },
          verification: {
            status: 'contradicted',
            evidenceClass: 'independent',
            summary: 'The observer still sees the prior state.',
          },
          compensation: {
            support: 'declared',
            status: 'not_attempted',
            strategy: 'Restore the previous container state.',
          },
        },
      },
    };

    apiFetchJSONMock
      .mockResolvedValueOnce({ view: 'pending', actions: [], count: 0 })
      .mockResolvedValueOnce({ view: 'settled', actions: [audit], count: 1 })
      .mockResolvedValueOnce({ audit, events: [] });

    await expect(ResourceActionsAPI.listActions('pending', 25)).resolves.toMatchObject({
      view: 'pending',
      count: 0,
    });
    await expect(ResourceActionsAPI.listActions('settled', 25)).resolves.toMatchObject({
      view: 'settled',
      count: 1,
    });
    const detail = await ResourceActionsAPI.getAction('action/one');

    expect(apiFetchJSONMock).toHaveBeenNthCalledWith(1, '/api/actions?view=pending&limit=25');
    expect(apiFetchJSONMock).toHaveBeenNthCalledWith(2, '/api/actions?view=settled&limit=25');
    expect(apiFetchJSONMock).toHaveBeenNthCalledWith(3, '/api/actions/action%2Fone');
    expect(detail.audit.plan.policyDecision).toEqual(policyDecision);
    expect(detail.audit.result?.actionResultV2).toMatchObject({
      execution: { status: 'succeeded' },
      verification: { status: 'contradicted', evidenceClass: 'independent' },
      compensation: { support: 'declared', status: 'not_attempted' },
    });
  });

  it('preserves typed APT ActionResultV2 axes and the durable receipt without normalization', async () => {
    const evidence = makeAgentEvidence('apt-update-evidence');
    const audit = {
      id: 'apt-update',
      createdAt: '2026-07-12T10:00:00Z',
      updatedAt: '2026-07-12T10:05:00Z',
      state: 'completed' as const,
      request: {
        requestId: 'apt-request',
        resourceId: 'proxmox:node:pve-1',
        capabilityName: 'install_os_updates',
        params: {},
        reason: 'Resolve the current update finding',
        requestedBy: 'pulse_patrol',
      },
      plan: {
        actionId: 'apt-update',
        requestId: 'apt-request',
        allowed: true,
        requiresApproval: true,
        approvalPolicy: 'admin' as const,
        rollbackAvailable: false,
      },
      result: {
        success: false,
        actionResultV2: {
          version: 2 as const,
          execution: {
            status: 'inconclusive' as const,
            reasonCode: 'possible_partial_effect',
            summary:
              'APT package updates: phase=install; 6 pending before, 3 pending after; package manager health: unhealthy; recovery required: true; reboot required: false',
          },
          verification: {
            status: 'contradicted' as const,
            evidenceClass: 'agent_attested' as const,
            summary: 'Three updates remain.',
            evidence: [evidence],
          },
          compensation: { support: 'unavailable' as const, status: 'not_available' as const },
        },
      },
      verificationOutcome: { status: 'failed' as const },
    };
    const detail = {
      audit,
      events: [],
      attempt: {
        id: 'attempt-1',
        actionId: audit.id,
        state: 'receipt_recorded' as const,
        createdAt: audit.createdAt,
        updatedAt: audit.updatedAt,
        dispatchCount: 1,
      },
      receipt: {
        attemptId: 'attempt-1',
        actionId: audit.id,
        transportRequestId: 'transport-1',
        receivedAt: audit.updatedAt,
      },
    };
    apiFetchJSONMock.mockResolvedValueOnce(detail);

    await expect(ResourceActionsAPI.getAction(audit.id)).resolves.toEqual(detail);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/actions/apt-update');
    expect((await Promise.resolve(detail)).audit.result.actionResultV2).toMatchObject({
      execution: { status: 'inconclusive' },
      verification: { status: 'contradicted', evidenceClass: 'agent_attested' },
      compensation: { support: 'unavailable', status: 'not_available' },
    });
    expect(detail.receipt).toMatchObject({
      attemptId: 'attempt-1',
      transportRequestId: 'transport-1',
    });
    expect(detail.audit.verificationOutcome?.status).toBe('failed');
    expect(detail.audit.result?.actionResultV2?.verification.evidence?.[0].digest).toBe(
      canonicalEvidenceDigest(evidence),
    );
  });

  it('guards tier-5 APT fixtures against impossible Task10 evidence combinations', () => {
    const evidence = makeAgentEvidence('evidence-1');
    const compensation = { support: 'unavailable' as const, status: 'not_available' as const };
    const valid = [
      {
        version: 2,
        execution: { status: 'succeeded' },
        verification: {
          status: 'confirmed',
          evidenceClass: 'agent_attested',
          evidence: [evidence],
        },
        compensation,
      },
      {
        version: 2,
        execution: { status: 'inconclusive', reasonCode: 'possible_partial_effect' },
        verification: {
          status: 'contradicted',
          evidenceClass: 'agent_attested',
          evidence: [evidence],
        },
        compensation,
      },
      {
        version: 2,
        execution: { status: 'inconclusive', reasonCode: 'possible_partial_effect' },
        verification: {
          status: 'inconclusive',
          evidenceClass: 'none',
          reasonCode: 'package_manager_health_unknown',
        },
        compensation,
      },
    ] satisfies ActionResultV2[];
    expect(valid.every(isTask10FixtureValid)).toBe(true);
    expect(valid.map(legacyVerificationStatus)).toEqual(['verified', 'failed', 'unverified']);
    const notAttempted = {
      version: 2,
      execution: { status: 'not_run', reasonCode: 'preflight_refused' },
      verification: { status: 'not_attempted', evidenceClass: 'none' },
      compensation,
    } satisfies ActionResultV2;
    expect(isTask10FixtureValid(notAttempted)).toBe(true);
    expect(legacyVerificationStatus(notAttempted)).toBe('unknown');
    expect(evidence.digest).toBe(canonicalEvidenceDigest(evidence));
    expect(new Date(evidence.receivedAt).valueOf()).toBeGreaterThanOrEqual(
      new Date(evidence.observedAt).valueOf(),
    );

    const impossible = [
      { ...valid[0], verification: { status: 'confirmed', evidenceClass: 'none', evidence: [] } },
      {
        ...valid[1],
        verification: { status: 'contradicted', evidenceClass: 'agent_attested', evidence: [] },
      },
      { ...valid[2], verification: { status: 'inconclusive', evidenceClass: 'none' } },
      {
        ...valid[2],
        verification: {
          status: 'inconclusive',
          evidenceClass: 'none',
          reasonCode: 'package_manager_health_unknown',
          evidence: [evidence],
        },
      },
      {
        ...valid[2],
        verification: {
          status: 'not_attempted',
          evidenceClass: 'agent_attested',
          evidence: [evidence],
        },
      },
      { ...valid[2], execution: { status: 'inconclusive' } },
      {
        ...valid[0],
        verification: {
          ...valid[0].verification,
          evidence: [{ ...evidence, digest: `sha256:${'a'.repeat(64)}` }],
        },
      },
      {
        ...valid[0],
        verification: {
          ...valid[0].verification,
          evidence: [
            {
              ...evidence,
              receivedAt: '2026-07-12T09:59:00Z',
              digest: canonicalEvidenceDigest({
                ...evidence,
                receivedAt: '2026-07-12T09:59:00Z',
                digest: '',
              }),
            },
          ],
        },
      },
    ] as ActionResultV2[];
    expect(impossible.map(isTask10FixtureValid)).toEqual([
      false,
      false,
      false,
      false,
      false,
      false,
      false,
      false,
    ]);
  });

  it('builds the canonical resource-scoped action audit query', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      audits: [
        {
          id: 'action-1',
          createdAt: '2026-04-29T12:00:00Z',
          updatedAt: '2026-04-29T12:01:00Z',
          state: 'completed',
          request: {
            requestId: 'req-1',
            resourceId: 'vm:42',
            capabilityName: 'restart',
            reason: 'Restart after patching',
            requestedBy: 'agent:ops',
          },
          plan: {
            actionId: 'action-1',
            requestId: 'req-1',
            allowed: true,
            requiresApproval: true,
            approvalPolicy: 'admin',
            rollbackAvailable: true,
          },
        },
      ],
      count: 1,
      resourceId: 'vm:42',
    });

    const response = await ActionAuditAPI.listActionAudits({
      resourceId: ' vm:42 ',
      since: '2026-04-29T11:00:00Z',
      limit: 5,
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/audit/actions?resourceId=vm%3A42&since=2026-04-29T11%3A00%3A00.000Z&limit=5',
      { cache: 'no-store' },
    );
    expect(response).toMatchObject({
      available: true,
      count: 1,
      resourceId: 'vm:42',
    });
    expect(response.audits).toHaveLength(1);
  });

  it('round-trips the post-dispatch verification result on the action audit response', async () => {
    // The broker now records ActionVerificationResult on completed audits
    // (read-after-write outcome). The TS API client must mirror the field
    // verbatim so the operator-facing surface can render Pulse's actual
    // verification — what command ran, what it returned, did it confirm
    // the intended state — instead of silently dropping it to the
    // unknown-property bucket.
    apiFetchJSONMock.mockResolvedValueOnce({
      audits: [
        {
          id: 'action-verify',
          createdAt: '2026-04-29T12:00:00Z',
          updatedAt: '2026-04-29T12:00:30Z',
          state: 'completed',
          request: {
            requestId: 'req-verify',
            resourceId: 'vm:42',
            capabilityName: 'pulse_control',
            reason: 'restart workload after backup',
            requestedBy: 'pulse_patrol',
          },
          plan: {
            actionId: 'action-verify',
            requestId: 'req-verify',
            allowed: true,
            requiresApproval: true,
            approvalPolicy: 'admin',
            plannedAt: '2026-04-29T11:59:50Z',
            expiresAt: '2026-04-29T12:04:50Z',
            planHash: 'sha256:test',
          },
          result: {
            success: true,
            output: 'OK',
          },
          verification: {
            ran: true,
            command: "systemctl is-active 'workload'",
            output: 'active',
            success: true,
            ranAt: '2026-04-29T12:00:25Z',
          },
        },
      ],
      count: 1,
    } as any);

    const response = await ActionAuditAPI.listActionAudits({ resourceId: 'vm:42' });
    expect(response.audits).toHaveLength(1);
    const v = response.audits[0].verification;
    expect(v?.ran).toBe(true);
    expect(v?.command).toBe("systemctl is-active 'workload'");
    expect(v?.output).toBe('active');
    expect(v?.success).toBe(true);
    expect(response.audits[0].result?.verification).toEqual(v);
  });

  it('normalizes legacy result verification onto the canonical action audit field', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      audits: [
        {
          id: 'action-legacy-verify',
          createdAt: '2026-04-29T12:00:00Z',
          updatedAt: '2026-04-29T12:00:30Z',
          state: 'completed',
          request: {
            requestId: 'req-legacy-verify',
            resourceId: 'vm:42',
            capabilityName: 'pulse_control',
            reason: 'restart workload after backup',
            requestedBy: 'pulse_patrol',
          },
          plan: {
            actionId: 'action-legacy-verify',
            requestId: 'req-legacy-verify',
            allowed: true,
            requiresApproval: false,
            approvalPolicy: 'none',
            rollbackAvailable: false,
          },
          result: {
            success: true,
            verification: {
              ran: false,
              success: false,
            },
          },
        },
      ],
      count: 1,
    } as any);

    const response = await ActionAuditAPI.listActionAudits({ resourceId: 'vm:42' });
    expect(response.audits[0].verification).toEqual({ ran: false, success: false });
    expect(response.audits[0].result?.verification).toEqual({ ran: false, success: false });
  });

  it('round-trips verification outcome evidence and refused execution results', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      audits: [
        {
          id: 'action-refused',
          createdAt: '2026-04-29T12:00:00Z',
          updatedAt: '2026-04-29T12:00:30Z',
          state: 'failed',
          request: {
            requestId: 'req-refused',
            resourceId: 'vm:42',
            capabilityName: 'restart_service',
            reason: 'restart workload after backup',
            requestedBy: 'pulse_patrol',
          },
          plan: {
            actionId: 'action-refused',
            requestId: 'req-refused',
            allowed: true,
            requiresApproval: true,
            approvalPolicy: 'admin',
            rollbackAvailable: false,
          },
          result: {
            success: false,
            errorMessage: 'resource_remediation_locked: operator lock is active',
          },
          verificationOutcome: {
            status: 'unverified',
            evidenceSummary: 'No dispatch occurred, so no verification probe ran.',
          },
        },
      ],
      count: 1,
    } as any);

    const response = await ActionAuditAPI.listActionAudits({ resourceId: 'vm:42' });
    expect(response.audits[0].result?.errorMessage).toBe(
      'resource_remediation_locked: operator lock is active',
    );
    expect(response.audits[0].verificationOutcome).toEqual({
      status: 'unverified',
      evidenceSummary: 'No dispatch occurred, so no verification probe ran.',
    });
    expect(response.audits[0].verification).toBeUndefined();
  });

  it('treats gated action audit endpoints as unavailable instead of throwing', async () => {
    apiFetchJSONMock.mockRejectedValueOnce(
      Object.assign(new Error('Payment Required'), { status: 402 }),
    );

    await expect(ActionAuditAPI.listActionAudits({ resourceId: 'vm:42' })).resolves.toEqual({
      audits: [],
      available: false,
      count: 0,
      resourceId: 'vm:42',
    });
  });

  it('plans resource actions through the governed action endpoint', async () => {
    const request: ResourceActionRequest = {
      requestId: 'req-docker-restart',
      resourceId: 'docker:container:abc123',
      capabilityName: 'docker.container.restart',
      reason: 'restart after configuration update',
      requestedBy: 'operator',
    };

    apiFetchJSONMock.mockResolvedValueOnce({
      actionId: 'action-docker-restart',
      requestId: request.requestId,
      allowed: true,
      requiresApproval: true,
      approvalPolicy: 'admin',
      rollbackAvailable: false,
    });

    const response = await ResourceActionsAPI.planAction(request);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/actions/plan', {
      method: 'POST',
      body: JSON.stringify(request),
    });
    expect(response).toMatchObject({
      actionId: 'action-docker-restart',
      requestId: request.requestId,
      allowed: true,
      requiresApproval: true,
    });
  });

  it('records decisions and executes actions through encoded governed action routes', async () => {
    const audit: ActionAuditRecord = {
      id: 'action/docker/restart',
      createdAt: '2026-06-12T20:39:00Z',
      updatedAt: '2026-06-12T20:40:00Z',
      state: 'approved',
      request: {
        requestId: 'req-docker-restart',
        resourceId: 'docker:container:abc123',
        capabilityName: 'docker.container.restart',
        reason: 'operator confirmed restart',
        requestedBy: 'operator',
      },
      plan: {
        actionId: 'action/docker/restart',
        requestId: 'req-docker-restart',
        allowed: true,
        requiresApproval: true,
        approvalPolicy: 'admin',
        rollbackAvailable: false,
      },
    };
    const decision: ActionDecisionResponse = {
      actionId: 'action/docker/restart',
      state: 'approved',
      approval: {
        actor: 'operator',
        method: 'api',
        timestamp: '2026-06-12T20:40:00Z',
        outcome: 'approved',
      },
      audit,
    };
    const execution: ActionExecutionResponse = {
      actionId: 'action/docker/restart',
      state: 'completed',
      result: {
        success: true,
        output: 'container restarted',
      },
      audit: {
        ...audit,
        updatedAt: '2026-06-12T20:41:00Z',
        state: 'completed',
        result: {
          success: true,
          output: 'container restarted',
        },
      },
    };

    apiFetchJSONMock.mockResolvedValueOnce(decision).mockResolvedValueOnce(execution);

    await expect(
      ResourceActionsAPI.decideAction(
        'action/docker/restart',
        'approved',
        'sha256:reviewed-plan',
        'operator confirmed restart',
      ),
    ).resolves.toEqual(decision);
    await expect(
      ResourceActionsAPI.executeAction('action/docker/restart', 'sha256:reviewed-plan'),
    ).resolves.toEqual(execution);

    expect(apiFetchJSONMock).toHaveBeenNthCalledWith(
      1,
      '/api/actions/action%2Fdocker%2Frestart/decision',
      {
        method: 'POST',
        body: JSON.stringify({
          outcome: 'approved',
          planHash: 'sha256:reviewed-plan',
          reason: 'operator confirmed restart',
        }),
      },
    );
    expect(apiFetchJSONMock).toHaveBeenNthCalledWith(
      2,
      '/api/actions/action%2Fdocker%2Frestart/execute',
      {
        method: 'POST',
        body: JSON.stringify({ planHash: 'sha256:reviewed-plan' }),
      },
    );
  });

  it('refuses action mutations without a reviewed plan identity', async () => {
    await expect(ResourceActionsAPI.decideAction('action-1', 'approved', '  ')).rejects.toThrow(
      'reviewed action plan identity',
    );
    await expect(ResourceActionsAPI.executeAction('action-1', '')).rejects.toThrow(
      'reviewed action plan identity',
    );
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
  });
});
