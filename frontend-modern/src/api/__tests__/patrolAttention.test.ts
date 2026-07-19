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
