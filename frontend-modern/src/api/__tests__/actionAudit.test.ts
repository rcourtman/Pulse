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
