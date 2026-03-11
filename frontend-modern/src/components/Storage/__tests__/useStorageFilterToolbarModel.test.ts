import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useStorageFilterToolbarModel } from '@/components/Storage/useStorageFilterToolbarModel';

describe('useStorageFilterToolbarModel', () => {
  it('centralizes storage filter toolbar state and reset behavior', () => {
    const [search, setSearch] = createSignal('tank');
    const [groupBy, setGroupBy] = createSignal<'none' | 'node'>('node');
    const [sortKey, setSortKey] = createSignal('usage');
    const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
    const [statusFilter, setStatusFilter] = createSignal<'all' | 'warning'>('warning');
    const [sourceFilter, setSourceFilter] = createSignal('agent');

    const { result } = renderHook(() =>
      useStorageFilterToolbarModel({
        search,
        setSearch,
        groupBy,
        setGroupBy,
        sortKey,
        setSortKey,
        sortDirection,
        setSortDirection,
        statusFilter,
        setStatusFilter,
        sourceFilter,
        setSourceFilter,
        sortOptions: [{ value: 'usage', label: 'Usage' }],
      }),
    );

    expect(result.activeFilterCount()).toBeGreaterThan(0);
    expect(result.showReset()).toBe(true);
    expect(result.sortOptions()).toEqual([{ value: 'usage', label: 'Usage' }]);
    expect(result.sortDirectionTitle()).toBe('Sort descending');
    expect(result.sortDirectionIconClass()).toBe('rotate-180');

    result.toggleSortDirection();
    expect(sortDirection()).toBe('desc');

    result.resetFilters();

    expect(search()).toBe('');
    expect(groupBy()).toBe('none');
    expect(sortKey()).toBe('priority');
    expect(sortDirection()).toBe('desc');
    expect(statusFilter()).toBe('all');
    expect(sourceFilter()).toBe('all');
  });
});
