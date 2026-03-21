import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import { useDashboardFilterState } from '@/components/Dashboard/useDashboardFilterState';

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: () => false,
  }),
}));

describe('useDashboardFilterState', () => {
  it('centralizes dashboard filter state and reset behavior', () => {
    const [search, setSearch] = createSignal('query');
    const [viewMode, setViewMode] = createSignal<
      'all' | 'vm' | 'system-container' | 'app-container' | 'pod'
    >('vm');
    const [statusMode, setStatusMode] = createSignal<'all' | 'running' | 'degraded' | 'stopped'>(
      'running',
    );
    const [groupingMode, setGroupingMode] = createSignal<'grouped' | 'flat'>('flat');
    const [sortKey, setSortKey] = createSignal('cpu');
    const [sortDirection, setSortDirection] = createSignal('desc');
    const hostOnChange = vi.fn();
    const namespaceOnChange = vi.fn();
    const runtimeOnChange = vi.fn();

    const { result } = renderHook(() =>
      useDashboardFilterState({
        search,
        setSearch,
        viewMode,
        setViewMode,
        statusMode,
        setStatusMode,
        groupingMode,
        setGroupingMode,
        setSortKey,
        setSortDirection,
        hostFilter: {
          value: 'host-1',
          options: [{ value: 'host-1', label: 'Host 1' }],
          onChange: hostOnChange,
        },
        namespaceFilter: {
          value: 'ns-1',
          options: [{ value: 'ns-1', label: 'NS 1' }],
          onChange: namespaceOnChange,
        },
        containerRuntimeFilter: {
          value: 'docker',
          options: [{ value: 'docker', label: 'Docker' }],
          onChange: runtimeOnChange,
        },
      }),
    );

    expect(result.activeFilterCount()).toBe(5);
    expect(result.showReset()).toBe(true);
    expect(result.showToolbarFilters()).toBe(true);
    expect(result.filtersOpen()).toBe(false);
    expect(result.isMobile()).toBe(false);

    result.setFiltersOpen(true);
    expect(result.filtersOpen()).toBe(true);
    expect(result.showToolbarFilters()).toBe(true);

    result.resetFilters();

    expect(search()).toBe('');
    expect(viewMode()).toBe('all');
    expect(statusMode()).toBe('all');
    expect(groupingMode()).toBe('grouped');
    expect(sortKey()).toBe('name');
    expect(sortDirection()).toBe('asc');
    expect(hostOnChange).toHaveBeenCalledWith('');
    expect(namespaceOnChange).toHaveBeenCalledWith('');
    expect(runtimeOnChange).not.toHaveBeenCalled();
  });
});
