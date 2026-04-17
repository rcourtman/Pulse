import { type Accessor, createEffect, createSignal, onCleanup } from 'solid-js';

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
 */
const retainedQueryCache = new Map<
  string,
  { error: unknown; resolvedOnce: boolean; value: unknown }
>();

export function resetCreateNonSuspendingQueryCacheForTest() {
  retainedQueryCache.clear();
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
    const cached = retainedQueryCache.get(cacheKey);
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

  const run = async (key: K, runOptions: QueryRunOptions = {}): Promise<T> => {
    const requestId = ++latestRequestId;
    const retainedCacheKey = getRetainedCacheKey(key);
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
        if (retainedCacheKey) {
          retainedQueryCache.set(retainedCacheKey, {
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
