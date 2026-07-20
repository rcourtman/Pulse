import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { MetricPoint, TimeRange, WorkloadChartsResponse } from '@/api/charts';
import { eventBus } from '@/stores/events';
import {
  __resetWorkloadsSummaryCacheForTests,
  fetchWorkloadsSummaryAndCache,
  hasFreshWorkloadsSummaryCache,
  persistWorkloadsSummaryCache,
  readInMemoryWorkloadsSummaryCache,
  readWorkloadsSummaryCache,
  workloadsSummaryCacheScopeKey,
} from '@/utils/workloadsSummaryCache';

// Module-private helpers (`normalizeNodeScope`, `normalizeMaxPoints`,
// `resolveOrgScope`, `storageCacheKeyFor`, `trimPoints`, `toCachedChartData`,
// `writeInMemoryWorkloadsSummaryCache`) are not exported, so they are exercised
// transitively through the exported entry points below, asserting on their
// observable outputs (cache keys, API call args, trimmed persisted arrays).

const mockGetWorkloadCharts = vi.fn();

vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return {
    ...actual,
    ChartsAPI: {
      ...actual.ChartsAPI,
      getWorkloadCharts: (...args: unknown[]) => mockGetWorkloadCharts(...args),
    },
  };
});

// `getOrgID` is the fallback inside `resolveOrgScope` (`orgScope ?? getOrgID()`).
// Mocking it to a sentinel lets us pin the nullish-orgScope arm deterministically.
vi.mock('@/utils/apiClient', () => ({
  getOrgID: () => 'test-org',
}));

// NOTE: `@/stores/events` is deliberately NOT mocked so the module's real
// `eventBus.on('org_switched', ...)` side-effect registers, letting us exercise
// the org-switch cache-invalidation handler by emitting the event.

const ORG_FROM_GETORGID = 'test-org';
const VERSION = 6;
const PREFIX = 'pulse.workloadsSummaryCharts.';

// Mirrors the module's private `resolveOrgScope` (with the mocked getOrgID) for
// setup-only key construction; assertions are on observable values, never on
// this helper's output.
const resolveOrgScopeForSetup = (orgScope?: string | null): string => {
  const raw = orgScope ?? ORG_FROM_GETORGID;
  const trimmed = (raw || '').trim();
  return trimmed || 'default';
};

const storageKeyFor = (
  range: TimeRange,
  normalizedNodeScope: string,
  orgScope?: string | null,
): string =>
  `${PREFIX}${encodeURIComponent(resolveOrgScopeForSetup(orgScope))}::${range}::${encodeURIComponent(normalizedNodeScope || '__all__')}`;

const metric = (value: number): MetricPoint => ({ timestamp: value, value });

const points = (n: number): MetricPoint[] => Array.from({ length: n }, (_, i) => metric(i));

const makeResponse = (cpu?: MetricPoint[]): WorkloadChartsResponse => ({
  data: {
    w1: {
      cpu: cpu ?? [metric(1), metric(2)],
      memory: [metric(3), metric(4)],
      disk: [metric(5), metric(6)],
      diskread: [metric(7), metric(8)],
      diskwrite: [metric(9), metric(10)],
      netin: [metric(11), metric(12)],
      netout: [metric(13), metric(14)],
    },
  },
  dockerData: {},
  guestTypes: {},
  timestamp: 1,
  stats: { oldestDataTimestamp: 1 },
});

const emptyResponse = (): WorkloadChartsResponse => ({
  data: {},
  dockerData: {},
  guestTypes: {},
  timestamp: 1,
  stats: { oldestDataTimestamp: 0 },
});

describe('workloadsSummaryCache branch coverage (supplemental)', () => {
  beforeEach(() => {
    mockGetWorkloadCharts.mockReset();
    mockGetWorkloadCharts.mockResolvedValue(emptyResponse());
    __resetWorkloadsSummaryCacheForTests();
    localStorage.clear();
  });

  describe('normalizeNodeScope (via workloadsSummaryCacheScopeKey)', () => {
    it('trims surrounding whitespace from a defined node scope', () => {
      expect(workloadsSummaryCacheScopeKey('1h', '  node-1  ', 'acme')).toBe(
        `${VERSION}::acme::1h::node-1`,
      );
    });

    it('coerces undefined node scope to empty string via the ?. ?? chain', () => {
      expect(workloadsSummaryCacheScopeKey('1h', undefined, 'acme')).toBe(`${VERSION}::acme::1h::`);
    });

    it('coerces null node scope to empty string (optional chaining treats null as nullish)', () => {
      expect(workloadsSummaryCacheScopeKey('1h', null, 'acme')).toBe(`${VERSION}::acme::1h::`);
    });

    it('collapses a whitespace-only node scope to empty string', () => {
      expect(workloadsSummaryCacheScopeKey('1h', '   ', 'acme')).toBe(`${VERSION}::acme::1h::`);
    });

    it('uses the defaulted params when only range is supplied', () => {
      // nodeScope defaults '' and orgScope defaults undefined -> getOrgID().
      expect(workloadsSummaryCacheScopeKey('1h')).toBe(`${VERSION}::${ORG_FROM_GETORGID}::1h::`);
    });
  });

  describe('resolveOrgScope (via workloadsSummaryCacheScopeKey)', () => {
    it('honours an explicitly provided org scope without calling getOrgID', () => {
      expect(workloadsSummaryCacheScopeKey('1h', '', 'custom')).toBe(`${VERSION}::custom::1h::`);
    });

    it('falls back to getOrgID() when orgScope is undefined (?? right operand)', () => {
      expect(workloadsSummaryCacheScopeKey('1h', '', undefined)).toBe(
        `${VERSION}::${ORG_FROM_GETORGID}::1h::`,
      );
    });

    it('falls back to getOrgID() when orgScope is null (?? right operand)', () => {
      expect(workloadsSummaryCacheScopeKey('1h', '', null)).toBe(
        `${VERSION}::${ORG_FROM_GETORGID}::1h::`,
      );
    });

    it('normalises an empty-string orgScope to the default org (empty string is not nullish)', () => {
      // '' ?? getOrgID() -> '' (not nullish), then normalizeOrgScope('') -> 'default'.
      expect(workloadsSummaryCacheScopeKey('1h', '', '')).toBe(`${VERSION}::default::1h::`);
    });

    it('trims and keeps a whitespace-padded org scope', () => {
      expect(workloadsSummaryCacheScopeKey('1h', '', '  acme  ')).toBe(`${VERSION}::acme::1h::`);
    });
  });

  describe('storageCacheKeyFor (observable via persisted localStorage key)', () => {
    it('substitutes "__all__" when nodeScope is empty (|| arm false)', () => {
      persistWorkloadsSummaryCache('1h', '', 'acme', makeResponse());
      expect(localStorage.getItem(`${PREFIX}acme::1h::__all__`)).not.toBeNull();
    });

    it('keeps the concrete node scope when it is non-empty (|| arm true)', () => {
      persistWorkloadsSummaryCache('1h', 'node-1', 'acme', makeResponse());
      expect(localStorage.getItem(`${PREFIX}acme::1h::node-1`)).not.toBeNull();
    });

    it('URL-encodes special characters in both org scope and node scope', () => {
      persistWorkloadsSummaryCache('1h', 'a/b', 'a b', makeResponse());
      expect(localStorage.getItem(`${PREFIX}a%20b::1h::a%2Fb`)).not.toBeNull();
    });
  });

  describe('normalizeMaxPoints (via fetchWorkloadsSummaryAndCache call args)', () => {
    const expectCall = async (maxPoints: unknown, expected: number | undefined): Promise<void> => {
      await fetchWorkloadsSummaryAndCache('1h', {
        maxPoints: maxPoints as number | null,
      });
      expect(mockGetWorkloadCharts).toHaveBeenCalledWith('1h', undefined, {
        nodeId: undefined,
        maxPoints: expected,
      });
    };

    it('rounds a positive fractional maxPoints to the nearest integer', async () => {
      await expectCall(180.7, 181);
    });

    it('passes a positive integer maxPoints through unchanged', async () => {
      await expectCall(360, 360);
    });

    it('returns undefined for a non-number type (string)', async () => {
      await expectCall('big', undefined);
    });

    it('returns undefined for null', async () => {
      await expectCall(null, undefined);
    });

    it('returns undefined for NaN (non-finite)', async () => {
      await expectCall(NaN, undefined);
    });

    it('returns undefined for Infinity (non-finite)', async () => {
      await expectCall(Infinity, undefined);
    });

    it('returns undefined for zero (maxPoints <= 0 guard)', async () => {
      await expectCall(0, undefined);
    });

    it('returns undefined for a negative number (maxPoints <= 0 guard)', async () => {
      await expectCall(-5, undefined);
    });
  });

  describe('normalizeNodeScope via fetch (nodeId arg normalization)', () => {
    it('passes the trimmed nodeId to the API', async () => {
      await fetchWorkloadsSummaryAndCache('1h', { nodeId: '  node-1  ' });
      expect(mockGetWorkloadCharts).toHaveBeenCalledWith('1h', undefined, {
        nodeId: 'node-1',
        maxPoints: undefined,
      });
    });

    it('converts an empty/whitespace nodeId to undefined (nodeScope || undefined)', async () => {
      await fetchWorkloadsSummaryAndCache('1h', { nodeId: '   ' });
      expect(mockGetWorkloadCharts).toHaveBeenCalledWith('1h', undefined, {
        nodeId: undefined,
        maxPoints: undefined,
      });
    });

    it('converts a null nodeId to undefined', async () => {
      await fetchWorkloadsSummaryAndCache('1h', { nodeId: null });
      expect(mockGetWorkloadCharts).toHaveBeenCalledWith('1h', undefined, {
        nodeId: undefined,
        maxPoints: undefined,
      });
    });
  });

  describe('resolveOrgScope via fetch (org routing)', () => {
    it('persists under the explicitly provided org scope', async () => {
      const response = makeResponse();
      mockGetWorkloadCharts.mockResolvedValueOnce(response);
      await fetchWorkloadsSummaryAndCache('1h', { orgScope: 'acme' });
      expect(readInMemoryWorkloadsSummaryCache('1h', '', 'acme')).toBe(response);
    });

    it('persists under getOrgID() when orgScope is omitted', async () => {
      const response = makeResponse();
      mockGetWorkloadCharts.mockResolvedValueOnce(response);
      await fetchWorkloadsSummaryAndCache('1h', {});
      expect(readInMemoryWorkloadsSummaryCache('1h', '', ORG_FROM_GETORGID)).toBe(response);
    });
  });

  describe('trimPoints (via persisted+read storage arrays)', () => {
    const persistAndReadCpu = async (cpu: MetricPoint[] | undefined) => {
      const response: WorkloadChartsResponse = {
        data: { w1: { cpu } },
        dockerData: {},
        guestTypes: {},
        timestamp: 1,
        stats: { oldestDataTimestamp: 0 },
      };
      persistWorkloadsSummaryCache('1h', '', 'acme', response);
      const read = readWorkloadsSummaryCache('1h', '', 'acme');
      return read?.data.w1?.cpu ?? null;
    };

    it('returns [] when the points array is empty', async () => {
      expect(await persistAndReadCpu([])).toEqual([]);
    });

    it('returns [] when the points array is undefined (!points arm)', async () => {
      expect(await persistAndReadCpu(undefined)).toEqual([]);
    });

    it('returns a short array unchanged (length <= max arm)', async () => {
      const cpu = await persistAndReadCpu([metric(10), metric(20)]);
      expect(cpu).toEqual([metric(10), metric(20)]);
    });

    it('downsamples 361 points without appending/slicing (step loop, no last-append, no final trim)', async () => {
      // step=ceil(361/360)=2 -> 181 samples; last sampled index == sliced last -> no append;
      // 181 <= 360 -> returned as-is.
      const cpu = await persistAndReadCpu(points(361));
      expect(cpu?.length).toBe(181);
      expect(cpu?.[0]).toEqual(metric(0));
      expect(cpu?.[180]).toEqual(metric(360));
    });

    it('appends the final point when the step loop misses it (last-append arm, no final trim)', async () => {
      // 362 points -> step 2 -> 181 sampled; last sampled (360) != sliced last (361) -> push;
      // 182 <= 360 -> returned as-is.
      const cpu = await persistAndReadCpu(points(362));
      expect(cpu?.length).toBe(182);
      expect(cpu?.[0]).toEqual(metric(0));
      expect(cpu?.[181]).toEqual(metric(361));
    });

    it('trims back to exactly max when downsample+append overflows (final slice arm)', async () => {
      // 720 points -> step 2 -> 360 sampled + 1 appended = 361 -> slice(1) -> 360.
      const cpu = await persistAndReadCpu(points(720));
      expect(cpu?.length).toBe(360);
      expect(cpu?.[0]).toEqual(metric(2));
      expect(cpu?.[359]).toEqual(metric(719));
    });

    it('trims a large array (1000 pts) down to max keeping the most recent tail', async () => {
      const cpu = await persistAndReadCpu(points(1000));
      expect(cpu?.length).toBe(360);
      expect(cpu?.[cpu.length - 1]).toEqual(metric(999));
    });
  });

  describe('readInMemoryWorkloadsSummaryCache', () => {
    it('returns null on a miss (?? null arm)', () => {
      expect(readInMemoryWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
    });

    it('returns the stored response on a hit', () => {
      const response = makeResponse();
      persistWorkloadsSummaryCache('1h', 'n1', 'acme', response);
      expect(readInMemoryWorkloadsSummaryCache('1h', 'n1', 'acme')).toBe(response);
    });

    it('returns null when the org scope differs', () => {
      const response = makeResponse();
      persistWorkloadsSummaryCache('1h', 'n1', 'acme', response);
      expect(readInMemoryWorkloadsSummaryCache('1h', 'n1', 'other')).toBeNull();
    });

    it('resolves the default-omitted orgScope through getOrgID()', () => {
      const response = makeResponse();
      persistWorkloadsSummaryCache('1h', 'n1', undefined, response);
      expect(readInMemoryWorkloadsSummaryCache('1h', 'n1')).toBe(response);
    });
  });

  describe('writeInMemoryWorkloadsSummaryCache eviction', () => {
    it('evicts the oldest entry once the 20-entry cap is exceeded', () => {
      const first = makeResponse([metric(1)]);
      persistWorkloadsSummaryCache('1h', 'node-0', 'acme', first);
      // Fill distinct scopes up to the cap.
      for (let i = 1; i < 20; i += 1) {
        persistWorkloadsSummaryCache('1h', `node-${i}`, 'acme', makeResponse());
      }
      // All 20 still resident.
      expect(readInMemoryWorkloadsSummaryCache('1h', 'node-0', 'acme')).toBe(first);
      expect(readInMemoryWorkloadsSummaryCache('1h', 'node-19', 'acme')).not.toBeNull();

      // 21st distinct scope triggers eviction of the oldest (node-0).
      const twentyFirst = makeResponse();
      persistWorkloadsSummaryCache('1h', 'node-20', 'acme', twentyFirst);
      expect(readInMemoryWorkloadsSummaryCache('1h', 'node-0', 'acme')).toBeNull();
      expect(readInMemoryWorkloadsSummaryCache('1h', 'node-20', 'acme')).toBe(twentyFirst);
    });

    it('overwrites in place without evicting when the scope key already exists', () => {
      const a = makeResponse([metric(1)]);
      const b = makeResponse([metric(2)]);
      for (let i = 0; i < 20; i += 1) {
        persistWorkloadsSummaryCache('1h', `node-${i}`, 'acme', a);
      }
      // Re-persist an existing key -> has() is true so no eviction; value updates.
      persistWorkloadsSummaryCache('1h', 'node-0', 'acme', b);
      expect(readInMemoryWorkloadsSummaryCache('1h', 'node-0', 'acme')).toBe(b);
      expect(readInMemoryWorkloadsSummaryCache('1h', 'node-19', 'acme')).toBe(a);
    });
  });

  describe('persistWorkloadsSummaryCache error / edge arms', () => {
    it('skips localStorage entirely when window is undefined (SSR guard) but still warms memory', () => {
      vi.stubGlobal('window', undefined);
      try {
        persistWorkloadsSummaryCache('1h', '', 'acme', makeResponse());
        expect(localStorage.length).toBe(0);
      } finally {
        vi.unstubAllGlobals();
      }
      // Memory was written before the SSR guard.
      expect(readInMemoryWorkloadsSummaryCache('1h', '', 'acme')).not.toBeNull();
    });

    it('stores an empty data record when response.data is undefined (|| {} arm)', () => {
      persistWorkloadsSummaryCache('1h', '', 'acme', {
        dockerData: {},
        timestamp: 1,
        stats: { oldestDataTimestamp: 0 },
      } as unknown as WorkloadChartsResponse);
      expect(readWorkloadsSummaryCache('1h', '', 'acme')?.data).toEqual({});
    });

    it('stores an empty dockerData record when response.dockerData is undefined (|| {} arm)', () => {
      persistWorkloadsSummaryCache('1h', '', 'acme', {
        data: {},
        timestamp: 1,
        stats: { oldestDataTimestamp: 0 },
      } as unknown as WorkloadChartsResponse);
      expect(readWorkloadsSummaryCache('1h', '', 'acme')?.dockerData).toEqual({});
    });

    it('persists dockerData entries through toCachedChartData', () => {
      persistWorkloadsSummaryCache('1h', '', 'acme', {
        data: {},
        dockerData: { c1: { cpu: [metric(1), metric(2)] } },
        timestamp: 1,
        stats: { oldestDataTimestamp: 0 },
      });
      expect(readWorkloadsSummaryCache('1h', '', 'acme')?.dockerData?.c1?.cpu).toEqual([
        metric(1),
        metric(2),
      ]);
    });

    it('drops the localStorage entry when the serialized payload exceeds the char cap', () => {
      // A single workload id whose key name alone pushes serialization past
      // WORKLOADS_SUMMARY_CACHE_MAX_CHARS (900_000) -> removeItem + early return.
      const hugeId = 'x'.repeat(1_000_000);
      persistWorkloadsSummaryCache('1h', '', 'acme', {
        data: { [hugeId]: { cpu: [metric(1)] } },
        dockerData: {},
        timestamp: 1,
        stats: { oldestDataTimestamp: 0 },
      });
      expect(localStorage.length).toBe(0);
      // Memory is populated before the size check.
      expect(readInMemoryWorkloadsSummaryCache('1h', '', 'acme')).not.toBeNull();
    });

    it('swallows a localStorage.setItem failure (catch arm) without throwing', () => {
      const spy = vi.spyOn(Storage.prototype, 'setItem').mockImplementationOnce(() => {
        throw new Error('QuotaExceededError');
      });
      const response = makeResponse();
      expect(() => persistWorkloadsSummaryCache('1h', '', 'acme', response)).not.toThrow();
      // Not persisted to storage (setItem threw)...
      expect(localStorage.length).toBe(0);
      // ...but memory was warmed before the try block.
      expect(readInMemoryWorkloadsSummaryCache('1h', '', 'acme')).toBe(response);
      spy.mockRestore();
    });
  });

  describe('readWorkloadsSummaryCache', () => {
    afterEach(() => vi.restoreAllMocks());

    it('returns null on the SSR path (window undefined) without touching storage', () => {
      persistWorkloadsSummaryCache('1h', '', 'acme', makeResponse());
      vi.stubGlobal('window', undefined);
      try {
        expect(readWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
      } finally {
        vi.unstubAllGlobals();
      }
      // Storage entry left intact (read returned before touching localStorage).
      expect(localStorage.length).toBe(1);
    });

    it('returns null when no entry exists (!raw arm)', () => {
      expect(readWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
    });

    it('returns null when JSON.parse throws (catch arm); leaves the malformed entry in place', () => {
      // The catch arm only swallows and returns null; it does NOT removeItem.
      const key = storageKeyFor('1h', '', 'acme');
      localStorage.setItem(key, '{ not valid json');
      expect(readWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
      expect(localStorage.getItem(key)).toBe('{ not valid json');
    });

    const writePayload = (
      range: TimeRange,
      nodeScope: string,
      orgScope: string | undefined,
      payload: Record<string, unknown>,
    ): void => {
      const normalizedNode = nodeScope?.trim() ?? '';
      localStorage.setItem(storageKeyFor(range, normalizedNode, orgScope), JSON.stringify(payload));
    };

    it('rejects and removes an entry whose version mismatches', () => {
      const key = storageKeyFor('1h', '', 'acme');
      writePayload('1h', '', 'acme', {
        version: 999,
        range: '1h',
        nodeScope: '',
        cachedAt: Date.now(),
        data: {},
        dockerData: {},
      });
      expect(readWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
      expect(localStorage.getItem(key)).toBeNull();
    });

    it('rejects and removes an entry whose range mismatches', () => {
      const key = storageKeyFor('1h', '', 'acme');
      writePayload('1h', '', 'acme', {
        version: VERSION,
        range: '4h',
        nodeScope: '',
        cachedAt: Date.now(),
        data: {},
        dockerData: {},
      });
      expect(readWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
      expect(localStorage.getItem(key)).toBeNull();
    });

    it('rejects and removes an entry whose nodeScope mismatches', () => {
      const key = storageKeyFor('1h', '', 'acme');
      writePayload('1h', '', 'acme', {
        version: VERSION,
        range: '1h',
        nodeScope: 'different',
        cachedAt: Date.now(),
        data: {},
        dockerData: {},
      });
      expect(readWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
      expect(localStorage.getItem(key)).toBeNull();
    });

    it('rejects and removes an entry whose cachedAt is not a number', () => {
      const key = storageKeyFor('1h', '', 'acme');
      writePayload('1h', '', 'acme', {
        version: VERSION,
        range: '1h',
        nodeScope: '',
        cachedAt: 'not-a-number',
        data: {},
        dockerData: {},
      });
      expect(readWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
      expect(localStorage.getItem(key)).toBeNull();
    });

    it('rejects and removes a stale entry past the 5-minute TTL', () => {
      const key = storageKeyFor('1h', '', 'acme');
      const stale = Date.now() - (5 * 60_000 + 1);
      writePayload('1h', '', 'acme', {
        version: VERSION,
        range: '1h',
        nodeScope: '',
        cachedAt: stale,
        data: {},
        dockerData: {},
      });
      expect(readWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
      expect(localStorage.getItem(key)).toBeNull();
    });

    it('accepts a fresh entry exactly at the TTL boundary (age == max age)', () => {
      const now = 10_000_000;
      vi.spyOn(Date, 'now').mockReturnValue(now);
      writePayload('1h', '', 'acme', {
        version: VERSION,
        range: '1h',
        nodeScope: '',
        cachedAt: now - 5 * 60_000,
        data: { w1: { cpu: [metric(7)] } },
        dockerData: {},
      });
      const read = readWorkloadsSummaryCache('1h', '', 'acme');
      expect(read?.data.w1?.cpu).toEqual([metric(7)]);
    });

    it('reconstructs the response with reset guestTypes/stats and the cached timestamp', () => {
      const now = 50_000;
      vi.spyOn(Date, 'now').mockReturnValue(now);
      writePayload('1h', '', 'acme', {
        version: VERSION,
        range: '1h',
        nodeScope: '',
        cachedAt: 12345,
        data: {},
        dockerData: {},
      });
      const read = readWorkloadsSummaryCache('1h', '', 'acme');
      expect(read).toEqual({
        data: {},
        dockerData: {},
        guestTypes: {},
        timestamp: 12345,
        stats: { oldestDataTimestamp: 0 },
      });
    });

    it('warms the in-memory cache as a side effect of a successful read', () => {
      const now = 99_000;
      vi.spyOn(Date, 'now').mockReturnValue(now);
      writePayload('1h', '', 'acme', {
        version: VERSION,
        range: '1h',
        nodeScope: '',
        cachedAt: now,
        data: {},
        dockerData: {},
      });
      expect(readInMemoryWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
      const read = readWorkloadsSummaryCache('1h', '', 'acme');
      expect(read).not.toBeNull();
      expect(readInMemoryWorkloadsSummaryCache('1h', '', 'acme')).toBe(read);
    });
  });

  describe('hasFreshWorkloadsSummaryCache', () => {
    afterEach(() => vi.restoreAllMocks());

    it('returns false when neither memory nor storage has an entry (both || arms false)', () => {
      expect(hasFreshWorkloadsSummaryCache('1h', '', 'acme')).toBe(false);
    });

    it('short-circuits true on a memory hit (|| left arm)', () => {
      const response = makeResponse();
      persistWorkloadsSummaryCache('1h', '', 'acme', response);
      // Drop storage so only memory holds it.
      localStorage.clear();
      expect(hasFreshWorkloadsSummaryCache('1h', '', 'acme')).toBe(true);
    });

    it('falls through to storage and returns true on a storage hit (|| right arm)', () => {
      const now = 1_000;
      vi.spyOn(Date, 'now').mockReturnValue(now);
      persistWorkloadsSummaryCache('1h', '', 'acme', makeResponse());
      // Reset memory (keeps localStorage) so only storage holds it.
      __resetWorkloadsSummaryCacheForTests();
      expect(hasFreshWorkloadsSummaryCache('1h', '', 'acme')).toBe(true);
    });
  });

  describe('fetchWorkloadsSummaryAndCache dedupe / signal arms', () => {
    it('deduplicates concurrent same-key fetches without a signal (inFlight return)', async () => {
      const response = makeResponse();
      mockGetWorkloadCharts.mockReset();
      mockGetWorkloadCharts.mockResolvedValue(response);

      const first = fetchWorkloadsSummaryAndCache('1h', { maxPoints: 180 });
      const second = fetchWorkloadsSummaryAndCache('1h', { maxPoints: 180 });

      await expect(Promise.all([first, second])).resolves.toEqual([response, response]);
      expect(mockGetWorkloadCharts).toHaveBeenCalledTimes(1);
    });

    it('clears inFlight on settlement so a later call fetches again (finally arm)', async () => {
      mockGetWorkloadCharts.mockReset();
      mockGetWorkloadCharts.mockResolvedValue(makeResponse());

      await fetchWorkloadsSummaryAndCache('1h', { maxPoints: 180 });
      await fetchWorkloadsSummaryAndCache('1h', { maxPoints: 180 });

      expect(mockGetWorkloadCharts).toHaveBeenCalledTimes(2);
    });

    it('bypasses dedupe when an AbortSignal is supplied (both !signal guards false)', async () => {
      mockGetWorkloadCharts.mockReset();
      mockGetWorkloadCharts.mockResolvedValue(makeResponse());

      const controller = new AbortController();
      const p1 = fetchWorkloadsSummaryAndCache('1h', { signal: controller.signal });
      const p2 = fetchWorkloadsSummaryAndCache('1h', { signal: controller.signal });

      await Promise.all([p1, p2]);
      expect(mockGetWorkloadCharts).toHaveBeenCalledTimes(2);
    });

    it('persists the fetched response before resolving', async () => {
      const response = makeResponse();
      mockGetWorkloadCharts.mockReset();
      mockGetWorkloadCharts.mockResolvedValue(response);

      const result = await fetchWorkloadsSummaryAndCache('1h', {
        nodeId: 'n1',
        orgScope: 'acme',
      });
      expect(result).toBe(response);
      expect(readInMemoryWorkloadsSummaryCache('1h', 'n1', 'acme')).toBe(response);
    });
  });

  describe('org_switched event handler', () => {
    it('clears the in-memory cache when org_switched is emitted', () => {
      const response = makeResponse();
      persistWorkloadsSummaryCache('1h', 'n1', 'acme', response);
      expect(readInMemoryWorkloadsSummaryCache('1h', 'n1', 'acme')).toBe(response);

      eventBus.emit('org_switched', 'new-org');

      expect(readInMemoryWorkloadsSummaryCache('1h', 'n1', 'acme')).toBeNull();
    });

    it('clears in-flight fetch tracking so the next request is not deduped', async () => {
      mockGetWorkloadCharts.mockReset();
      mockGetWorkloadCharts.mockResolvedValue(makeResponse());

      // Kick off a never-resolving first fetch so inFlight is populated.
      let resolveFirst!: (v: WorkloadChartsResponse) => void;
      mockGetWorkloadCharts.mockReturnValueOnce(
        new Promise<WorkloadChartsResponse>((resolve) => {
          resolveFirst = resolve;
        }),
      );
      void fetchWorkloadsSummaryAndCache('1h', { maxPoints: 180 });

      // Emit org_switched -> inFlight map is cleared.
      eventBus.emit('org_switched', 'new-org');

      // A new concurrent call (same key) must NOT be deduped -> 2nd API call.
      const p2 = fetchWorkloadsSummaryAndCache('1h', { maxPoints: 180 });
      expect(mockGetWorkloadCharts).toHaveBeenCalledTimes(2);

      // Allow both promises to settle.
      mockGetWorkloadCharts.mockResolvedValueOnce(makeResponse());
      resolveFirst(makeResponse());
      await p2;
    });
  });

  describe('__resetWorkloadsSummaryCacheForTests', () => {
    it('clears the in-memory cache', () => {
      persistWorkloadsSummaryCache('1h', '', 'acme', makeResponse());
      expect(readInMemoryWorkloadsSummaryCache('1h', '', 'acme')).not.toBeNull();
      __resetWorkloadsSummaryCacheForTests();
      expect(readInMemoryWorkloadsSummaryCache('1h', '', 'acme')).toBeNull();
    });
  });
});
