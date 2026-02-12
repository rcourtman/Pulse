import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { getPatrolRunHistory, getPatrolRunHistoryWithToolCalls } from '@/api/patrol';
import { apiFetchJSON } from '@/utils/apiClient';

describe('patrol api', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    apiFetchJSONMock.mockResolvedValue([] as any);
  });

  it('normalizes invalid limits for patrol history queries', async () => {
    await getPatrolRunHistory(Number.POSITIVE_INFINITY);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/runs?limit=30');

    await getPatrolRunHistory(0);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/runs?limit=1');
  });

  it('builds tool-call history query with normalized limit', async () => {
    await getPatrolRunHistoryWithToolCalls(25.9);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/runs?include=tool_calls&limit=25');
  });
});
