import { batch, createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useStoragePageFilters } from '@/components/Storage/useStoragePageFilters';

describe('useStoragePageFilters', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const setup = (initialSearch = '') => {
    const [pathname] = createSignal('/storage');
    const [searchValue, setSearchValue] = createSignal(initialSearch);
    const navigate = vi.fn((nextPath: string) => {
      const nextSearch = nextPath.includes('?') ? nextPath.slice(nextPath.indexOf('?')) : '';
      setSearchValue(nextSearch);
    });

    let dispose = () => {};
    let filters!: ReturnType<typeof useStoragePageFilters>;
    createRoot((d) => {
      dispose = d;
      filters = useStoragePageFilters({
        location: {
          get pathname() {
            return pathname();
          },
          get search() {
            return searchValue();
          },
        },
        navigate,
      });
    });

    return { filters, navigate, searchValue, dispose };
  };

  it('reads canonical storage query state through the shared page-filter hook', () => {
    const ctx = setup('?tab=disks&source=agent&status=warning&group=status&sort=usage&order=asc&q=tank');

    vi.runAllTimers();

    expect(ctx.filters.view()).toBe('disks');
    expect(ctx.filters.sourceFilter()).toBe('agent');
    expect(ctx.filters.healthFilter()).toBe('warning');
    expect(ctx.filters.groupBy()).toBe('status');
    expect(ctx.filters.sortKey()).toBe('usage');
    expect(ctx.filters.sortDirection()).toBe('asc');
    expect(ctx.filters.search()).toBe('tank');

    ctx.dispose();
  });

  it('coalesces filter changes into a single storage route replace navigation', () => {
    const ctx = setup();

    batch(() => {
      ctx.filters.setView('disks');
      ctx.filters.setSourceFilter('agent');
      ctx.filters.setSearch('tank');
    });

    vi.runAllTimers();

    expect(ctx.navigate).toHaveBeenCalledTimes(1);
    expect(ctx.navigate).toHaveBeenCalledWith('/storage?tab=disks&source=agent&q=tank', {
      replace: true,
    });

    ctx.dispose();
  });
});
