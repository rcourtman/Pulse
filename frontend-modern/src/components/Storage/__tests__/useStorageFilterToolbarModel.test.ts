import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useStorageFilterToolbarModel } from '@/components/Storage/useStorageFilterToolbarModel';
import type { StorageSourceOption } from '@/utils/storageSources';

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

  it('keeps storage source options reactive for restored route state', () => {
    const [search, setSearch] = createSignal('');
    const [sortKey, setSortKey] = createSignal('priority');
    const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('desc');
    const [sourceOptions, setSourceOptions] = createSignal<StorageSourceOption[]>([
      { key: 'all', label: 'All Sources', tone: 'slate' as const },
    ]);

    const { result } = renderHook(() =>
      useStorageFilterToolbarModel({
        search,
        setSearch,
        sortKey,
        setSortKey,
        sortDirection,
        setSortDirection,
        sourceOptions,
      }),
    );

    expect(result.sourceOptions()).toEqual([{ key: 'all', label: 'All Sources', tone: 'slate' }]);

    setSourceOptions([
      { key: 'all', label: 'All Sources', tone: 'slate' },
      { key: 'proxmox-pve', label: 'PVE', tone: 'orange' },
      { key: 'truenas', label: 'TrueNAS', tone: 'blue' },
    ]);

    expect(result.sourceOptions()).toEqual([
      { key: 'all', label: 'All Sources', tone: 'slate' },
      { key: 'proxmox-pve', label: 'PVE', tone: 'orange' },
      { key: 'truenas', label: 'TrueNAS', tone: 'blue' },
    ]);
  });
});
