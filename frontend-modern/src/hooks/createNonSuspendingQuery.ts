import { type Accessor, createEffect, createSignal, onCleanup } from 'solid-js';
import { eventBus } from '@/stores/events';

interface CreateNonSuspendingQueryOptions<T, K> {
  source: Accessor<K | null>;
  fetcher: (key: K) => Promise<T>;
  initialValue: T;
  cacheKey?: (key: K) => string | null;
  pollMs?: number;
}

interface QueryRunOptions {
  background?: boolean;
}

/**
 * Keep query-backed surfaces out of the app-level Suspense boundary by
 * retaining the last fulfilled value while the next request is in flight.
 *
 * This cache is shared across route/drawer remounts, so it needs an explicit
 * ownership boundary. In particular, history responses can be large and their
 * keys contain resource IDs and time ranges that continually change during a
 * long-lived browser session.
 */
const RETAINED_QUERY_CACHE_MAX_ENTRIES = 64;
const RETAINED_QUERY_CACHE_MAX_AGE_MS = 5 * 60_000;

type RetainedQueryCacheEntry = {
  cachedAt: number;
  error: unknown;
  resolvedOnce: boolean;
  value: unknown;
};

const retainedQueryCache = new Map<string, RetainedQueryCacheEntry>();
let retainedQueryCacheGeneration = 0;
let retainedQueryCacheExpiryTimer: ReturnType<typeof setTimeout> | null = null;

const clearRetainedQueryCacheExpiryTimer = () => {
  if (retainedQueryCacheExpiryTimer !== null) {
    clearTimeout(retainedQueryCacheExpiryTimer);
    retainedQueryCacheExpiryTimer = null;
  }
};

const pruneExpiredRetainedQueryEntries = (now = Date.now()) => {
  for (const [key, entry] of retainedQueryCache) {
    if (now - entry.cachedAt >= RETAINED_QUERY_CACHE_MAX_AGE_MS) {
      retainedQueryCache.delete(key);
    }
  }
};

const scheduleRetainedQueryCacheExpiry = () => {
  clearRetainedQueryCacheExpiryTimer();
  if (retainedQueryCache.size === 0) {
    return;
  }

  let nextExpiryAt = Number.POSITIVE_INFINITY;
  for (const entry of retainedQueryCache.values()) {
    nextExpiryAt = Math.min(nextExpiryAt, entry.cachedAt + RETAINED_QUERY_CACHE_MAX_AGE_MS);
  }

  retainedQueryCacheExpiryTimer = setTimeout(
    () => {
      retainedQueryCacheExpiryTimer = null;
      pruneExpiredRetainedQueryEntries();
      scheduleRetainedQueryCacheExpiry();
    },
    Math.max(0, nextExpiryAt - Date.now()),
  );
};

const clearRetainedQueryCache = () => {
  retainedQueryCacheGeneration += 1;
  retainedQueryCache.clear();
  clearRetainedQueryCacheExpiryTimer();
};

const readRetainedQueryCacheEntry = (key: string): RetainedQueryCacheEntry | null => {
  const cached = retainedQueryCache.get(key);
  if (!cached) {
    return null;
  }
  if (Date.now() - cached.cachedAt >= RETAINED_QUERY_CACHE_MAX_AGE_MS) {
    retainedQueryCache.delete(key);
    scheduleRetainedQueryCacheExpiry();
    return null;
  }

  // Map insertion order is the LRU order. A cache hit keeps a remounted
  // surface's value ahead of older, inactive resource/range combinations.
  retainedQueryCache.delete(key);
  retainedQueryCache.set(key, cached);
  return cached;
};

const writeRetainedQueryCacheEntry = (
  key: string,
  value: Omit<RetainedQueryCacheEntry, 'cachedAt'>,
) => {
  pruneExpiredRetainedQueryEntries();
  retainedQueryCache.delete(key);
  retainedQueryCache.set(key, {
    ...value,
    cachedAt: Date.now(),
  });

  while (retainedQueryCache.size > RETAINED_QUERY_CACHE_MAX_ENTRIES) {
    const oldestKey = retainedQueryCache.keys().next().value;
    if (oldestKey === undefined) {
      break;
    }
    retainedQueryCache.delete(oldestKey);
  }
  scheduleRetainedQueryCacheExpiry();
};

export function resetCreateNonSuspendingQueryCacheForTest() {
  clearRetainedQueryCache();
}

export function getCreateNonSuspendingQueryCacheDiagnosticsForTest() {
  pruneExpiredRetainedQueryEntries();
  return {
    keys: Array.from(retainedQueryCache.keys()),
    maxAgeMs: RETAINED_QUERY_CACHE_MAX_AGE_MS,
    maxEntries: RETAINED_QUERY_CACHE_MAX_ENTRIES,
    size: retainedQueryCache.size,
  };
}

const unsubscribeRetainedQueryOrgSwitch = eventBus.on('org_switched', () => {
  clearRetainedQueryCache();
});

if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    unsubscribeRetainedQueryOrgSwitch();
    clearRetainedQueryCache();
  });
}

export function createNonSuspendingQuery<T, K>(options: CreateNonSuspendingQueryOptions<T, K>) {
  const getRetainedCacheKey = (key: K | null): string | null => {
    if (key === null || !options.cacheKey) {
      return null;
    }
    return options.cacheKey(key);
  };

  const readRetainedValue = (key: K | null) => {
    const cacheKey = getRetainedCacheKey(key);
    if (!cacheKey) {
      return null;
    }
    const cached = readRetainedQueryCacheEntry(cacheKey);
    if (!cached) {
      return null;
    }
    return cached as { error: unknown; resolvedOnce: boolean; value: T };
  };

  const applyRetainedValue = (cached: { error: unknown; resolvedOnce: boolean; value: T }) => {
    setValue(() => cached.value);
    setError(cached.error);
    setResolvedOnce(cached.resolvedOnce);
  };

  const initialCached = readRetainedValue(options.source());

  const [value, setValue] = createSignal<T>(initialCached?.value ?? options.initialValue);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal<unknown>(initialCached?.error ?? null);
  const [resolvedOnce, setResolvedOnce] = createSignal(initialCached?.resolvedOnce ?? false);

  let latestRequestId = 0;

  const reset = () => {
    latestRequestId += 1;
    setValue(() => options.initialValue);
    setLoading(false);
    setError(null);
    setResolvedOnce(false);
    return options.initialValue;
  };

  const unsubscribeLiveQueryOrgSwitch = eventBus.on('org_switched', () => {
    reset();
  });
  onCleanup(unsubscribeLiveQueryOrgSwitch);

  const run = async (key: K, runOptions: QueryRunOptions = {}): Promise<T> => {
    const requestId = ++latestRequestId;
    const retainedCacheKey = getRetainedCacheKey(key);
    const cacheGeneration = retainedQueryCacheGeneration;
    if (!runOptions.background) {
      setLoading(true);
    }

    try {
      const nextValue = await options.fetcher(key);
      if (requestId !== latestRequestId) {
        return value();
      }
      setValue(() => nextValue);
      setError(null);
      return nextValue;
    } catch (nextError) {
      if (requestId === latestRequestId) {
        setError(nextError);
      }
      return value();
    } finally {
      if (requestId === latestRequestId) {
        setResolvedOnce(true);
        if (retainedCacheKey && cacheGeneration === retainedQueryCacheGeneration) {
          writeRetainedQueryCacheEntry(retainedCacheKey, {
            error: error(),
            resolvedOnce: true,
            value: value(),
          });
        }
        if (!runOptions.background) {
          setLoading(false);
        }
      }
    }
  };

  createEffect(() => {
    const key = options.source();
    if (key === null) {
      reset();
      return;
    }
    const cached = readRetainedValue(key);
    if (cached) {
      applyRetainedValue(cached);
    }
    void run(key, { background: Boolean(cached) });
  });

  const refetch = async (runOptions: QueryRunOptions = {}): Promise<T> => {
    const key = options.source();
    if (key === null) {
      return reset();
    }
    return run(key, runOptions);
  };

  if (typeof options.pollMs === 'number' && options.pollMs > 0) {
    const interval = setInterval(() => {
      void refetch({ background: true });
    }, options.pollMs);
    onCleanup(() => clearInterval(interval));
  }

  return {
    error,
    loading,
    refetch,
    resolvedOnce,
    value,
  };
}
