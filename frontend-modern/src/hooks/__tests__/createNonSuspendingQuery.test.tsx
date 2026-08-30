import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import {
  createNonSuspendingQuery,
  getCreateNonSuspendingQueryCacheDiagnosticsForTest,
  resetCreateNonSuspendingQueryCacheForTest,
} from '@/hooks/createNonSuspendingQuery';
import { eventBus } from '@/stores/events';

afterEach(() => {
  resetCreateNonSuspendingQueryCacheForTest();
  cleanup();
  vi.useRealTimers();
});

function QueryProbe(props: {
  cacheNamespace: string;
  fetcher: (key: string, signal?: AbortSignal) => Promise<string>;
  queryKey?: () => string;
  retainPreviousValueOnSourceChange?: boolean;
}) {
  const state = createNonSuspendingQuery<string, string>({
    source: () => props.queryKey?.() ?? 'stable-key',
    cacheKey: (key) => `${props.cacheNamespace}:${key}`,
    fetcher: props.fetcher,
    initialValue: 'initial',
    retainPreviousValueOnSourceChange: props.retainPreviousValueOnSourceChange,
  });

  return (
    <div data-testid="query-probe">{`${state.value()}|resolved:${String(state.resolvedOnce())}|loading:${String(state.loading())}`}</div>
  );
}

describe('createNonSuspendingQuery', () => {
  it('reuses the last fulfilled value when the same query remounts', async () => {
    const cacheNamespace = `query-cache-${Date.now()}`;
    const firstFetcher = vi.fn(async () => 'loaded');
    const secondFetcher = vi.fn(() => new Promise<string>(() => {}));

    const firstRender = render(() => (
      <QueryProbe cacheNamespace={cacheNamespace} fetcher={firstFetcher} />
    ));

    await waitFor(() => {
      expect(screen.getByTestId('query-probe').textContent).toContain('loaded');
      expect(screen.getByTestId('query-probe').textContent).toContain('resolved:true');
    });

    firstRender.unmount();

    render(() => <QueryProbe cacheNamespace={cacheNamespace} fetcher={secondFetcher} />);

    await waitFor(() => {
      expect(secondFetcher).toHaveBeenCalledWith('stable-key', expect.any(AbortSignal));
    });

    expect(screen.getByTestId('query-probe').textContent).toContain('loaded');
    expect(screen.getByTestId('query-probe').textContent).toContain('resolved:true');
    expect(screen.getByTestId('query-probe').textContent).toContain('loading:false');
    expect(screen.getByTestId('query-probe').textContent).not.toContain('initial');
  });

  it('retains the previous source value by default while the replacement loads', async () => {
    const [queryKey, setQueryKey] = createSignal('1h');
    const fetcher = vi.fn((key: string) =>
      key === '1h' ? Promise.resolve('loaded:1h') : new Promise<string>(() => {}),
    );

    render(() => (
      <QueryProbe
        cacheNamespace={`retained-source-${Date.now()}`}
        fetcher={fetcher}
        queryKey={queryKey}
      />
    ));

    await waitFor(() => {
      expect(screen.getByTestId('query-probe').textContent).toContain('loaded:1h');
    });

    setQueryKey('24h');
    await waitFor(() => {
      expect(fetcher).toHaveBeenCalledWith('24h', expect.any(AbortSignal));
    });

    expect(screen.getByTestId('query-probe').textContent).toContain('loaded:1h');
    expect(screen.getByTestId('query-probe').textContent).toContain('loading:true');
  });

  it('clears a prior source immediately when retained data would mislabel the active range', async () => {
    const [queryKey, setQueryKey] = createSignal('1h');
    const fetcher = vi.fn((key: string) =>
      key === '1h' ? Promise.resolve('loaded:1h') : new Promise<string>(() => {}),
    );

    render(() => (
      <QueryProbe
        cacheNamespace={`source-honest-${Date.now()}`}
        fetcher={fetcher}
        queryKey={queryKey}
        retainPreviousValueOnSourceChange={false}
      />
    ));

    await waitFor(() => {
      expect(screen.getByTestId('query-probe').textContent).toContain('loaded:1h');
    });

    setQueryKey('24h');
    await waitFor(() => {
      expect(fetcher).toHaveBeenCalledWith('24h', expect.any(AbortSignal));
    });

    expect(screen.getByTestId('query-probe').textContent).toContain('initial');
    expect(screen.getByTestId('query-probe').textContent).toContain('resolved:false');
    expect(screen.getByTestId('query-probe').textContent).toContain('loading:true');
    expect(screen.getByTestId('query-probe').textContent).not.toContain('loaded:1h');
  });

  it('aborts superseded source requests', async () => {
    const [queryKey, setQueryKey] = createSignal('1h');
    const signals: AbortSignal[] = [];
    const fetcher = vi.fn((_key: string, signal?: AbortSignal) => {
      signals.push(signal!);
      return new Promise<string>(() => {});
    });

    render(() => (
      <QueryProbe
        cacheNamespace={`abort-superseded-${Date.now()}`}
        fetcher={fetcher}
        queryKey={queryKey}
      />
    ));

    await waitFor(() => expect(signals).toHaveLength(1));
    expect(signals[0].aborted).toBe(false);

    setQueryKey('7d');
    await waitFor(() => expect(signals).toHaveLength(2));
    expect(signals[0].aborted).toBe(true);
    expect(signals[1].aborted).toBe(false);
  });

  it('evicts least-recently-used resource and range entries at the cache limit', async () => {
    const cacheNamespace = `bounded-query-cache-${Date.now()}`;
    const { maxEntries } = getCreateNonSuspendingQueryCacheDiagnosticsForTest();
    const [queryKey, setQueryKey] = createSignal('resource-0:1h');
    const fetcher = vi.fn(async (key: string) => `loaded:${key}`);

    render(() => (
      <QueryProbe cacheNamespace={cacheNamespace} fetcher={fetcher} queryKey={queryKey} />
    ));

    for (let index = 0; index <= maxEntries; index += 1) {
      const key = `resource-${index}:1h`;
      setQueryKey(key);
      await waitFor(() => {
        expect(screen.getByTestId('query-probe').textContent).toContain(`loaded:${key}`);
      });
    }

    const diagnostics = getCreateNonSuspendingQueryCacheDiagnosticsForTest();
    expect(diagnostics.size).toBe(maxEntries);
    expect(diagnostics.keys).not.toContain(`${cacheNamespace}:resource-0:1h`);
    expect(diagnostics.keys).toContain(`${cacheNamespace}:resource-${maxEntries}:1h`);
  });

  it('drops retained values when the organization changes', async () => {
    const cacheNamespace = `org-query-cache-${Date.now()}`;
    const firstRender = render(() => (
      <QueryProbe cacheNamespace={cacheNamespace} fetcher={async () => 'org-a-value'} />
    ));

    await waitFor(() => {
      expect(screen.getByTestId('query-probe').textContent).toContain('org-a-value');
    });
    firstRender.unmount();
    expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().size).toBe(1);

    eventBus.emit('org_switched', 'org-b');

    render(() => (
      <QueryProbe cacheNamespace={cacheNamespace} fetcher={() => new Promise<string>(() => {})} />
    ));

    expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().size).toBe(0);
    expect(screen.getByTestId('query-probe').textContent).toContain('initial');
    expect(screen.getByTestId('query-probe').textContent).not.toContain('org-a-value');
  });

  it('does not repopulate the cache when an old-org request resolves late', async () => {
    // The org switch itself now issues a fresh request, so keep every
    // resolver and settle the pre-switch one specifically. Resolving the
    // latest would prove nothing about stale responses.
    const resolvers: ((value: string) => void)[] = [];
    render(() => (
      <QueryProbe
        cacheNamespace={`late-query-cache-${Date.now()}`}
        fetcher={() =>
          new Promise<string>((resolve) => {
            resolvers.push(resolve);
          })
        }
      />
    ));

    await waitFor(() => {
      expect(resolvers).toHaveLength(1);
    });
    const resolveOldOrgFetch = resolvers[0];
    eventBus.emit('org_switched', 'org-b');
    resolveOldOrgFetch('late-org-a-value');

    await waitFor(() => {
      expect(screen.getByTestId('query-probe').textContent).toContain('initial');
    });
    expect(screen.getByTestId('query-probe').textContent).not.toContain('late-org-a-value');
    expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().size).toBe(0);
  });

  it('expires inactive retained values after the cache age limit', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-07-23T12:00:00.000Z'));
    const diagnostics = getCreateNonSuspendingQueryCacheDiagnosticsForTest();

    render(() => (
      <QueryProbe cacheNamespace="expiring-query-cache" fetcher={async () => 'short-lived-value'} />
    ));

    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();
    expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().size).toBe(1);

    vi.advanceTimersByTime(diagnostics.maxAgeMs);

    expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().size).toBe(0);
  });

  it('refetches a constant-source query after an org switch', async () => {
    // c77571685 added an org_switched handler that only called reset().
    // reset() writes signals the source effect does not track, so for a
    // consumer with a constant source and no polling (connectionsSnapshot,
    // patrol-status) the panel emptied and stayed empty until remount.
    const fetcher = vi.fn(async (key: string) => `value-for-${key}`);

    render(() => <QueryProbe cacheNamespace={`org-refetch-${Date.now()}`} fetcher={fetcher} />);

    await waitFor(() => {
      expect(screen.getByTestId('query-probe').textContent).toContain('value-for-stable-key');
    });
    expect(fetcher).toHaveBeenCalledTimes(1);

    eventBus.emit('org_switched', 'org-b');

    await waitFor(() => {
      expect(fetcher).toHaveBeenCalledTimes(2);
    });
    await waitFor(() => {
      expect(screen.getByTestId('query-probe').textContent).toContain('value-for-stable-key');
      expect(screen.getByTestId('query-probe').textContent).toContain('resolved:true');
    });
  });
});
