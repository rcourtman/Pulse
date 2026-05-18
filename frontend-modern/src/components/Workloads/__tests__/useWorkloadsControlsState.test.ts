import { createRoot, createSignal } from 'solid-js';
import { beforeEach, describe, expect, it } from 'vitest';
import type { ViewMode } from '@/types/workloads';
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
    setWideViewport();
  });

  it('lets Docker scope hide writable-layer disk by default without hiding disk globally', () => {
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const dockerState = useWorkloadsControlsState({
          viewMode: () => 'app-container' as ViewMode,
          showFilters,
          setShowFilters,
          columnVisibilityStorageScope: 'docker',
          additionalDefaultHiddenColumnIds: ['disk'],
          columnLabelOverrides: { disk: 'Writable layer' },
        });

        expect(dockerState.columnVisibility.hiddenColumns()).toContain('disk');
        expect(dockerState.visibleColumns().map((column) => column.id)).not.toContain('disk');
        expect(dockerState.visibleColumns().map((column) => column.id)).toContain('diskIo');
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

        const globalDiskColumn = globalState.visibleColumns().find((column) => column.id === 'disk');
        expect(globalDiskColumn?.label).toBe('Disk');
        expect(globalDiskColumn).toBeTruthy();
      } finally {
        dispose();
      }
    });
  });
});
