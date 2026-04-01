import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';
import { useTableWindowing } from './useTableWindowing';

interface UseUnifiedResourceTableViewportSyncOptions {
  expandedResourceId: Accessor<string | null>;
  totalCount: Accessor<number>;
  estimatedRowHeight: number;
  hostWindowing: ReturnType<typeof useTableWindowing>;
}

export function useUnifiedResourceTableViewportSync(
  options: UseUnifiedResourceTableViewportSyncOptions,
) {
  const { expandedResourceId, totalCount, estimatedRowHeight, hostWindowing } = options;
  const [hostBodyRef, setHostBodyRef] = createSignal<HTMLTableSectionElement | null>(null);
  const rowRefs = new Map<string, HTMLTableRowElement>();

  const syncHostWindowToViewport = () => {
    if (!hostWindowing.isWindowed() || typeof window === 'undefined') return;
    const body = hostBodyRef();
    if (!body) return;
    const rect = body.getBoundingClientRect();
    const scrollTop = Math.max(0, -rect.top);
    hostWindowing.onScroll(scrollTop, window.innerHeight, estimatedRowHeight);
  };

  const registerRowRef = (resourceId: string, element?: HTMLTableRowElement) => {
    if (element) {
      rowRefs.set(resourceId, element);
      return;
    }
    rowRefs.delete(resourceId);
  };

  createEffect(() => {
    const selectedId = expandedResourceId();
    if (!selectedId) return;
    hostWindowing.startIndex();
    hostWindowing.endIndex();
    const row = rowRefs.get(selectedId);
    if (row) {
      const rect = row.getBoundingClientRect();
      const fullyVisible = rect.top >= 0 && rect.bottom <= window.innerHeight;
      if (fullyVisible) return;
      row.scrollIntoView({ block: 'center', behavior: 'smooth' });
    }
  });

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
    registerRowRef,
    setHostBodyRef,
  };
}

export type UnifiedResourceTableViewportSync = ReturnType<
  typeof useUnifiedResourceTableViewportSync
>;
