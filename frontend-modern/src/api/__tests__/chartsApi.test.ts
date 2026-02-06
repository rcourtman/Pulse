import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { ChartsAPI } from '@/api/charts';
import { apiFetchJSON } from '@/utils/apiClient';

describe('ChartsAPI', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('calls /api/charts with default range', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);

    await ChartsAPI.getCharts();

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts?range=1h', { signal: undefined });
  });

  it('forwards an abort signal when provided', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);
    const controller = new AbortController();

    await ChartsAPI.getCharts('24h', controller.signal);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts?range=24h', { signal: controller.signal });
  });

  it('builds metrics history query params including maxPoints', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);

    await ChartsAPI.getMetricsHistory({
      resourceType: 'node',
      resourceId: 'node-1',
      metric: 'cpu',
      range: '7d',
      maxPoints: 321.4,
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/metrics-store/history?resourceType=node&resourceId=node-1&metric=cpu&range=7d&maxPoints=321',
    );
  });
});

