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
    setWideViewport();
  });

  it('lets an explicit toggle pin a layout-hidden column into the compact table', () => {
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

        // netIo is layout-gated to the wide breakpoint: invisible at 1280px
        // and reported hidden by the menu even though the user never hid it.
        expect(state.workloadTableVisibleColumnIds()).not.toContain('netIo');
        expect(menu.isColumnHidden('netIo')).toBe(true);

        // First toggle pins it into view instead of flipping a flag the user
        // cannot see the effect of.
        menu.onColumnToggle('netIo');
        expect(state.workloadTableVisibleColumnIds()).toContain('netIo');
        expect(menu.isColumnHidden('netIo')).toBe(false);
        expect(state.columnVisibility.hiddenColumns()).not.toContain('netIo');

        // Second toggle unpins it; the column returns to its layout default
        // rather than becoming user-hidden on wide viewports too.
        menu.onColumnToggle('netIo');
        expect(state.workloadTableVisibleColumnIds()).not.toContain('netIo');
        expect(state.columnVisibility.hiddenColumns()).not.toContain('netIo');

        // Layout-visible columns keep the plain hide/show semantics.
        menu.onColumnToggle('backup');
        expect(state.columnVisibility.hiddenColumns()).toContain('backup');
        menu.onColumnToggle('backup');
        expect(state.columnVisibility.hiddenColumns()).not.toContain('backup');

        // Reset clears pinned columns along with user-hidden ones.
        menu.onColumnToggle('netIo');
        expect(state.workloadTableVisibleColumnIds()).toContain('netIo');
        menu.onColumnReset();
        expect(state.workloadTableVisibleColumnIds()).not.toContain('netIo');
      } finally {
        dispose();
      }
    });
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
