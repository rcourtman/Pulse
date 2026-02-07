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

  it('adds node query for host-scoped workloads summary', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);

    await ChartsAPI.getWorkloadsSummaryCharts('1h', undefined, { nodeId: 'cluster-a-node-1' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/charts/workloads-summary?range=1h&node=cluster-a-node-1',
      { signal: undefined },
    );
  });

  it('calls workload-only charts endpoint with node and maxPoints', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as any);

    await ChartsAPI.getWorkloadCharts('1h', undefined, {
      nodeId: 'cluster-a-node-1',
      maxPoints: 180.2,
    });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/charts/workloads?range=1h&node=cluster-a-node-1&maxPoints=180',
      { signal: undefined },
    );
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
