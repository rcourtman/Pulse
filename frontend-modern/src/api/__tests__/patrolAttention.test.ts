import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import {
  getPatrolAttention,
  getPatrolAttentionDetail,
  getPatrolAttentionSummary,
  planPatrolAttentionAction,
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
