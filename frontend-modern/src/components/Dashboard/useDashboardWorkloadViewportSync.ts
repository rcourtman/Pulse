import { createEffect, onCleanup, type Accessor } from 'solid-js';

import type { UseGroupedTableWindowingResult } from './useGroupedTableWindowing';

interface DashboardWorkloadViewportSyncOptions {
  filteredGuestCount: Accessor<number>;
  groupedWindowing: UseGroupedTableWindowingResult;
  rowHeight: number;
  tableBodyRef: Accessor<HTMLTableSectionElement | null>;
}

export function useDashboardWorkloadViewportSync(
  options: DashboardWorkloadViewportSyncOptions,
) {
  const syncGuestWindowToViewport = () => {
    if (!options.groupedWindowing.isWindowed() || typeof window === 'undefined') return;
    const body = options.tableBodyRef();
    if (!body) return;
    const rect = body.getBoundingClientRect();
    const scrollTop = Math.max(0, -rect.top);
    options.groupedWindowing.onScroll(
      scrollTop,
      window.innerHeight,
      options.rowHeight,
    );
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    options.filteredGuestCount();
    if (!options.groupedWindowing.isWindowed()) return;
    if (!options.tableBodyRef()) return;

    const handleViewportChange = () => {
      syncGuestWindowToViewport();
    };

    handleViewportChange();
    window.addEventListener('scroll', handleViewportChange, { passive: true });
    window.addEventListener('resize', handleViewportChange);
    onCleanup(() => {
      window.removeEventListener('scroll', handleViewportChange);
      window.removeEventListener('resize', handleViewportChange);
    });
  });
}
