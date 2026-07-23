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
  fetcher: (key: string) => Promise<string>;
  queryKey?: () => string;
}) {
  const state = createNonSuspendingQuery<string, string>({
    source: () => props.queryKey?.() ?? 'stable-key',
    cacheKey: (key) => `${props.cacheNamespace}:${key}`,
    fetcher: props.fetcher,
    initialValue: 'initial',
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
      expect(secondFetcher).toHaveBeenCalledWith('stable-key');
    });

    expect(screen.getByTestId('query-probe').textContent).toContain('loaded');
    expect(screen.getByTestId('query-probe').textContent).toContain('resolved:true');
    expect(screen.getByTestId('query-probe').textContent).toContain('loading:false');
    expect(screen.getByTestId('query-probe').textContent).not.toContain('initial');
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
    let resolveFetch: ((value: string) => void) | undefined;
    render(() => (
      <QueryProbe
        cacheNamespace={`late-query-cache-${Date.now()}`}
        fetcher={() =>
          new Promise<string>((resolve) => {
            resolveFetch = resolve;
          })
        }
      />
    ));

    await waitFor(() => {
      expect(resolveFetch).toBeTypeOf('function');
    });
    eventBus.emit('org_switched', 'org-b');
    resolveFetch?.('late-org-a-value');

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
});
