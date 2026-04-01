import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';
import { useTableWindowing } from './useTableWindowing';

interface UseUnifiedResourceTableViewportSyncOptions {
  totalCount: Accessor<number>;
  estimatedRowHeight: number;
  hostWindowing: ReturnType<typeof useTableWindowing>;
}

export function useUnifiedResourceTableViewportSync(
  options: UseUnifiedResourceTableViewportSyncOptions,
) {
  const { totalCount, estimatedRowHeight, hostWindowing } = options;
  const [hostBodyRef, setHostBodyRef] = createSignal<HTMLTableSectionElement | null>(null);

  const syncHostWindowToViewport = () => {
    if (!hostWindowing.isWindowed() || typeof window === 'undefined') return;
    const body = hostBodyRef();
    if (!body) return;
    const rect = body.getBoundingClientRect();
    const scrollTop = Math.max(0, -rect.top);
    hostWindowing.onScroll(scrollTop, window.innerHeight, estimatedRowHeight);
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    totalCount();
    if (!hostWindowing.isWindowed()) return;
    if (!hostBodyRef()) return;

    const handleViewportChange = () => {
      syncHostWindowToViewport();
    };

    handleViewportChange();
    window.addEventListener('scroll', handleViewportChange, { passive: true });
    window.addEventListener('resize', handleViewportChange);
    onCleanup(() => {
      window.removeEventListener('scroll', handleViewportChange);
      window.removeEventListener('resize', handleViewportChange);
    });
  });

  return {
    setHostBodyRef,
  };
}

export type UnifiedResourceTableViewportSync = ReturnType<
  typeof useUnifiedResourceTableViewportSync
>;
