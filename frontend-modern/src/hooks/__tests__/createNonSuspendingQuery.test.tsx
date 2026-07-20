import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import {
  createNonSuspendingQuery,
  resetCreateNonSuspendingQueryCacheForTest,
} from '@/hooks/createNonSuspendingQuery';

afterEach(() => {
  resetCreateNonSuspendingQueryCacheForTest();
  cleanup();
});

function QueryProbe(props: { cacheNamespace: string; fetcher: (key: string) => Promise<string> }) {
  const state = createNonSuspendingQuery<string, string>({
    source: () => 'stable-key',
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
});
