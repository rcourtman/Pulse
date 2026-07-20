import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { ActionAuditAPI } from '@/api/actionAudit';
import { apiFetchJSON } from '@/utils/apiClient';
import type { ActionAuditRecord, ActionVerificationResult } from '@/types/actionAudit';

const baseAudit: ActionAuditRecord = {
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
};

describe('ActionAuditAPI listActionAudits branch coverage', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  // Covers:
  //  - buildActionAuditQuery() empty-query arm (`return query ? \`?${query}\` : ''`)
  //  - normalizeSince undefined arm (first `||` clause short-circuits)
  //  - normalizeLimit undefined arm (not finite)
  //  - normalizeActionAuditListResponse count-absent arm (count -> audits.length)
  //  - normalizeActionAuditListResponse resourceId-absent + no fallback arm
  it('omits the query string entirely when no filter options are given and derives count from audits.length', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      audits: [baseAudit, { ...baseAudit, id: 'action-2' }],
    });

    const result = await ActionAuditAPI.listActionAudits();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/audit/actions', {
      cache: 'no-store',
    });
    expect(result.available).toBe(true);
    expect(result.count).toBe(2);
    expect(result.resourceId).toBeUndefined();
    expect(result.audits).toHaveLength(2);
  });

  // Covers normalizeSince `since instanceof Date` true arm (uses the Date directly
  // instead of constructing `new Date(since)`).
  it('encodes a Date instance since filter into an ISO query parameter', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ audits: [], count: 0 });

    await ActionAuditAPI.listActionAudits({
      since: new Date('2026-04-29T11:00:00Z'),
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/audit/actions?since=2026-04-29T11%3A00%3A00.000Z',
      { cache: 'no-store' },
    );
  });

  // Covers:
  //  - normalizeSince ``${since}`.trim() === ''` true arm (empty-string since dropped)
  //  - normalizeLimit `(limit ?? 0) <= 0` arm (limit=0 dropped)
  it('drops an empty-string since and a zero limit, leaving only the trimmed resourceId in the query', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ audits: [], count: 0, resourceId: 'vm:42' });

    await ActionAuditAPI.listActionAudits({
      resourceId: 'vm:42',
      since: '',
      limit: 0,
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/audit/actions?resourceId=vm%3A42', {
      cache: 'no-store',
    });
  });

  // Covers normalizeSince invalid-date arm:
  // `Number.isFinite(date.getTime()) ? date.toISOString() : undefined` false branch.
  it('drops a non-parseable since string from the query instead of emitting an Invalid Date', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ audits: [], count: 0 });

    await ActionAuditAPI.listActionAudits({ since: 'not-a-real-date' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/audit/actions', {
      cache: 'no-store',
    });
  });

  // Covers normalizeLimit Math.trunc arm (`return Math.trunc(limit ?? 0)`).
  it('truncates a positive fractional limit when building the query', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ audits: [], count: 0 });

    await ActionAuditAPI.listActionAudits({ limit: 4.9 });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/audit/actions?limit=4', {
      cache: 'no-store',
    });
  });

  // Covers normalizeActionAuditListResponse fallbackResourceId arm:
  // when raw.resourceId is absent/blank, fall back to the trimmed caller-supplied resourceId.
  it('falls back to the trimmed input resourceId when the response omits resourceId', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      audits: [baseAudit],
      count: 1,
    });

    const result = await ActionAuditAPI.listActionAudits({ resourceId: '  vm:42  ' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/audit/actions?resourceId=vm%3A42', {
      cache: 'no-store',
    });
    expect(result.resourceId).toBe('vm:42');
  });

  // Covers normalizeActionAuditRecord no-result arm:
  // verification present at top level but `audit.result` is undefined -> the
  // `audit.result ? {...} : audit.result` ternary takes the false branch.
  it('promotes a top-level verification onto an audit that has no result block, leaving result undefined', async () => {
    const verification: ActionVerificationResult = {
      ran: true,
      success: true,
      command: "systemctl is-active 'workload'",
      output: 'active',
    };
    apiFetchJSONMock.mockResolvedValueOnce({
      audits: [
        {
          ...baseAudit,
          id: 'action-verify-noresult',
          verification,
        },
      ],
      count: 1,
    });

    const result = await ActionAuditAPI.listActionAudits({ resourceId: 'vm:42' });

    expect(result.audits).toHaveLength(1);
    expect(result.audits[0].verification).toEqual(verification);
    expect(result.audits[0].result).toBeUndefined();
  });

  // Covers the catch block in listActionAudits:
  //  - 401 and 403 share the unavailable arm with the already-covered 402
  //  - a status outside ACTION_AUDIT_UNAVAILABLE_STATUSES (500) takes the rethrow arm.
  it('treats 401 and 403 as unavailable but rethrows a 500', async () => {
    apiFetchJSONMock
      .mockRejectedValueOnce(Object.assign(new Error('Unauthorized'), { status: 401 }))
      .mockRejectedValueOnce(Object.assign(new Error('Forbidden'), { status: 403 }))
      .mockRejectedValueOnce(Object.assign(new Error('Boom'), { status: 500 }));

    await expect(ActionAuditAPI.listActionAudits({ resourceId: 'vm:42' })).resolves.toEqual({
      audits: [],
      available: false,
      count: 0,
      resourceId: 'vm:42',
    });
    await expect(ActionAuditAPI.listActionAudits({ resourceId: 'vm:42' })).resolves.toEqual({
      audits: [],
      available: false,
      count: 0,
      resourceId: 'vm:42',
    });
    await expect(ActionAuditAPI.listActionAudits({ resourceId: 'vm:42' })).rejects.toThrow('Boom');
  });
});
