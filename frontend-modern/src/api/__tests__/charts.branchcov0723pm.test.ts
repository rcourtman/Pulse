/**
 * Branch-coverage tests for the two currently-uncalled ChartsAPI surface points:
 *   - ChartsAPI.getStorageSummaryCharts  (function never invoked by existing specs)
 *   - timeRangeToMinutes                 (module-private helper, never invoked;
 *                                         not exported, so exercised solely
 *                                         through getStorageSummaryCharts by
 *                                         asserting the resulting ?range=<minutes>
 *                                         query value for every switch arm)
 *
 * Mock harness matches charts.branchcov0718.test.ts / chartsApi.test.ts:
 *   vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }))
 * No real network call is made. Nothing already covered by charts.test.ts,
 * chartsApi.test.ts or charts.branchcov0718.test.ts is re-asserted here.
 *
 * Branches exercised:
 *   getStorageSummaryCharts:
 *     - range_ default ('1h') when called with no args
 *     - signal present vs undefined (always wrapped in { signal } either way)
 *     - options undefined entirely  -> `node=` omitted
 *     - options.nodeId undefined     -> `node=` omitted
 *     - options.nodeId ''            -> `node=` omitted (falsy branch)
 *     - options.nodeId truthy string -> `node=<id>` appended
 *     - special chars in nodeId URL-encoded by URLSearchParams.toString
 *     - combined range + node + signal in a single request
 *     - response with EMPTY pools/disks returned verbatim (empty arm)
 *     - fully-populated response returned verbatim
 *     - rejection from apiFetchJSON propagates verbatim (error arm)
 *   timeRangeToMinutes (observed via the ?range=<minutes> value):
 *     - every recognised token: 5m/15m/30m/1h/4h/12h/24h/7d/30d
 *     - default fallback for an UNRECOGNISED token -> 60
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import { ChartsAPI, type StorageSummaryChartsResponse, type TimeRange } from '@/api/charts';
import { apiFetchJSON } from '@/utils/apiClient';

describe('ChartsAPI.getStorageSummaryCharts — branch coverage', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('routes to /api/storage-charts with range=60 (default 1h) and signal undefined when called with no args', async () => {
    // Exercises BOTH the default `range_ = '1h'` parameter AND the default
    // `signal = undefined` / `options = undefined` branches together.
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getStorageSummaryCharts();

    expect(apiFetchJSONMock).toHaveBeenCalledTimes(1);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/storage-charts?range=60', {
      signal: undefined,
    });
  });

  it('forwards an AbortSignal through to apiFetchJSON wrapped in { signal }', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);
    const controller = new AbortController();

    await ChartsAPI.getStorageSummaryCharts('1h', controller.signal);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/storage-charts?range=60', {
      signal: controller.signal,
    });
  });

  it('omits the node query param when options is undefined entirely', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getStorageSummaryCharts('1h', undefined, undefined);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/storage-charts?range=60', {
      signal: undefined,
    });
  });

  it('omits the node query param when options.nodeId is undefined', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getStorageSummaryCharts('1h', undefined, { nodeId: undefined });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/storage-charts?range=60', {
      signal: undefined,
    });
  });

  it('omits the node query param when options.nodeId is an empty string (falsy branch)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getStorageSummaryCharts('1h', undefined, { nodeId: '' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/storage-charts?range=60', {
      signal: undefined,
    });
  });

  it('appends node=<id> when options.nodeId is a non-empty string', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getStorageSummaryCharts('1h', undefined, { nodeId: 'pve1' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/storage-charts?range=60&node=pve1', {
      signal: undefined,
    });
  });

  it('URL-encodes special characters in the node id (URLSearchParams.toString)', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getStorageSummaryCharts('1h', undefined, { nodeId: 'node a/b' });

    // space -> '+', '/' -> '%2F'
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/storage-charts?range=60&node=node+a%2Fb', {
      signal: undefined,
    });
  });

  it('combines range + node + signal in a single request with range-first/node-second ordering', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({} as never);
    const controller = new AbortController();

    await ChartsAPI.getStorageSummaryCharts('4h', controller.signal, { nodeId: 'pve1' });

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/storage-charts?range=240&node=pve1', {
      signal: controller.signal,
    });
  });

  it('returns a payload with EMPTY pools/disks verbatim from apiFetchJSON (empty arm)', async () => {
    const emptyPayload: StorageSummaryChartsResponse = {
      pools: {},
      disks: {},
      stats: { oldestDataTimestamp: 1733700000000 },
    };
    apiFetchJSONMock.mockResolvedValueOnce(emptyPayload as never);

    const result = await ChartsAPI.getStorageSummaryCharts('1h');

    expect(result).toBe(emptyPayload);
    expect(result.pools).toEqual({});
    expect(result.disks).toEqual({});
  });

  it('returns a fully-populated StorageSummaryChartsResponse payload verbatim', async () => {
    const payload: StorageSummaryChartsResponse = {
      pools: {
        'local-zfs': {
          name: 'local-zfs',
          usage: [{ timestamp: 1700000000000, value: 42 }],
          used: [{ timestamp: 1700000000000, value: 2100 }],
          avail: [{ timestamp: 1700000000000, value: 2900 }],
        },
      },
      disks: {
        'serial-1': {
          name: 'sda',
          node: 'pve1',
          temperature: [{ timestamp: 1700000000000, value: 38 }],
        },
      },
      stats: {
        oldestDataTimestamp: 1699900000000,
        range: '1h',
        rangeSeconds: 3600,
        metricsStoreEnabled: true,
      },
    };
    apiFetchJSONMock.mockResolvedValueOnce(payload as never);

    const result = await ChartsAPI.getStorageSummaryCharts('1h');

    expect(result).toBe(payload);
    expect(result.pools['local-zfs'].used![0].value).toBe(2100);
    expect(result.disks['serial-1'].temperature[0].value).toBe(38);
  });

  it('propagates a rejection from apiFetchJSON verbatim (error arm)', async () => {
    const error = new Error('storage-charts 503');
    apiFetchJSONMock.mockRejectedValueOnce(error);

    await expect(ChartsAPI.getStorageSummaryCharts('1h')).rejects.toBe(error);
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/storage-charts?range=60', {
      signal: undefined,
    });
  });
});

describe('timeRangeToMinutes — every switch arm (observed via getStorageSummaryCharts)', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  // Each entry asserts the specific minute value the helper computes for a
  // given range token, observed as the `?range=<minutes>` query value. This
  // is real observable behaviour, not a tautology: the source maps tokens to
  // distinct magic numbers and we pin each one down.
  const cases: Array<{ token: TimeRange; minutes: number }> = [
    { token: '5m', minutes: 5 },
    { token: '15m', minutes: 15 },
    { token: '30m', minutes: 30 },
    { token: '1h', minutes: 60 },
    { token: '4h', minutes: 240 },
    { token: '12h', minutes: 720 },
    { token: '24h', minutes: 1440 },
    { token: '7d', minutes: 10080 },
    { token: '30d', minutes: 43200 },
  ];

  it.each(cases)(
    'maps TimeRange="$token" to range=$minutes minutes in the storage-charts URL',
    async ({ token, minutes }) => {
      apiFetchJSONMock.mockResolvedValueOnce({} as never);

      await ChartsAPI.getStorageSummaryCharts(token);

      expect(apiFetchJSONMock).toHaveBeenCalledWith(`/api/storage-charts?range=${minutes}`, {
        signal: undefined,
      });
    },
  );

  it('falls back to range=60 for an UNRECOGNISED range token (default switch arm)', async () => {
    // '2h' is not in the TimeRange union and not handled by any case in
    // timeRangeToMinutes -> hits the `default: return 60` branch.
    const unknownToken = '2h' as unknown as TimeRange;
    apiFetchJSONMock.mockResolvedValueOnce({} as never);

    await ChartsAPI.getStorageSummaryCharts(unknownToken);

    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/storage-charts?range=60', {
      signal: undefined,
    });
  });
});
