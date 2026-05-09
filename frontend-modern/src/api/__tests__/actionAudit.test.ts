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
            verification: {
              ran: true,
              command: "systemctl is-active 'workload'",
              output: 'active',
              success: true,
              ranAt: '2026-04-29T12:00:25Z',
            },
          },
        },
      ],
      count: 1,
    } as any);

    const response = await ActionAuditAPI.listActionAudits({ resourceId: 'vm:42' });
    expect(response.audits).toHaveLength(1);
    const v = response.audits[0].result?.verification;
    expect(v?.ran).toBe(true);
    expect(v?.command).toBe("systemctl is-active 'workload'");
    expect(v?.output).toBe('active');
    expect(v?.success).toBe(true);
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
