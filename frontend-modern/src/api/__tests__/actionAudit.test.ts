import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { ActionAuditAPI } from '@/api/actionAudit';
import { apiFetchJSON } from '@/utils/apiClient';

describe('ActionAuditAPI', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
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
});
