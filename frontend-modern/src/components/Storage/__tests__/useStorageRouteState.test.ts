import { batch, createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { buildStoragePath } from '@/routing/resourceLinks';
import { useStorageRouteState } from '@/components/Storage/useStorageRouteState';

describe('useStorageRouteState', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const setup = (initialSearch = '') => {
    const [pathname] = createSignal('/storage');
    const [search, setSearch] = createSignal(initialSearch);
    const [tab, setTab] = createSignal('pools');
    const [source, setSource] = createSignal('all');
    const [status, setStatus] = createSignal('all');
    const [node, setNode] = createSignal('all');
    const [query, setQuery] = createSignal('');
    const [group, setGroup] = createSignal('none');
    const [sort, setSort] = createSignal('priority');
    const [order, setOrder] = createSignal('desc');
    const navigate = vi.fn((nextPath: string) => {
      const nextSearch = nextPath.includes('?') ? nextPath.slice(nextPath.indexOf('?')) : '';
      setSearch(nextSearch);
    });

    let dispose = () => {};
    createRoot((d) => {
      dispose = d;
      useStorageRouteState({
        location: {
          get pathname() {
            return pathname();
          },
          get search() {
            return search();
          },
        },
        navigate,
        buildPath: buildStoragePath,
        useCurrentPathForNavigation: true,
        fields: {
          tab: {
            get: tab,
            set: setTab,
            read: (parsed) => (parsed.tab === 'disks' ? 'disks' : 'pools'),
            write: (value) => (value !== 'pools' ? value : null),
          },
          source: {
            get: source,
            set: setSource,
            read: (parsed) => parsed.source || 'all',
            write: (value) => (value !== 'all' ? value : null),
          },
          status: {
            get: status,
            set: setStatus,
            read: (parsed) => parsed.status || 'all',
            write: (value) => (value !== 'all' ? value : null),
          },
          node: {
            get: node,
            set: setNode,
            read: (parsed) => parsed.node || 'all',
            write: (value) => (value !== 'all' ? value : null),
          },
          query: {
            get: query,
            set: setQuery,
            read: (parsed) => parsed.query || '',
            write: (value) => value.trim() || null,
          },
          group: {
            get: group,
            set: setGroup,
            read: (parsed) => parsed.group || 'none',
            write: (value) => (value !== 'none' ? value : null),
          },
          sort: {
            get: sort,
            set: setSort,
            read: (parsed) => parsed.sort || 'priority',
            write: (value) => (value !== 'priority' ? value : null),
          },
          order: {
            get: order,
            set: setOrder,
            read: (parsed) => parsed.order || 'desc',
            write: (value) => (value !== 'desc' ? value : null),
          },
        },
      });
    });

    return {
      navigate,
      dispose,
      search,
      setSearch,
      setTab,
      setSource,
      setStatus,
      setNode,
      setQuery,
      setGroup,
      setSort,
      setOrder,
    };
  };

  it('does not strip an unmanaged resource deep-link param', () => {
    const ctx = setup('?resource=storage-minipc');

    vi.runAllTimers();

    expect(ctx.navigate).not.toHaveBeenCalled();
    expect(ctx.search()).toBe('?resource=storage-minipc');

    ctx.dispose();
  });

  it('does not navigate for semantically equivalent search params with different ordering', () => {
    const ctx = setup('?source=agent&tab=disks');

    vi.runAllTimers();

    expect(ctx.navigate).not.toHaveBeenCalled();
    expect(ctx.search()).toBe('?source=agent&tab=disks');

    ctx.dispose();
  });

  it('coalesces reactive writes into one replace navigation', () => {
    const ctx = setup();

    batch(() => {
      ctx.setSource('agent');
      ctx.setTab('disks');
      ctx.setSource('proxmox-pbs');
    });

    vi.runAllTimers();

    expect(ctx.navigate).toHaveBeenCalledTimes(1);
    expect(ctx.navigate).toHaveBeenCalledWith('/storage?tab=disks&source=proxmox-pbs', {
      replace: true,
    });

    ctx.dispose();
  });
});
