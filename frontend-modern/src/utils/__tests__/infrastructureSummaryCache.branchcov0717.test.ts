import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type {
  ChartData,
  ChartsResponse,
  InfrastructureChartsResponse,
  InfrastructureSummaryMetric,
  TimeRange,
} from '@/api/charts';
import { setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import {
  __resetInfrastructureSummaryFetchesForTests,
  extractInfrastructureSummaryChartMap,
  extractInfrastructureSummaryChartMapFromInfrastructureResponse,
  fetchInfrastructureSummaryAndCache,
  persistInfrastructureSummaryCache,
  readInfrastructureSummaryCache,
} from '@/utils/infrastructureSummaryCache';

// ChartsAPI is mocked (keeping `apiClient`, `orgScope` and `events` real) so the
// fetch entry point is deterministic and offline — the same isolation strategy
// the sibling `infrastructureSummaryCache.branchcov.test.ts` uses. Helpers below
// mirror its fixture shape so the assertions stay concrete.
const mockGetCharts = vi.fn();

vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return {
    ...actual,
    ChartsAPI: {
      ...actual.ChartsAPI,
      getInfrastructureSummaryCharts: (...args: unknown[]) => mockGetCharts(...args),
    },
  };
});

const DEFAULT_METRICS_KEY = 'cpu,memory,disk,diskread,diskwrite,netin,netout';

const cacheKeyForRange = (
  range: TimeRange,
  orgScope = 'default',
  metrics = DEFAULT_METRICS_KEY,
) => `pulse.infrastructureSummaryCharts.${encodeURIComponent(orgScope)}::${range}::${metrics}`;

interface MetricPoint {
  timestamp: number;
  value: number;
}

const metricPoint = (timestamp: number, value: number): MetricPoint => ({ timestamp, value });

// Series of N points starting at `start`, stepping by `step` ms.
const makeSeries = (count: number, start = 0, step = 30_000): MetricPoint[] =>
  Array.from({ length: count }, (_, i) => metricPoint(start + i * step, i));

const now = () => Date.now();

const makeInfraResponse = (
  overrides: Partial<InfrastructureChartsResponse> = {},
): InfrastructureChartsResponse => ({
  nodeData: {},
  timestamp: now(),
  stats: { oldestDataTimestamp: now() - 60_000 },
  ...overrides,
});

// Shape of the JSON payload persistInfrastructureSummaryCache writes; used to
// hand-craft localStorage entries that exercise readInfrastructureSummaryCache's
// coercion branches directly.
interface StoredPayload {
  version: number;
  range: TimeRange;
  cachedAt: number;
  metrics: InfrastructureSummaryMetric[];
  oldestDataTimestamp: number | null;
  charts: Record<string, Record<string, unknown>> | undefined;
}

const buildPayload = (overrides: Partial<StoredPayload> = {}): StoredPayload => ({
  version: 5,
  range: '1h',
  cachedAt: now(),
  metrics: ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'],
  oldestDataTimestamp: null,
  charts: {},
  ...overrides,
});

const storePayload = (
  range: TimeRange,
  payload: StoredPayload,
  orgScope = 'default',
  metrics = DEFAULT_METRICS_KEY,
) => {
  window.localStorage.setItem(cacheKeyForRange(range, orgScope, metrics), JSON.stringify(payload));
};

// Replace a global property with `undefined` for the duration of a callback, then
// restore the original descriptor. Used to exercise the `typeof window ===
// 'undefined'` and `typeof performance === 'undefined'` early-return / fallback
// arms that the jsdom test environment otherwise keeps firmly on the happy path.
const withGlobalUndefined = <T>(name: 'window' | 'performance', fn: () => T): T => {
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, name);
  Object.defineProperty(globalThis, name, {
    value: undefined,
    configurable: true,
    writable: true,
  });
  try {
    return fn();
  } finally {
    if (descriptor) {
      Object.defineProperty(globalThis, name, descriptor);
    } else {
      // No pre-existing descriptor: leave the property undefined (matches the
      // pre-test state we just established) but ensure it stays configurable.
      Object.defineProperty(globalThis, name, {
        value: undefined,
        configurable: true,
        writable: true,
      });
    }
  }
};

describe('extractInfrastructureSummaryChartMap — direct call coverage (uncovered getter)', () => {
  // extractInfrastructureSummaryChartMap is only reached in production via
  // extractInfrastructureSummaryChartMapFromInfrastructureResponse, which always
  // supplies `nodeData: response.nodeData ?? {}`. That keeps the inner
  // `if (response.nodeData)` arm firmly truthy. Calling the getter directly
  // with an absent nodeData covers the falsy arm of all three optional fields.

  it('returns an empty map when called directly with every data field absent', () => {
    const empty = {
      data: {},
      storageData: {},
      timestamp: now(),
      stats: { oldestDataTimestamp: now() },
    } as unknown as ChartsResponse;
    const map = extractInfrastructureSummaryChartMap(empty);
    expect(map.size).toBe(0);
  });

  it('returns an empty map when only nodeData is absent (covers the falsy nodeData arm directly)', () => {
    // agentData/dockerHostData falsy arms are already reached via the wrapper,
    // but the wrapper never forwards an absent nodeData, so this is the only
    // way to take the `if (response.nodeData)` false branch.
    const response = {
      data: {},
      nodeData: undefined,
      storageData: {},
      agentData: { 'agent-1': { cpu: makeSeries(1) } },
      timestamp: now(),
      stats: { oldestDataTimestamp: now() },
    } as unknown as ChartsResponse;
    const map = extractInfrastructureSummaryChartMap(response);
    expect([...map.keys()]).toEqual(['agent-1']);
    expect(map.get('agent-1')?.cpu?.length).toBe(1);
  });

  it('exposes ChartsResponse.data verbatim because the extractor ignores that field', () => {
    // Documents a real-behaviour fact: data on the ChartsResponse is *not*
    // indexed, only nodeData/agentData/dockerHostData are. A direct call with
    // populated `data` and nothing else yields an empty map.
    const response = {
      data: { 'vm-1': { cpu: makeSeries(3) } },
      nodeData: {},
      storageData: {},
      timestamp: now(),
      stats: { oldestDataTimestamp: now() },
    } as unknown as ChartsResponse;
    expect(extractInfrastructureSummaryChartMap(response).size).toBe(0);
  });
});

describe('extractInfrastructureSummaryChartMapFromInfrastructureResponse — uncovered optional-forwarding arms', () => {
  it('forwards an absent nodeData through the ?? and still extracts agent/docker data', () => {
    // The `response.nodeData ?? {}` expression's right operand is only reached
    // when nodeData is absent; the wrapper test only exercised populated nodeData.
    const map = extractInfrastructureSummaryChartMapFromInfrastructureResponse(
      makeInfraResponse({
        nodeData: undefined,
        agentData: { 'agent-1': { memory: makeSeries(2) } },
        dockerHostData: { 'docker-1': { disk: makeSeries(2) } },
      }),
    );
    expect([...map.keys()].sort()).toEqual(['agent-1', 'docker-1']);
    expect(map.get('agent-1')?.memory?.length).toBe(2);
    expect(map.get('docker-1')?.disk?.length).toBe(2);
  });
});

describe('readInfrastructureSummaryCache — series coercion coverage', () => {
  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  it('coerces a non-array cpu field to an empty array (Array.isArray false arm for cpu)', () => {
    // The sibling test sets `cpu: makeSeries(1)` so the cpu cond-expr's false
    // arm stays open; here we store a non-array cpu and assert the coercion.
    storePayload(
      '1h',
      buildPayload({
        charts: {
          'node-1': {
            cpu: 'not-an-array',
            memory: [],
            disk: [],
            diskread: [],
            diskwrite: [],
            netin: [],
            netout: [],
          },
        },
      }),
    );
    const series = readInfrastructureSummaryCache('1h')?.map.get('node-1');
    expect(series?.cpu).toEqual([]);
  });

  it('coerces a non-array netout field to an empty array (Array.isArray false arm for netout)', () => {
    // The sibling test sets `netout: []` which is still an array, leaving the
    // netout cond-expr false arm uncovered.
    storePayload(
      '1h',
      buildPayload({
        charts: {
          'node-1': {
            cpu: [],
            memory: [],
            disk: [],
            diskread: [],
            diskwrite: [],
            netin: [],
            netout: { length: 5 },
          },
        },
      }),
    );
    const series = readInfrastructureSummaryCache('1h')?.map.get('node-1');
    expect(series?.netout).toEqual([]);
  });

  it('returns null oldestDataTimestamp for a non-finite numeric value (Infinity)', () => {
    // typeof Infinity === 'number' is true, so the only way to reach the null
    // fallback past the typeof guard is a number that fails Number.isFinite.
    storePayload(
      '1h',
      buildPayload({
        oldestDataTimestamp: Infinity as unknown as number,
        charts: undefined,
      }),
    );
    expect(readInfrastructureSummaryCache('1h')?.oldestDataTimestamp).toBeNull();
  });

  it('returns null oldestDataTimestamp for NaN', () => {
    storePayload(
      '1h',
      buildPayload({
        oldestDataTimestamp: NaN,
        charts: undefined,
      }),
    );
    expect(readInfrastructureSummaryCache('1h')?.oldestDataTimestamp).toBeNull();
  });

  it('falls back to an empty entries list when charts is a non-object truthy value', () => {
    // `parsed.charts && typeof parsed.charts === 'object'`: a truthy non-object
    // (string) takes the && left-operand true / right-operand false path,
    // producing [] rather than Object.entries throwing.
    storePayload(
      '1h',
      buildPayload({ charts: 'not-an-object' as unknown as Record<string, never> }),
    );
    const cached = readInfrastructureSummaryCache('1h');
    expect(cached).not.toBeNull();
    expect(cached?.map.size).toBe(0);
  });

  it('treats a null charts field as the emptyentries branch', () => {
    // null && ... -> false -> [] arm. Distinct from undefined (also falsy) and
    // from the non-object truthy case above.
    storePayload(
      '1h',
      buildPayload({ charts: null as unknown as undefined }),
    );
    const cached = readInfrastructureSummaryCache('1h');
    expect(cached).not.toBeNull();
    expect(cached?.map.size).toBe(0);
  });
});

describe('readInfrastructureSummaryCache — hit / miss / expiry / empty boundaries', () => {
  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  it('returns a cache hit with the exact cachedAt timestamp on the happy path', () => {
    const cachedAt = now() - 1_000;
    storePayload(
      '1h',
      buildPayload({
        cachedAt,
        oldestDataTimestamp: now() - 5_000,
        charts: {
          'node-1': { cpu: makeSeries(2), memory: [], disk: [], diskread: [], diskwrite: [], netin: [], netout: [] },
        },
      }),
    );
    const hit = readInfrastructureSummaryCache('1h');
    expect(hit).not.toBeNull();
    expect(hit?.cachedAt).toBe(cachedAt);
    expect(hit?.oldestDataTimestamp).toBe(now() - 5_000);
    expect(hit?.map.get('node-1')?.cpu?.length).toBe(2);
  });

  it('returns a cache miss for a never-written key', () => {
    expect(readInfrastructureSummaryCache('1h')).toBeNull();
  });

  it('returns a cache hit when age equals maxAgeMs exactly (strict > boundary)', () => {
    // `Date.now() - parsed.cachedAt > maxAgeMs` uses strict >, so an entry that
    // is exactly maxAgeMs old must still be a hit (boundary is inclusive).
    const maxAge = 60_000;
    const cachedAt = now() - maxAge;
    storePayload(
      '1h',
      buildPayload({ cachedAt, charts: undefined }),
    );
    expect(readInfrastructureSummaryCache('1h', maxAge)).not.toBeNull();
  });

  it('returns an empty map (cache hit, but empty) when charts is absent', () => {
    storePayload('1h', buildPayload({ charts: undefined }));
    const hit = readInfrastructureSummaryCache('1h');
    expect(hit).not.toBeNull();
    expect(hit?.map.size).toBe(0);
    expect(hit?.oldestDataTimestamp).toBeNull();
  });

  it('returns early with null when window is undefined', () => {
    // Covers the `if (typeof window === 'undefined') return null;` true arm in
    // readInfrastructureSummaryCache.
    storePayload('1h', buildPayload({ charts: undefined }));
    const hit = withGlobalUndefined('window', () => readInfrastructureSummaryCache('1h'));
    expect(hit).toBeNull();
  });
});

describe('persistInfrastructureSummaryCache — write-boundary coverage', () => {
  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  it('returns early without writing when window is undefined', () => {
    // Covers the `if (typeof window === 'undefined') return;` true arm.
    localStorage.setItem('sentinel', 'keep-me');
    withGlobalUndefined('window', () => {
      persistInfrastructureSummaryCache(
        '1h',
        new Map([['node-1', { cpu: makeSeries(2) }]]),
        now() - 60_000,
      );
    });
    // No infra cache key was created.
    const keys: string[] = [];
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      if (k) keys.push(k);
    }
    expect(keys.some((k) => k.startsWith('pulse.infrastructureSummaryCharts.'))).toBe(false);
    expect(localStorage.getItem('sentinel')).toBe('keep-me');
  });

  it('uses an explicit orgScope argument rather than falling back to getOrgID()', () => {
    // Covers the left operand of `orgScope ?? getOrgID()` in the persist path:
    // when orgScope is supplied, getOrgID() must NOT be consulted, so the entry
    // lands under the explicit scope even though the ambient org ID differs.
    setOrgID('ambient-org');
    persistInfrastructureSummaryCache(
      '1h',
      new Map([['node-x', { cpu: makeSeries(1) }]]),
      null,
      'explicit-scope',
    );
    expect(localStorage.getItem(cacheKeyForRange('1h', 'explicit-scope'))).not.toBeNull();
    // Nothing landed under the ambient org scope.
    expect(localStorage.getItem(cacheKeyForRange('1h', 'ambient-org'))).toBeNull();
  });

  it('triggers the data.cpu ?? [] fallback in toCachedChartData (cpu field absent)', () => {
    // The sibling test always supplies cpu, so the `?? []` right operand for
    // cpu stays uncovered. Persist a ChartData without cpu and verify the
    // cached entry round-trips with cpu === [].
    persistInfrastructureSummaryCache(
      '1h',
      new Map([['node-1', { memory: makeSeries(2) }]]),
      null,
    );
    const series = readInfrastructureSummaryCache('1h')?.map.get('node-1');
    expect(series?.cpu).toEqual([]);
    expect(series?.memory?.length).toBe(2);
  });
});

describe('countChartMapPoints — uncovered ?? 0 fallbacks via fetch', () => {
  beforeEach(() => {
    mockGetCharts.mockReset();
    __resetInfrastructureSummaryFetchesForTests();
    setOrgID('default');
    localStorage.clear();
  });

  it('counts zero points for an entry whose cpu field is absent (data.cpu?.length ?? 0)', () => {
    // The perf-log data object is built eagerly on every successful fetch, so
    // countChartMapPoints runs even with perf disabled. Supplying a node with
    // no cpu takes the `?? 0` right operand for the cpu line.
    mockGetCharts.mockResolvedValueOnce(
      makeInfraResponse({
        nodeData: { 'node-1': { memory: makeSeries(3) } },
      }),
    );
    // Assertion is on the observable round-trip: the fetch resolved and the
    // entry round-tripped through persist/read without dropping memory.
    return fetchInfrastructureSummaryAndCache('1h').then((result) => {
      expect(result.map.get('node-1')?.memory?.length).toBe(3);
      expect(result.map.get('node-1')?.cpu).toBeUndefined();
    });
  });
});

describe('fetchInfrastructureSummaryAndCache — error and caller edge arms', () => {
  beforeEach(() => {
    mockGetCharts.mockReset();
    __resetInfrastructureSummaryFetchesForTests();
    setOrgID('default');
    localStorage.clear();
  });

  it('rethrows a non-Error rejection as-is after the String(error) fallback arm', () => {
    // `error instanceof Error ? error.message : String(error)`: a string throw
    // takes the false arm and String('boom') === 'boom' is fed to the perf log.
    mockGetCharts.mockRejectedValueOnce('boom');
    return expect(fetchInfrastructureSummaryAndCache('1h')).rejects.toBe('boom');
  });

  it('rethrows a numeric rejection (String(42) fallback shape)', () => {
    mockGetCharts.mockRejectedValueOnce(42);
    return expect(fetchInfrastructureSummaryAndCache('1h')).rejects.toBe(42);
  });

  it("falls back to 'unknown' when options.caller is an empty string (|| right arm)", () => {
    // `options?.caller || 'unknown'`: empty string is falsy, so the right arm
    // is taken. The fetch still resolves and persists a cache entry, which is
    // the observable proof the call completed normally.
    mockGetCharts.mockResolvedValueOnce(makeInfraResponse());
    return fetchInfrastructureSummaryAndCache('1h', { caller: '' }).then(() => {
      expect(readInfrastructureSummaryCache('1h')).not.toBeNull();
      // Caller normalisation must not bleed into the persisted metrics list.
      expect(mockGetCharts).toHaveBeenNthCalledWith(1, '1h', undefined, {
        metrics: ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'],
      });
    });
  });

  it('passes a null metrics option through normalize -> default metrics list', () => {
    // `normalizeInfrastructureSummaryMetrics(null)`: Array.isArray(null) is
    // false, so the !Array.isArray arm returns the default list rather than
    // constructing from the (null) input.
    mockGetCharts.mockResolvedValueOnce(makeInfraResponse());
    return fetchInfrastructureSummaryAndCache('1h', { metrics: null }).then(() => {
      expect(mockGetCharts).toHaveBeenNthCalledWith(1, '1h', undefined, {
        metrics: ['cpu', 'memory', 'disk', 'diskread', 'diskwrite', 'netin', 'netout'],
      });
    });
  });

  it('deduplicates a custom metric list (separate inFlightKey) from the default list', () => {
    // Two different metric lists produce two different inFlightKeys, so they
    // issue two distinct requests. This pins the inFlightKeyFor metrics
    // participation: changing the metrics list must NOT be deduped against the
    // default list.
    mockGetCharts.mockResolvedValueOnce(makeInfraResponse());
    mockGetCharts.mockResolvedValueOnce(makeInfraResponse());
    return Promise.all([
      fetchInfrastructureSummaryAndCache('1h', { metrics: ['cpu'] }),
      fetchInfrastructureSummaryAndCache('1h'),
    ]).then(() => {
      expect(mockGetCharts).toHaveBeenCalledTimes(2);
    });
  });
});

describe('infraSummaryPerfNow — Date.now() fallback arm', () => {
  beforeEach(() => {
    mockGetCharts.mockReset();
    __resetInfrastructureSummaryFetchesForTests();
    setOrgID('default');
    localStorage.clear();
  });

  it('still records a finite ms duration when performance is unavailable', async () => {
    // `typeof performance !== 'undefined' && typeof performance.now === 'function'`:
    // hiding performance takes the && false arm, so infraSummaryPerfNow falls
    // back to Date.now(). The fetch path calls perfNow at start and end, so a
    // successful round-trip with performance masked proves the fallback works.
    mockGetCharts.mockResolvedValueOnce(
      makeInfraResponse({ nodeData: { 'n': { cpu: makeSeries(1) } } }),
    );
    const result = await withGlobalUndefined('performance', () =>
      fetchInfrastructureSummaryAndCache('1h'),
    );
    expect(result.map.get('n')?.cpu?.length).toBe(1);
  });
});

describe('org_switched cache invalidation handler — uncovered getter + branches', () => {
  // The org_switched handler registered at module load (and its inner forEach
  // callback) are never invoked by the existing tests. Emitting the event here
  // exercises the whole invalidation closure: the `typeof window` guard, the
  // key/prefix filter, the keysToRemove forEach, and the catch-all.
  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('removes every prefixed cache entry on org_switched and preserves unrelated keys', () => {
    const prefixedA = `${'pulse.infrastructureSummaryCharts.'}default::1h::${DEFAULT_METRICS_KEY}`;
    const prefixedB = `${'pulse.infrastructureSummaryCharts.'}default::24h::cpu`;
    const unrelated = 'pulse.somethingElse.key';
    localStorage.setItem(prefixedA, 'a');
    localStorage.setItem(prefixedB, 'b');
    localStorage.setItem(unrelated, 'c');

    eventBus.emit('org_switched', 'new-org');

    expect(localStorage.getItem(prefixedA)).toBeNull();
    expect(localStorage.getItem(prefixedB)).toBeNull();
    expect(localStorage.getItem(unrelated)).toBe('c');
  });

  it('skips keys for which localStorage.key returns null (key && ... false arm)', () => {
    // The && left operand is reachable only if a stored key is null; that never
    // happens with the in-memory storage used in tests, so mock key() to force
    // a null return and confirm the handler completes without pushing anything.
    const prefixed = `${'pulse.infrastructureSummaryCharts.'}default::1h::${DEFAULT_METRICS_KEY}`;
    localStorage.setItem(prefixed, 'a');
    // Stub Storage.prototype.key to return null for the first slot, which makes
    // the `key && key.startsWith(...)` short-circuit on the && left operand.
    const keySpy = vi.spyOn(Storage.prototype, 'key').mockReturnValue(null);
    try {
      expect(() => eventBus.emit('org_switched', 'newer-org')).not.toThrow();
    } finally {
      keySpy.mockRestore();
    }
    // Because key() returned null, the real prefixed entry was never matched
    // and is still present.
    expect(localStorage.getItem(prefixed)).toBe('a');
  });

  it('no-ops when there are no prefixed entries to remove (empty keysToRemove)', () => {
    // Covers the forEach callback's "zero iterations" path: keysToRemove is
    // empty because no key starts with the prefix.
    const unrelated = 'completely.unrelated';
    localStorage.setItem(unrelated, 'keep');
    expect(() => eventBus.emit('org_switched', 'empty-org')).not.toThrow();
    expect(localStorage.getItem(unrelated)).toBe('keep');
  });

  it('does not throw when the localStorage iteration itself throws (catch arm)', () => {
    // Covers the try/catch around the invalidation loop: if localStorage.length
    // throws, the handler must swallow it and return silently.
    const lengthSpy = vi
      .spyOn(Storage.prototype, 'length', 'get')
      .mockImplementation(() => {
        throw new Error('storage poisoned');
      });
    try {
      expect(() => eventBus.emit('org_switched', 'broken-org')).not.toThrow();
    } finally {
      lengthSpy.mockRestore();
    }
  });

  it('returns early without iterating when window is undefined (typeof window guard true arm)', () => {
    // Covers the `if (typeof window === 'undefined') return;` true arm of the
    // org_switched handler. A prefixed entry must survive because the handler
    // bails out before touching localStorage.
    const prefixed = `${'pulse.infrastructureSummaryCharts.'}default::1h::${DEFAULT_METRICS_KEY}`;
    localStorage.setItem(prefixed, 'survives');
    withGlobalUndefined('window', () => {
      expect(() => eventBus.emit('org_switched', 'no-window-org')).not.toThrow();
    });
    expect(localStorage.getItem(prefixed)).toBe('survives');
  });
});

describe('infraSummaryPerfLog — perf-enabled branches via module reload', () => {
  // `infraSummaryPerfEnabled` is captured once at module load from
  // `import.meta.env.DEV && import.meta.env.VITE_INFRA_SUMMARY_PERF === '1'`.
  // Reaching the perf-log body therefore requires stubbing the env var AND
  // re-evaluating the module (vi.resetModules + dynamic import) so the constant
  // is re-read with the stub in place. The static top-level imports that every
  // other describe block uses keep pointing at the original (perf-disabled)
  // module instance, so this isolation does not leak.

  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  afterEach(() => {
    vi.resetModules();
    vi.unstubAllEnvs();
  });

  it('emits [InfraSummaryPerf] console.debug lines when VITE_INFRA_SUMMARY_PERF=1', async () => {
    vi.stubEnv('VITE_INFRA_SUMMARY_PERF', '1');
    vi.resetModules();
    const debugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});
    try {
      const mod = await import('@/utils/infrastructureSummaryCache');
      mod.__resetInfrastructureSummaryFetchesForTests();
      mockGetCharts.mockReset();
      mockGetCharts.mockResolvedValueOnce(
        makeInfraResponse({ nodeData: { 'n': { cpu: makeSeries(1) } } }),
      );
      await mod.fetchInfrastructureSummaryAndCache('1h', { caller: 'perf-enabled' });

      const prefixLines = debugSpy.mock.calls
        .map((call) => (typeof call[0] === 'string' ? call[0] : ''))
        .filter((msg) => msg.startsWith('[InfraSummaryPerf]'));
      // fetch start, fetch done (data present -> truthy arm of `if (data)`).
      expect(prefixLines.some((m) => m.endsWith('fetch start'))).toBe(true);
      expect(prefixLines.some((m) => m.endsWith('fetch done'))).toBe(true);
      // The data object was forwarded as the second console.debug argument.
      const doneCall = debugSpy.mock.calls.find(
        ([msg]) => typeof msg === 'string' && msg.endsWith('fetch done'),
      );
      expect(doneCall?.[1]).toMatchObject({ caller: 'perf-enabled', range: '1h' });
    } finally {
      debugSpy.mockRestore();
    }
  });

  it('logs the fetch-error path with a stringified error when perf is enabled', async () => {
    // Covers the perf-log call inside .catch (still gated on perfEnabled) with
    // a non-Error rejection so `error instanceof Error ? ... : String(error)`
    // takes its String(error) arm under perf-enabled conditions.
    vi.stubEnv('VITE_INFRA_SUMMARY_PERF', '1');
    vi.resetModules();
    const debugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});
    try {
      const mod = await import('@/utils/infrastructureSummaryCache');
      mod.__resetInfrastructureSummaryFetchesForTests();
      mockGetCharts.mockReset();
      mockGetCharts.mockRejectedValueOnce('perf-boom');
      await expect(mod.fetchInfrastructureSummaryAndCache('1h')).rejects.toBe('perf-boom');

      const errorLine = debugSpy.mock.calls
        .map((call) => (typeof call[0] === 'string' ? call[0] : ''))
        .find((msg) => msg.endsWith('fetch error'));
      expect(errorLine).toBeDefined();
      const dataArg = debugSpy.mock.calls.find(
        ([msg]) => typeof msg === 'string' && msg.endsWith('fetch error'),
      )?.[1];
      expect(dataArg).toMatchObject({ error: 'perf-boom' });
    } finally {
      debugSpy.mockRestore();
    }
  });
});

describe('readInfrastructureSummaryCache — read failure path', () => {
  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  it('swallows a localStorage.getItem failure and returns null (catch arm)', () => {
    // Covers the outer try/catch in readInfrastructureSummaryCache: a throw out
    // of getItem must be swallowed and surface as a null result.
    const getItemSpy = vi
      .spyOn(Storage.prototype, 'getItem')
      .mockImplementation(() => {
        throw new Error('getItem exploded');
      });
    try {
      expect(readInfrastructureSummaryCache('1h')).toBeNull();
    } finally {
      getItemSpy.mockRestore();
    }
  });
});

describe('persistInfrastructureSummaryCache — JSON.stringify failure path', () => {
  beforeEach(() => {
    setOrgID('default');
    localStorage.clear();
  });

  it('swallows a JSON.stringify cycle error without throwing', () => {
    // Build a value that JSON.stringify cannot serialize (a self-referencing
    // object smuggled in as a ChartData field). The try/catch around the
    // persist body must swallow the TypeError.
    const cyclic = { cpu: [] as MetricPoint[] } as unknown as ChartData;
    (cyclic as { self: unknown }).self = cyclic;
    expect(() =>
      persistInfrastructureSummaryCache(
        '1h',
        new Map([['node-1', cyclic]]),
        null,
      ),
    ).not.toThrow();
  });
});
