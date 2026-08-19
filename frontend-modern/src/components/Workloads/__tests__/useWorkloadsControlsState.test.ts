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

  it('URL-backs free-text search via q so bookmarks capture -term exclusions', () => {
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

  it('uses the remaining container subtype for columns when a platform excludes the other subtype', () => {
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const proxmoxState = useWorkloadsControlsState({
          viewMode: () => 'container' as ViewMode,
          excludedWorkloadTypes: ['app-container'],
          showFilters,
          setShowFilters,
        });

        const visibleColumnIds = proxmoxState.visibleColumns().map((column) => column.id);
        expect(visibleColumnIds).toContain('vmid');
        expect(visibleColumnIds).toContain('backup');
        expect(visibleColumnIds).not.toContain('runtime');
        expect(visibleColumnIds).not.toContain('image');
        expect(visibleColumnIds).not.toContain('context');
        expect(visibleColumnIds).not.toContain('update');
      } finally {
        dispose();
      }
    });

    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const appContainerState = useWorkloadsControlsState({
          viewMode: () => 'container' as ViewMode,
          excludedWorkloadTypes: ['system-container'],
          showFilters,
          setShowFilters,
        });

        const visibleColumnIds = appContainerState.visibleColumns().map((column) => column.id);
        expect(visibleColumnIds).toContain('runtime');
        expect(visibleColumnIds).toContain('image');
        expect(visibleColumnIds).toContain('context');
        expect(visibleColumnIds).toContain('update');
        expect(visibleColumnIds).not.toContain('vmid');
        expect(visibleColumnIds).not.toContain('backup');
      } finally {
        dispose();
      }
    });
  });

  // The Backup column reads only `resource.proxmox.lastBackup`, so scopes that
  // opt into hiding it (vSphere) rendered "None" on every row for anyone whose
  // saved preference predates that default. The one-time migration retires that
  // state without touching Proxmox, where the column has real data.
  it('retires a stale Backup preference on scopes that default-hide it, and leaves Proxmox alone', async () => {
    localStorage.setItem(
      'workloadsHiddenColumns:vmware-vms',
      JSON.stringify(['aiContext', 'os', 'ip']),
    );

    const disposeVmware = createRoot((dispose) => {
      const [showFilters, setShowFilters] = createSignal(false);
      const state = useWorkloadsControlsState({
        viewMode: () => 'all' as ViewMode,
        showFilters,
        setShowFilters,
        columnVisibilityStorageScope: 'vmware-vms',
        additionalDefaultHiddenColumnIds: ['backup'],
      });
      expect(state.columnVisibility.hiddenColumns()).toContain('backup');
      expect(state.visibleColumns().map((column) => column.id)).not.toContain('backup');
      return dispose;
    });

    await Promise.resolve();
    disposeVmware();

    expect(JSON.parse(localStorage.getItem('workloadsHiddenColumns:vmware-vms') ?? '[]')).toContain(
      'backup',
    );
    expect(
      JSON.parse(
        localStorage.getItem('workloadsHiddenColumns:vmware-vms:default-hidden-applied') ?? '[]',
      ),
    ).toContain('backup');

    // Showing it again sticks: the migration marker stops the next mount from
    // re-hiding a column the user deliberately restored.
    localStorage.setItem(
      'workloadsHiddenColumns:vmware-vms',
      JSON.stringify(['aiContext', 'os', 'ip']),
    );
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const state = useWorkloadsControlsState({
          viewMode: () => 'all' as ViewMode,
          showFilters,
          setShowFilters,
          columnVisibilityStorageScope: 'vmware-vms',
          additionalDefaultHiddenColumnIds: ['backup'],
        });
        expect(state.columnVisibility.hiddenColumns()).not.toContain('backup');
      } finally {
        dispose();
      }
    });

    // Proxmox never opts into hiding Backup, so the migration must not fire there.
    localStorage.setItem('workloadsHiddenColumns', JSON.stringify(['aiContext', 'os', 'ip']));
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const state = useWorkloadsControlsState({
          viewMode: () => 'all' as ViewMode,
          showFilters,
          setShowFilters,
        });
        expect(state.columnVisibility.hiddenColumns()).not.toContain('backup');
        expect(state.visibleColumns().map((column) => column.id)).toContain('backup');
      } finally {
        dispose();
      }
    });
  });

  // vSphere briefly default-hid Tags while the adapter shipped provenance
  // strings in place of real vCenter tags. The adapter now reads vCenter's
  // tagging API, so no scope default-hides the column and none retires a saved
  // preference for it — a scoped hide must not outlive the reason for it.
  it('keeps the Tags column on every scope now that vSphere carries real vCenter tags', async () => {
    localStorage.setItem(
      'workloadsHiddenColumns:vmware-vms',
      JSON.stringify(['aiContext', 'os', 'ip']),
    );

    const disposeVmware = createRoot((dispose) => {
      const [showFilters, setShowFilters] = createSignal(false);
      const state = useWorkloadsControlsState({
        viewMode: () => 'all' as ViewMode,
        showFilters,
        setShowFilters,
        columnVisibilityStorageScope: 'vmware-vms',
        // The vSphere scope's live default-hidden set: Backup only.
        additionalDefaultHiddenColumnIds: ['backup'],
      });
      expect(state.columnVisibility.hiddenColumns()).not.toContain('tags');
      expect(state.visibleColumns().map((column) => column.id)).toContain('tags');
      // The Backup hide is unrelated and still stands.
      expect(state.columnVisibility.hiddenColumns()).toContain('backup');
      return dispose;
    });

    await Promise.resolve();
    disposeVmware();

    localStorage.setItem('workloadsHiddenColumns', JSON.stringify(['aiContext', 'os', 'ip']));
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const state = useWorkloadsControlsState({
          viewMode: () => 'all' as ViewMode,
          showFilters,
          setShowFilters,
        });
        expect(state.columnVisibility.hiddenColumns()).not.toContain('tags');
        expect(state.visibleColumns().map((column) => column.id)).toContain('tags');
      } finally {
        dispose();
      }
    });
  });

  // The Availability cell is empty until a check is linked to that workload,
  // so the column is gated on live data rather than on a stored preference.
  it('drops the Availability column while nothing is probed and restores it once something is', () => {
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const [probed, setProbed] = createSignal(false);
        const state = useWorkloadsControlsState({
          viewMode: () => 'all' as ViewMode,
          showFilters,
          setShowFilters,
          hasAvailabilityData: probed,
        });

        expect(state.visibleColumns().map((column) => column.id)).not.toContain('availability');
        expect(state.columnVisibility.availableToggles().map((column) => column.id)).not.toContain(
          'availability',
        );
        // Gating the column must not write a hidden preference, otherwise the
        // user's own choice would be overwritten the moment a probe appears.
        expect(state.columnVisibility.hiddenColumns()).not.toContain('availability');

        setProbed(true);
        expect(state.visibleColumns().map((column) => column.id)).toContain('availability');
        expect(state.columnVisibility.availableToggles().map((column) => column.id)).toContain(
          'availability',
        );
      } finally {
        dispose();
      }
    });
  });

  it('keeps the Availability column when no probe accessor is supplied', () => {
    createRoot((dispose) => {
      try {
        const [showFilters, setShowFilters] = createSignal(false);
        const state = useWorkloadsControlsState({
          viewMode: () => 'all' as ViewMode,
          showFilters,
          setShowFilters,
        });
        expect(state.visibleColumns().map((column) => column.id)).toContain('availability');
      } finally {
        dispose();
      }
    });
  });
});
