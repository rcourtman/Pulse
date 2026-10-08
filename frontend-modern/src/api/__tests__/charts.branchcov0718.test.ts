/**
 * Branch-coverage tests for ChartsAPI.getInfrastructureSummaryCharts
 * range/node/signal request shaping.
 *
 * These tests assert request shaping (final path + query string + signal) and
 * response handling. They mock the transport with the same harness used by
 * chartsApi.test.ts (vi.mock('@/utils/apiClient', ...)) and intentionally do
 * NOT re-assert anything chartsApi.test.ts already covers
 * (getCharts, getInfrastructureSummaryCharts metric filters, getWorkload*,
 * getMetricsHistory).
 *
 * Branches exercised here:
 *   - range default ('1h') vs explicit value
 *   - signal present vs undefined
 *   - options.nodeId truthy (string) -> `node=` param appended
 *   - options.nodeId falsy variants -> `node=` param omitted:
 *        * null
 *        * '' (empty string)
 *        * options undefined entirely
 *   - URL-encoding of special chars inside nodeId (URLSearchParams.toString)
 *   - combined range + node + signal in a single request
 *   - the parsed payload from apiFetchJSON is returned verbatim
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { ChartsAPI, type InfrastructureChartsResponse, type TimeRange } from '@/api/charts';
import { apiFetchJSON } from '@/utils/apiClient';

const ALL_TIME_RANGES: TimeRange[] = ['5m', '15m', '30m', '1h', '4h', '12h', '24h', '7d', '30d'];

describe('ChartsAPI.getInfrastructureSummaryCharts — branch coverage', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('routes to /charts/infrastructure with default range=1h and signal undefined when called with no args', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureSummaryCharts();

    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=1h', {
      signal: undefined,
    });
  });

  it('passes an explicit range token through to the URL without transformation', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureSummaryCharts('24h');

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=24h', {
      signal: undefined,
    });
  });

  it.each(ALL_TIME_RANGES)(
    'forwards TimeRange="%s" verbatim into the range query param',
    async (range) => {
      apiFetchJSONMock.mockResolvedValueOnce({} as never);

      await ChartsAPI.getInfrastructureSummaryCharts(range);

      expect(apiFetchJSONMock).toHaveBeenCalledWith(`/api/charts/infrastructure?range=${range}`, {
        signal: undefined,
      });
    },
  );

  it('forwards an AbortSignal through to apiFetchJSON', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);
    const controller = new AbortController();

    await ChartsAPI.getInfrastructureSummaryCharts('1h', controller.signal);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=1h', {
      signal: controller.signal,
    });
  });

  it('appends node=<id> when options.nodeId is a non-empty string', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureSummaryCharts('1h', undefined, { nodeId: 'cluster-a-node-1' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/charts/infrastructure?range=1h&node=cluster-a-node-1',
      { signal: undefined },
    );
  });

  it('omits the node query param when options.nodeId is explicitly null (falsy branch)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureSummaryCharts('1h', undefined, { nodeId: null });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=1h', {
      signal: undefined,
    });
  });

  it('omits the node query param when options.nodeId is an empty string (falsy branch)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureSummaryCharts('1h', undefined, { nodeId: '' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=1h', {
      signal: undefined,
    });
  });

  it('omits the node query param when options is undefined entirely', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureSummaryCharts('1h', undefined, undefined);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=1h', {
      signal: undefined,
    });
  });

  it('URL-encodes special characters in the node id (URLSearchParams.toString)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureSummaryCharts('1h', undefined, { nodeId: 'node a/b' });

    // space -> '+', '/' -> '%2F'
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/charts/infrastructure?range=1h&node=node+a%2Fb',
      { signal: undefined },
    );
  });

  it('combines range + node + signal in a single request with range-first/node-second ordering', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);
    const controller = new AbortController();

    await ChartsAPI.getInfrastructureSummaryCharts('4h', controller.signal, { nodeId: 'pve1' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=4h&node=pve1', {
      signal: controller.signal,
    });
  });

  it('returns the parsed InfrastructureChartsResponse payload verbatim from apiFetchJSON', async () => {
    const payload: InfrastructureChartsResponse = {
      nodeData: {
        pve1: { cpu: [{ timestamp: 1000, value: 12.5 }] },
      },
      dockerHostData: { 'dh-1': { memory: [{ timestamp: 2000, value: 70 }] } },
      agentData: { 'agent-7': { disk: [{ timestamp: 3000, value: 5 }] } },
      timestamp: 1733700000000,
      stats: {
        oldestDataTimestamp: 1733696400000,
        range: '1h',
        rangeSeconds: 3600,
        metricsStoreEnabled: true,
      },
    };
    apiFetchJSONMock.mockResolvedValueOnce(payload as never);

    const result = await ChartsAPI.getInfrastructureSummaryCharts('1h');

    expect(result).toBe(payload);
  });
});
