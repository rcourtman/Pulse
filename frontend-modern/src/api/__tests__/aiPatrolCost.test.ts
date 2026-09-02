import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { getPatrolCostPreview, getPatrolModelGuidance } from '@/api/aiPatrolCost';
import { apiFetchJSON } from '@/utils/apiClient';

describe('aiPatrolCost API', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('previews the configured Patrol model when no query is given', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ provider: 'gemini' } as never);
    await getPatrolCostPreview();
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/cost-preview', undefined);
  });

  it('encodes a pending model route and schedule, including manual-only', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);
    const controller = new AbortController();
    await getPatrolCostPreview(
      { model: ' anthropic:claude-haiku-4-5 ', intervalMinutes: 0 },
      controller.signal,
    );
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/ai/patrol/cost-preview?model=anthropic%3Aclaude-haiku-4-5&interval_minutes=0',
      { signal: controller.signal },
    );
  });

  it('drops blank models and non-finite intervals from the query', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);
    await getPatrolCostPreview({ model: '   ', intervalMinutes: Number.NaN });
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/cost-preview', undefined);
  });

  it('fetches model guidance from the canonical route', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({ rules: [] } as never);
    await expect(getPatrolModelGuidance()).resolves.toEqual({ rules: [] });
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/patrol/model-guidance');
  });
});
