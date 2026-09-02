import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import {
  acknowledgePatrolAttention,
  getPatrolAttention,
  getPatrolAttentionDetail,
  getPatrolAttentionEvidence,
  getPatrolAttentionSummary,
  getPatrolWorkReceipts,
  planPatrolAttentionAction,
  suppressPatrolAttention,
  unacknowledgePatrolAttention,
  unsuppressPatrolAttention,
} from '@/api/patrolAttention';
import { apiFetchJSON } from '@/utils/apiClient';

describe('Patrol attention API', () => {
  const fetchMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    fetchMock.mockReset();
    fetchMock.mockResolvedValue({});
  });

  it('returns the backend flapping summary on attention items unchanged', async () => {
    const flapping = {
      transitionCount: 11,
      windowHours: 24,
      firstTransitionAt: '2026-08-09T15:00:00Z',
      lastTransitionAt: '2026-08-10T14:30:00Z',
    };
    fetchMock.mockResolvedValue({
      data: [{ id: 'record-flap', flapping }],
      summary: {},
      meta: { page: 1, limit: 50, total: 1, totalPages: 1 },
    });
    const response = await getPatrolAttention();
    expect(response.data[0].flapping).toEqual(flapping);
  });

  it('uses one bounded typed list query', async () => {
    await getPatrolAttention('stale_unknown', 2, 40);
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/ai/patrol/attention?filter=stale_unknown&page=2&limit=40',
    );
  });

  it('uses the canonical summary and encoded stable item detail routes', async () => {
    await getPatrolAttentionSummary();
    expect(fetchMock).toHaveBeenLastCalledWith('/api/ai/patrol/attention/summary');

    await getPatrolAttentionDetail('record/one');
    expect(fetchMock).toHaveBeenLastCalledWith('/api/ai/patrol/attention/record%2Fone');

    await getPatrolAttentionEvidence('record/one', 'evidence/two');
    expect(fetchMock).toHaveBeenLastCalledWith(
      '/api/ai/patrol/attention/record%2Fone/evidence/evidence%2Ftwo',
    );
  });

  it('uses the server-filtered verified work receipt projection', async () => {
    await getPatrolWorkReceipts(6);
    expect(fetchMock).toHaveBeenLastCalledWith('/api/ai/patrol/attention/receipts?limit=6');
  });

  it('uses canonical item-scoped lifecycle mutations with bounded suppression input', async () => {
    await acknowledgePatrolAttention('record/one');
    expect(fetchMock).toHaveBeenLastCalledWith(
      '/api/ai/patrol/attention/record%2Fone/acknowledge',
      { method: 'POST', body: '{}' },
    );

    await unacknowledgePatrolAttention('record/one');
    expect(fetchMock).toHaveBeenLastCalledWith(
      '/api/ai/patrol/attention/record%2Fone/unacknowledge',
      { method: 'POST', body: '{}' },
    );

    await suppressPatrolAttention('record/one', 'Maintenance window', '2026-07-20T08:00:00Z');
    expect(fetchMock).toHaveBeenLastCalledWith('/api/ai/patrol/attention/record%2Fone/suppress', {
      method: 'POST',
      body: JSON.stringify({
        reason: 'Maintenance window',
        expiresAt: '2026-07-20T08:00:00Z',
      }),
    });

    await unsuppressPatrolAttention('record/one');
    expect(fetchMock).toHaveBeenLastCalledWith('/api/ai/patrol/attention/record%2Fone/unsuppress', {
      method: 'POST',
      body: '{}',
    });
  });

  it('plans the fixed attention capability without accepting public action authority', async () => {
    await planPatrolAttentionAction('record/one', 'restart');

    expect(fetchMock).toHaveBeenLastCalledWith(
      '/api/ai/patrol/attention/record%2Fone/actions/restart/plan',
      {
        method: 'POST',
        body: '{}',
      },
    );
  });
});
