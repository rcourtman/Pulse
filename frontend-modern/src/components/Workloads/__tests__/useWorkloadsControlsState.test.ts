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

describe('useWorkloadsControlsState', () => {
  beforeEach(() => {
    localStorage.clear();
    setMockRouterSearch('');
    navigateSpy.mockClear();
    setWideViewport();
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
