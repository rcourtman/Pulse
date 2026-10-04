import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
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

it.each(['success', 'failure'] as const)(
  'settles a foreground loading state when its latest background replacement ends in %s',
  async (outcome) => {
    vi.useFakeTimers();
    let finishManual!: (value: string) => void;
    const manual = new Promise<string>((resolve) => {
      finishManual = resolve;
    });
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce('first')
      .mockReturnValueOnce(manual)
      .mockImplementationOnce(() =>
        outcome === 'success'
          ? Promise.resolve('latest')
          : Promise.reject(new Error('Unavailable')),
      );
    const Probe = () => {
      const query = createNonSuspendingQuery({
        source: () => 'pbs-host',
        fetcher,
        initialValue: '',
        pollMs: 30_000,
      });
      return (
        <>
          <button onClick={() => void query.refetch()}>Refresh</button>
          <output data-testid="query">{`${query.value()}|loading:${query.loading()}`}</output>
        </>
      );
    };
    render(() => <Probe />);
    await vi.advanceTimersByTimeAsync(0);
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    expect(screen.getByTestId('query')).toHaveTextContent('first|loading:true');
    await vi.advanceTimersByTimeAsync(30_000);
    expect(fetcher).toHaveBeenCalledTimes(3);
    expect(fetcher.mock.calls[1][1].aborted).toBe(true);
    const expected = outcome === 'success' ? 'latest|loading:false' : 'first|loading:false';
    expect(screen.getByTestId('query')).toHaveTextContent(expected);
    finishManual('obsolete');
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByTestId('query')).toHaveTextContent(expected);
  },
);

const settle = async () => {
  await Promise.resolve();
  await Promise.resolve();
};
const denied = (status: number) => Object.assign(new Error('private transport detail'), { status });
function mount(key: string, fetcher: () => Promise<string>) {
  let query!: ReturnType<typeof createNonSuspendingQuery<string, string>>;
  const view = render(() => {
    query = createNonSuspendingQuery({
      source: () => key,
      cacheKey: (key) => key,
      fetcher,
      initialValue: '',
    });
    return <output>{query.value()}</output>;
  });
  return { query, ...view };
}

describe('retained query access boundary', () => {
  it.each([401, 403])(
    'withdraws the denied value and all remount entries after %s',
    async (status) => {
      const oldRange = mount('pbs:1h', async () => 'former reading');
      await settle();
      oldRange.unmount();
      let rejectRead = false;
      const active = mount('pbs:24h', async () => {
        if (rejectRead) throw denied(status);
        return 'current reading';
      });
      await settle();
      expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().size).toBe(2);
      rejectRead = true;
      await active.query.refetch({ background: true });
      expect(active.query.value()).toBe('');
      expect(active.query.error()).toMatchObject({ status });
      expect(active.query.loading()).toBe(false);
      expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().size).toBe(0);
      active.unmount();
      const remount = mount('pbs:1h', () => new Promise(() => {}));
      expect(remount.query.value()).toBe('');
      expect(remount.query.resolvedOnce()).toBe(false);
    },
  );

  it('does not repopulate the remount cache from a pre-denial read', async () => {
    let complete!: (value: string) => void;
    const pending = mount(
      'pbs:1h',
      () =>
        new Promise((resolve) => {
          complete = resolve;
        }),
    );
    const active = mount('pbs:24h', async () => {
      throw denied(403);
    });
    await settle();
    expect(active.query.value()).toBe('');
    complete('before-access-change');
    await settle();
    expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().size).toBe(0);
    pending.unmount();
    const remount = mount('pbs:1h', () => new Promise(() => {}));
    expect(remount.query.value()).toBe('');
  });

  it.each([500, 503, 429, undefined])(
    'retains useful readings on transient status %s',
    async (status) => {
      let rejectRead = false;
      const active = mount('pbs:24h', async () => {
        if (rejectRead) throw denied(status as number);
        return 'stored reading';
      });
      await settle();
      rejectRead = true;
      await active.query.refetch();
      expect(active.query.value()).toBe('stored reading');
      expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().size).toBe(1);
    },
  );

  it('restores only freshly authorised data after denial', async () => {
    let result: string | Error = 'before denial';
    const active = mount('pbs:24h', async () => {
      if (result instanceof Error) throw result;
      return result;
    });
    await settle();
    result = denied(403);
    await active.query.refetch();
    expect(active.query.value()).toBe('');
    result = 'newly authorised reading';
    await active.query.refetch();
    expect(active.query.value()).toBe(result);
    expect(active.query.error()).toBeNull();
    expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().size).toBe(1);
  });
  it('ignores a superseded denial rather than clearing the new target and its cache', async () => {
    let rejectOld!: (error: Error) => void;
    let query!: ReturnType<typeof createNonSuspendingQuery<string, string>>;
    const [source, setSource] = createSignal('old');
    render(() => {
      query = createNonSuspendingQuery({
        source,
        cacheKey: (key) => key,
        initialValue: '',
        fetcher: (key) =>
          key === 'old'
            ? new Promise((_, reject) => {
                rejectOld = reject;
              })
            : Promise.resolve('new reading'),
      });
      return <output>{query.value()}</output>;
    });
    setSource('new');
    await settle();
    rejectOld(denied(403));
    await settle();
    expect(query.value()).toBe('new reading');
    expect(query.error()).toBeNull();
    expect(getCreateNonSuspendingQueryCacheDiagnosticsForTest().keys).toEqual(['new']);
  });
});
