import { createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';

import { useTableWindowing } from '@/components/Infrastructure/useTableWindowing';

const BACKUP_TABLE_ESTIMATED_ROW_HEIGHT = 32;

export function useProxmoxBackupTableWindowing<T>(items: Accessor<readonly T[]>) {
  const [rootRef, setRootRef] = createSignal<HTMLElement | null>(null);
  const windowing = useTableWindowing({ totalCount: () => items().length });

  const visibleItems = createMemo<readonly T[]>(() => {
    if (!windowing.isWindowed()) return items();
    return items().slice(windowing.startIndex(), windowing.endIndex());
  });
  const topSpacerHeight = createMemo(() =>
    windowing.isWindowed() ? windowing.startIndex() * BACKUP_TABLE_ESTIMATED_ROW_HEIGHT : 0,
  );
  const bottomSpacerHeight = createMemo(() =>
    windowing.isWindowed()
      ? Math.max(0, items().length - windowing.endIndex()) * BACKUP_TABLE_ESTIMATED_ROW_HEIGHT
      : 0,
  );

  const syncWindowToViewport = () => {
    if (!windowing.isWindowed() || typeof window === 'undefined') return;
    const root = rootRef();
    if (!root) return;
    const rect = root.getBoundingClientRect();
    windowing.onScroll(
      Math.max(0, -rect.top),
      window.innerHeight,
      BACKUP_TABLE_ESTIMATED_ROW_HEIGHT,
    );
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    items().length;
    if (!windowing.isWindowed() || !rootRef()) return;

    const handleViewportChange = () => syncWindowToViewport();
    handleViewportChange();
    window.addEventListener('scroll', handleViewportChange, { passive: true });
    window.addEventListener('resize', handleViewportChange);
    onCleanup(() => {
      window.removeEventListener('scroll', handleViewportChange);
      window.removeEventListener('resize', handleViewportChange);
    });
  });

  return {
    isWindowed: windowing.isWindowed,
    visibleItems,
    topSpacerHeight,
    bottomSpacerHeight,
    setRootRef,
  };
}
