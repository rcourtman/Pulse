import { createRoot, createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ViewMode } from '@/types/workloads';

const mockRouterPathname = '/workloads';
const [mockRouterSearch, setMockRouterSearch] = createSignal('');
const navigateSpy = vi.fn((path: string) => {
  const queryIndex = path.indexOf('?');
  setMockRouterSearch(queryIndex >= 0 ? path.slice(queryIndex) : '');
});

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    get pathname() {
      return mockRouterPathname;
    },
    get search() {
      return mockRouterSearch();
    },
  }),
  useNavigate: () => navigateSpy,
}));

import { useWorkloadsControlsState } from '../useWorkloadsControlsState';

const setWideViewport = () => {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    value: 1600,
  });
};

const setCompactViewport = () => {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    value: 1280,
  });
};

describe('useWorkloadsControlsState', () => {
  beforeEach(() => {
    localStorage.clear();
    setMockRouterSearch('');
    navigateSpy.mockClear();
    window.history.replaceState(null, '', '/workloads');
    setWideViewport();
  });

  it('only offers columns the compact table can present without horizontal scrolling', () => {
    setCompactViewport();
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const state = useWorkloadsControlsState({
          viewMode: () => 'all' as ViewMode,
          showFilters,
          setShowFilters,
        });
        const menu = state.workloadsFilterColumnVisibility();

        expect(state.workloadTableVisibleColumnIds()).not.toContain('netIo');
        expect(menu.availableColumns.map((column) => column.id)).not.toContain('netIo');
        expect(menu.availableColumns.map((column) => column.id)).not.toContain('ip');
        expect(menu.availableColumns.map((column) => column.id)).toContain('backup');

        menu.onColumnToggle('backup');
        expect(state.columnVisibility.hiddenColumns()).toContain('backup');
        expect(state.workloadTableVisibleColumnIds()).not.toContain('backup');
        menu.onColumnToggle('backup');
        expect(state.columnVisibility.hiddenColumns()).not.toContain('backup');
        expect(state.workloadTableVisibleColumnIds()).toContain('backup');
      } finally {
        dispose();
      }
    });
  });

  it('restores selected wide columns when a measured container widens again', () => {
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const [layoutWidth, setLayoutWidth] = createSignal(1500);
        const state = useWorkloadsControlsState({
          viewMode: () => 'all' as ViewMode,
          showFilters,
          setShowFilters,
          layoutWidth,
        });
        expect(state.workloadTableLayoutMode()).toBe('wide');
        expect(state.workloadTableVisibleColumnIds()).toContain('netIo');
        expect(
          state.workloadsFilterColumnVisibility().availableColumns.map((column) => column.id),
        ).toContain('netIo');

        setLayoutWidth(1000);
        expect(state.workloadTableLayoutMode()).toBe('compact');
        expect(state.workloadTableVisibleColumnIds()).not.toContain('netIo');
        expect(
          state.workloadsFilterColumnVisibility().availableColumns.map((column) => column.id),
        ).not.toContain('netIo');

        setLayoutWidth(1500);
        expect(state.workloadTableVisibleColumnIds()).toContain('netIo');
      } finally {
        dispose();
      }
    });
  });

  it('URL-backs free-text search via q so saved views capture -term exclusions', () => {
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const state = useWorkloadsControlsState({
          viewMode: () => 'all' as ViewMode,
          showFilters,
          setShowFilters,
        });

        expect(state.search()).toBe('');

        state.setSearch('-noisy');

        expect(navigateSpy).toHaveBeenCalledWith('/workloads?q=-noisy', { replace: true });
        expect(state.search()).toBe('-noisy');

        state.setSearch('');

        expect(mockRouterSearch()).toBe('');
        expect(state.search()).toBe('');
      } finally {
        dispose();
      }
    });
  });

  it('persists scoped workload status so platform pages restore it when revisited', async () => {
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const state = useWorkloadsControlsState({
          viewMode: () => 'all' as ViewMode,
          showFilters,
          setShowFilters,
          statusModeStorageScope: 'proxmox',
        });

        state.setStatusMode('running');

        expect(mockRouterSearch()).toBe('?status=running');
        expect(localStorage.getItem('workloadsStatusMode:proxmox')).toBe('running');
      } finally {
        dispose();
      }
    });

    setMockRouterSearch('');
    // During a router transition the browser URL can still reflect the route
    // being left. Restores must target the router location that owns the hook.
    window.history.replaceState(null, '', '/stale-workspace-entry');
    navigateSpy.mockClear();

    const disposeRestore = createRoot((dispose) => {
      const [showFilters, setShowFilters] = createSignal(false);
      useWorkloadsControlsState({
        viewMode: () => 'all' as ViewMode,
        showFilters,
        setShowFilters,
        statusModeStorageScope: 'proxmox',
      });
      return dispose;
    });

    await Promise.resolve();

    expect(navigateSpy).toHaveBeenCalledWith('/workloads?status=running', { replace: true });
    expect(mockRouterSearch()).toBe('?status=running');
    disposeRestore();
  });

  it('lets Docker scope use a container-native column profile without hiding disk globally', () => {
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const dockerState = useWorkloadsControlsState({
          viewMode: () => 'app-container' as ViewMode,
          showFilters,
          setShowFilters,
          columnVisibilityStorageScope: 'docker-runtime-containers',
          additionalDefaultHiddenColumnIds: ['disk', 'tags'],
          columnLabelOverrides: { context: 'Host', disk: 'Writable layer' },
        });

        const visibleColumnIds = dockerState.visibleColumns().map((column) => column.id);
        expect(dockerState.columnVisibility.hiddenColumns()).toContain('disk');
        expect(dockerState.columnVisibility.hiddenColumns()).toContain('tags');
        expect(visibleColumnIds).not.toContain('disk');
        expect(visibleColumnIds).not.toContain('tags');
        expect(visibleColumnIds).not.toContain('type');
        expect(visibleColumnIds).not.toContain('info');
        expect(visibleColumnIds).not.toContain('backup');
        expect(visibleColumnIds).toContain('runtime');
        expect(visibleColumnIds).toContain('image');
        expect(visibleColumnIds).toContain('context');
        expect(visibleColumnIds).toContain('update');
        expect(visibleColumnIds).toContain('diskIo');
        expect(visibleColumnIds).toContain('netIo');
        expect(
          dockerState.columnVisibility.availableToggles().find((column) => column.id === 'context')
            ?.label,
        ).toBe('Host');
        expect(
          dockerState.columnVisibility.availableToggles().find((column) => column.id === 'disk')
            ?.label,
        ).toBe('Writable layer');
      } finally {
        dispose();
      }
    });

    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const globalState = useWorkloadsControlsState({
          viewMode: () => 'app-container' as ViewMode,
          showFilters,
          setShowFilters,
        });

        const globalDiskColumn = globalState
          .visibleColumns()
          .find((column) => column.id === 'disk');
        expect(globalDiskColumn?.label).toBe('Disk');
        expect(globalDiskColumn).toBeTruthy();
      } finally {
        dispose();
      }
    });
  });
});
