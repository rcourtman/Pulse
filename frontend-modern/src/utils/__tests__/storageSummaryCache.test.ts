import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { StorageSummaryChartsResponse, TimeRange } from '@/api/charts';
import { eventBus } from '@/stores/events';
import { getOrgID, setOrgID } from '@/utils/apiClient';
import { fetchStorageSummaryAndCache, readStorageSummaryCache } from '@/utils/storageSummaryCache';

const mockGetStorageSummaryCharts = vi.fn();

vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return {
    ...actual,
    ChartsAPI: {
      ...actual.ChartsAPI,
      getStorageSummaryCharts: (...args: unknown[]) => mockGetStorageSummaryCharts(...args),
    },
  };
});

const makeResponse = (seed = 1): StorageSummaryChartsResponse => ({
  pools: {
    [`pool-${seed}`]: {
      name: `pool-${seed}`,
      usage: [{ timestamp: seed, value: 50 }],
      used: [{ timestamp: seed, value: 5 }],
      avail: [{ timestamp: seed, value: 5 }],
    },
  },
  disks: {},
  stats: { oldestDataTimestamp: seed },
});

describe('storageSummaryCache', () => {
  beforeEach(() => {
    mockGetStorageSummaryCharts.mockReset();
    localStorage.clear();
    setOrgID('default');
    // The module exposes no test-only reset helper, so rely on its own
    // sanctioned clearing path (the org_switched subscriber) to give every
    // test a clean in-memory cache and an empty in-flight table.
    eventBus.emit('org_switched', 'default');
  });

  describe('readStorageSummaryCache', () => {
    it('returns null on a cache miss before any fetch', () => {
      expect(readStorageSummaryCache('1h')).toBeNull();
      expect(readStorageSummaryCache('1h', 'node-1')).toBeNull();
    });

    it('returns the cached response after a successful fetch', async () => {
      const response = makeResponse(7);
      mockGetStorageSummaryCharts.mockResolvedValueOnce(response);

      await fetchStorageSummaryAndCache('1h');

      expect(readStorageSummaryCache('1h')).toBe(response);
    });
  });

  describe('fetchStorageSummaryAndCache', () => {
    it('calls ChartsAPI with the range and nodeId option and caches the result', async () => {
      const response = makeResponse();
      mockGetStorageSummaryCharts.mockResolvedValueOnce(response);

      const result = await fetchStorageSummaryAndCache('1h', { nodeId: 'node-1' });

      expect(result).toBe(response);
      expect(mockGetStorageSummaryCharts).toHaveBeenCalledTimes(1);
      expect(mockGetStorageSummaryCharts).toHaveBeenCalledWith('1h', expect.any(AbortSignal), {
        nodeId: 'node-1',
      });
      expect(readStorageSummaryCache('1h', 'node-1')).toBe(response);
    });

    it('passes an undefined nodeId when no node scope is supplied', async () => {
      mockGetStorageSummaryCharts.mockResolvedValueOnce(makeResponse());

      await fetchStorageSummaryAndCache('24h');

      expect(mockGetStorageSummaryCharts).toHaveBeenCalledWith('24h', expect.any(AbortSignal), {
        nodeId: undefined,
      });
    });

    it('deduplicates concurrent requests for the same range and node', async () => {
      let resolveFetch: ((value: StorageSummaryChartsResponse) => void) | undefined;
      const response = makeResponse(42);
      mockGetStorageSummaryCharts.mockImplementationOnce(
        () =>
          new Promise<StorageSummaryChartsResponse>((resolve) => {
            resolveFetch = resolve;
          }),
      );

      const first = fetchStorageSummaryAndCache('1h');
      const second = fetchStorageSummaryAndCache('1h');

      // Two concurrent callers share a single in-flight request.
      expect(mockGetStorageSummaryCharts).toHaveBeenCalledTimes(1);

      resolveFetch?.(response);
      const [firstResult, secondResult] = await Promise.all([first, second]);

      expect(firstResult).toBe(response);
      expect(secondResult).toBe(response);
      expect(readStorageSummaryCache('1h')).toBe(response);
    });

    it('fetches separately for distinct ranges', async () => {
      mockGetStorageSummaryCharts.mockImplementation((_range: TimeRange) =>
        Promise.resolve(makeResponse(_range === '1h' ? 1 : 24)),
      );

      await fetchStorageSummaryAndCache('1h');
      await fetchStorageSummaryAndCache('24h');

      expect(mockGetStorageSummaryCharts).toHaveBeenCalledTimes(2);
      expect(readStorageSummaryCache('1h')).not.toBeNull();
      expect(readStorageSummaryCache('24h')).not.toBeNull();
      expect(readStorageSummaryCache('1h')).not.toEqual(readStorageSummaryCache('24h'));
    });

    it('fetches separately per node scope', async () => {
      mockGetStorageSummaryCharts.mockImplementation(
        (_range: TimeRange, _signal: unknown, options?: { nodeId?: string }) =>
          Promise.resolve(makeResponse(options?.nodeId ? 10 : 20)),
      );

      await fetchStorageSummaryAndCache('1h', { nodeId: 'node-1' });
      await fetchStorageSummaryAndCache('1h');

      expect(mockGetStorageSummaryCharts).toHaveBeenCalledTimes(2);
      expect(readStorageSummaryCache('1h', 'node-1')).not.toBeNull();
      expect(readStorageSummaryCache('1h')).not.toBeNull();
      // node-scoped and global entries are distinct.
      expect(readStorageSummaryCache('1h', 'node-1')).not.toBe(readStorageSummaryCache('1h'));
    });

    it('does not poison the cache when the request rejects', async () => {
      mockGetStorageSummaryCharts.mockRejectedValueOnce(new Error('boom'));

      await expect(fetchStorageSummaryAndCache('1h')).rejects.toThrow('boom');

      expect(readStorageSummaryCache('1h')).toBeNull();
    });

    it('clears the in-flight entry on rejection so the next call refetches', async () => {
      mockGetStorageSummaryCharts.mockRejectedValueOnce(new Error('boom'));
      await expect(fetchStorageSummaryAndCache('1h')).rejects.toThrow('boom');

      const response = makeResponse(99);
      mockGetStorageSummaryCharts.mockResolvedValueOnce(response);

      const result = await fetchStorageSummaryAndCache('1h');

      expect(mockGetStorageSummaryCharts).toHaveBeenCalledTimes(2);
      expect(result).toBe(response);
      expect(readStorageSummaryCache('1h')).toBe(response);
    });

    it('isolates cache entries per org scope', async () => {
      mockGetStorageSummaryCharts.mockResolvedValue(makeResponse(1));

      setOrgID('org-a');
      await fetchStorageSummaryAndCache('1h');
      expect(getOrgID()).toBe('org-a');
      expect(readStorageSummaryCache('1h')).not.toBeNull();

      // Switching orgs (without emitting org_switched) must not surface the
      // other org's cached entry.
      setOrgID('org-b');
      expect(readStorageSummaryCache('1h')).toBeNull();

      // Switching back to the original org still finds its entry.
      setOrgID('org-a');
      expect(readStorageSummaryCache('1h')).not.toBeNull();

      expect(mockGetStorageSummaryCharts).toHaveBeenCalledTimes(1);
    });

    it('clears the in-memory cache when the org_switched event fires', async () => {
      mockGetStorageSummaryCharts.mockResolvedValueOnce(makeResponse(5));

      await fetchStorageSummaryAndCache('1h');
      expect(readStorageSummaryCache('1h')).not.toBeNull();

      eventBus.emit('org_switched', 'org-b');

      expect(readStorageSummaryCache('1h')).toBeNull();
    });

    it('clears in-flight dedupe state on org switch so a fresh fetch is issued', async () => {
      const first = makeResponse(1);
      const second = makeResponse(2);
      mockGetStorageSummaryCharts.mockResolvedValueOnce(first);
      await fetchStorageSummaryAndCache('1h');

      eventBus.emit('org_switched', 'default');

      mockGetStorageSummaryCharts.mockResolvedValueOnce(second);
      await fetchStorageSummaryAndCache('1h');

      expect(mockGetStorageSummaryCharts).toHaveBeenCalledTimes(2);
      expect(readStorageSummaryCache('1h')).toBe(second);
    });

    it('aborts an in-flight summary request when the organization changes', async () => {
      let signal: AbortSignal | undefined;
      mockGetStorageSummaryCharts.mockImplementationOnce(
        (_range: TimeRange, requestSignal: AbortSignal) => {
          signal = requestSignal;
          return new Promise<StorageSummaryChartsResponse>((_resolve, reject) => {
            requestSignal.addEventListener('abort', () => {
              reject(new DOMException('Aborted', 'AbortError'));
            });
          });
        },
      );

      const request = fetchStorageSummaryAndCache('1h');
      expect(signal?.aborted).toBe(false);

      eventBus.emit('org_switched', 'org-b');

      expect(signal?.aborted).toBe(true);
      await expect(request).rejects.toMatchObject({ name: 'AbortError' });
      expect(readStorageSummaryCache('1h')).toBeNull();
    });

    it('keeps only the 20 most recently used node and range summaries', async () => {
      mockGetStorageSummaryCharts.mockImplementation(
        (_range: TimeRange, _signal: AbortSignal, options?: { nodeId?: string }) =>
          Promise.resolve(makeResponse(Number(options?.nodeId?.split('-').at(-1)) || 1)),
      );

      for (let index = 0; index <= 20; index += 1) {
        await fetchStorageSummaryAndCache('1h', { nodeId: `node-${index}` });
      }

      expect(readStorageSummaryCache('1h', 'node-0')).toBeNull();
      expect(readStorageSummaryCache('1h', 'node-1')).not.toBeNull();
      expect(readStorageSummaryCache('1h', 'node-20')).not.toBeNull();
    });
  });
});
