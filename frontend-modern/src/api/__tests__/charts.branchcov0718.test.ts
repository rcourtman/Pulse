/**
 * Branch-coverage tests for the currently-uncovered ChartsAPI methods:
 *   - ChartsAPI.getInfrastructureCharts (deprecated delegate -> getInfrastructureSummaryCharts)
 *   - ChartsAPI.getStorageSummaryTrend
 *
 * These tests assert request shaping (final path + query string + signal) and
 * response handling. They mock the transport with the same harness used by
 * chartsApi.test.ts (vi.mock('@/utils/apiClient', ...)) and intentionally do
 * NOT re-assert anything chartsApi.test.ts already covers
 * (getCharts, getInfrastructureSummaryCharts metric filters, getWorkload*,
 * getMetricsHistory).
 *
 * Branches exercised here:
 *   - range default ('1h' for infra, '24h' for storage-trend) vs explicit value
 *   - signal present vs undefined
 *   - options.nodeId truthy (string) -> `node=` param appended
 *   - options.nodeId falsy variants -> `node=` param omitted:
 *        * null
 *        * '' (empty string)
 *        * options undefined entirely
 *   - URL-encoding of special chars inside nodeId (URLSearchParams.toString)
 *   - combined range + node + signal in a single request
 *   - getStorageSummaryTrend forwards the raw TimeRange token without calling
 *     timeRangeToMinutes() (unlike getStorageSummaryCharts)
 *   - each function returns the parsed payload from apiFetchJSON verbatim
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import {
  ChartsAPI,
  type InfrastructureChartsResponse,
  type StorageSummaryTrendResponse,
  type TimeRange,
} from '@/api/charts';
import { apiFetchJSON } from '@/utils/apiClient';

const ALL_TIME_RANGES: TimeRange[] = ['5m', '15m', '30m', '1h', '4h', '12h', '24h', '7d', '30d'];

describe('ChartsAPI.getInfrastructureCharts — branch coverage', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('routes to /charts/infrastructure with default range=1h and signal undefined when called with no args', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureCharts();

    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=1h', {
      signal: undefined,
    });
  });

  it('passes an explicit range token through to the URL without transformation', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureCharts('24h');

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=24h', {
      signal: undefined,
    });
  });

  it.each(ALL_TIME_RANGES)(
    'forwards TimeRange="%s" verbatim into the range query param',
    async (range) => {
      apiFetchJSONMock.mockResolvedValueOnce({} as never);

      await ChartsAPI.getInfrastructureCharts(range);

      expect(apiFetchJSONMock).toHaveBeenCalledWith(`/api/charts/infrastructure?range=${range}`, {
        signal: undefined,
      });
    },
  );

  it('forwards an AbortSignal through to apiFetchJSON', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);
    const controller = new AbortController();

    await ChartsAPI.getInfrastructureCharts('1h', controller.signal);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=1h', {
      signal: controller.signal,
    });
  });

  it('appends node=<id> when options.nodeId is a non-empty string', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureCharts('1h', undefined, { nodeId: 'cluster-a-node-1' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/charts/infrastructure?range=1h&node=cluster-a-node-1',
      { signal: undefined },
    );
  });

  it('omits the node query param when options.nodeId is explicitly null (falsy branch)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureCharts('1h', undefined, { nodeId: null });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=1h', {
      signal: undefined,
    });
  });

  it('omits the node query param when options.nodeId is an empty string (falsy branch)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureCharts('1h', undefined, { nodeId: '' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=1h', {
      signal: undefined,
    });
  });

  it('omits the node query param when options is undefined entirely', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureCharts('1h', undefined, undefined);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/infrastructure?range=1h', {
      signal: undefined,
    });
  });

  it('URL-encodes special characters in the node id (URLSearchParams.toString)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getInfrastructureCharts('1h', undefined, { nodeId: 'node a/b' });

    // space -> '+', '/' -> '%2F'
    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/charts/infrastructure?range=1h&node=node+a%2Fb',
      { signal: undefined },
    );
  });

  it('combines range + node + signal in a single request with range-first/node-second ordering', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);
    const controller = new AbortController();

    await ChartsAPI.getInfrastructureCharts('4h', controller.signal, { nodeId: 'pve1' });

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

    const result = await ChartsAPI.getInfrastructureCharts('1h');

    expect(result).toBe(payload);
  });
});

describe('ChartsAPI.getStorageSummaryTrend — branch coverage', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('routes to /charts/storage-summary with default range=24h and signal undefined when called with no args', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getStorageSummaryTrend();

    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/storage-summary?range=24h', {
      signal: undefined,
    });
  });

  it('passes the range token through WITHOUT minutes conversion (key difference from getStorageSummaryCharts)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getStorageSummaryTrend('5m');

    // NOTE: getStorageSummaryCharts('5m') would build range=5 (minutes via
    // timeRangeToMinutes). getStorageSummaryTrend keeps the raw token '5m'.
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/storage-summary?range=5m', {
      signal: undefined,
    });
  });

  it.each(ALL_TIME_RANGES)(
    'forwards TimeRange="%s" verbatim into range param (no minutes conversion)',
    async (range) => {
      apiFetchJSONMock.mockResolvedValueOnce({} as never);

      await ChartsAPI.getStorageSummaryTrend(range);

      expect(apiFetchJSONMock).toHaveBeenCalledWith(`/api/charts/storage-summary?range=${range}`, {
        signal: undefined,
      });
    },
  );

  it('forwards an AbortSignal through to apiFetchJSON', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);
    const controller = new AbortController();

    await ChartsAPI.getStorageSummaryTrend('24h', controller.signal);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/storage-summary?range=24h', {
      signal: controller.signal,
    });
  });

  it('passes signal=undefined to apiFetchJSON when no AbortSignal is supplied', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getStorageSummaryTrend('12h');

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/charts/storage-summary?range=12h', {
      signal: undefined,
    });
  });

  it('returns the parsed StorageSummaryTrendResponse payload verbatim from apiFetchJSON', async () => {
    const payload: StorageSummaryTrendResponse = {
      capacity: [
        { timestamp: 1700000000000, value: 80 },
        { timestamp: 1700000060000, value: 81 },
      ],
      timestamp: 1700000060000,
      stats: {
        oldestDataTimestamp: 1699900000000,
        range: '24h',
        rangeSeconds: 86400,
        metricsStoreEnabled: true,
        primarySourceHint: 'store',
      },
    };
    apiFetchJSONMock.mockResolvedValueOnce(payload as never);

    const result = await ChartsAPI.getStorageSummaryTrend();

    expect(result).toBe(payload);
  });
});
