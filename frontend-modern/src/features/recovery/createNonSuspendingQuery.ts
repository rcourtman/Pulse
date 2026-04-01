import { type Accessor, createEffect, createSignal, onCleanup } from 'solid-js';

interface CreateNonSuspendingQueryOptions<T, K> {
  source: Accessor<K | null>;
  fetcher: (key: K) => Promise<T>;
  initialValue: T;
  pollMs?: number;
}

interface QueryRunOptions {
  background?: boolean;
}

/**
 * Keep query-backed surfaces out of the app-level Suspense boundary by
 * retaining the last fulfilled value while the next request is in flight.
 */
export function createNonSuspendingQuery<T, K>(options: CreateNonSuspendingQueryOptions<T, K>) {
  const [value, setValue] = createSignal<T>(options.initialValue);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal<unknown>(null);

  let latestRequestId = 0;

  const reset = () => {
    latestRequestId += 1;
    setValue(() => options.initialValue);
    setLoading(false);
    setError(null);
    return options.initialValue;
  };

  const run = async (key: K, runOptions: QueryRunOptions = {}): Promise<T> => {
    const requestId = ++latestRequestId;
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
      if (requestId === latestRequestId && !runOptions.background) {
        setLoading(false);
      }
    }
  };

  createEffect(() => {
    const key = options.source();
    if (key === null) {
      reset();
      return;
    }
    void run(key);
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
    value,
  };
}
