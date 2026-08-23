import { createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';

import { useTableWindowing } from '@/components/Infrastructure/useTableWindowing';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { StoragePoolsTableGroupModel } from '@/features/storageBackups/storagePoolsTablePresentation';

const STORAGE_POOL_WINDOW_SIZE = 72;
const STORAGE_POOL_ESTIMATED_ROW_HEIGHT = 32;
const STORAGE_POOL_TABLE_DIVIDER_HEIGHT = 1;
const SCROLLABLE_OVERFLOW_PATTERN = /(?:auto|scroll|overlay)/;
const WHEEL_LINE_HEIGHT_PX = 16;

type StoragePoolsTableGroupItem = {
  kind: 'group';
  key: string;
  group: StoragePoolsTableGroupModel;
};

type StoragePoolsTableRecordItem = {
  kind: 'record';
  key: string;
  group: StoragePoolsTableGroupModel;
  record: StorageRecord;
};

export type StoragePoolsTableItem = StoragePoolsTableGroupItem | StoragePoolsTableRecordItem;

export const buildStoragePoolsTableItems = (
  groups: readonly StoragePoolsTableGroupModel[],
): StoragePoolsTableItem[] => {
  const items: StoragePoolsTableItem[] = [];
  for (const group of groups) {
    if (group.showHeader) {
      items.push({ kind: 'group', key: `group:${group.key}`, group });
    }
    if (!group.expanded) continue;
    for (const record of group.items) {
      items.push({ kind: 'record', key: `record:${record.id}`, group, record });
    }
  }
  return items;
};

const findScrollContainer = (element: HTMLElement): HTMLElement | null => {
  let parent = element.parentElement;
  while (parent && parent !== document.body && parent !== document.documentElement) {
    const styles = getComputedStyle(parent);
    const hasVerticalScrollRange = parent.scrollHeight - parent.clientHeight > 1;
    if (
      SCROLLABLE_OVERFLOW_PATTERN.test(styles.overflowY) &&
      (styles.overflowY === 'scroll' || hasVerticalScrollRange)
    ) {
      return parent;
    }
    parent = parent.parentElement;
  }
  return null;
};

const wheelDeltaInPixels = (event: WheelEvent, viewportHeight: number) => {
  if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) return event.deltaY * WHEEL_LINE_HEIGHT_PX;
  if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) return event.deltaY * viewportHeight;
  return event.deltaY;
};

type UseStoragePoolsTableWindowingOptions = {
  groups: Accessor<readonly StoragePoolsTableGroupModel[]>;
  expandedPoolId: Accessor<string | null>;
};

export const useStoragePoolsTableWindowing = (options: UseStoragePoolsTableWindowingOptions) => {
  const [bodyRef, setBodyRef] = createSignal<HTMLTableSectionElement | null>(null);
  const [estimatedRowHeight, setEstimatedRowHeight] = createSignal(
    STORAGE_POOL_ESTIMATED_ROW_HEIGHT,
  );
  const items = createMemo(() => buildStoragePoolsTableItems(options.groups()));
  const expandedRecordIndex = createMemo(() => {
    const expandedId = options.expandedPoolId();
    if (!expandedId) return null;
    const index = items().findIndex(
      (item) => item.kind === 'record' && item.record.id === expandedId,
    );
    return index >= 0 ? index : null;
  });
  const windowing = useTableWindowing({
    totalCount: () => items().length,
    windowSize: STORAGE_POOL_WINDOW_SIZE,
    enabled: () => items().length > STORAGE_POOL_WINDOW_SIZE,
    revealIndex: expandedRecordIndex,
  });

  const visibleItems = createMemo<readonly StoragePoolsTableItem[]>(() => {
    if (!windowing.isWindowed()) return items();
    return items().slice(windowing.startIndex(), windowing.endIndex());
  });
  const topSpacerHeight = createMemo(() => {
    if (!windowing.isWindowed() || windowing.startIndex() <= 0) return 0;
    return Math.max(
      0,
      windowing.startIndex() * estimatedRowHeight() - STORAGE_POOL_TABLE_DIVIDER_HEIGHT,
    );
  });
  const bottomSpacerHeight = createMemo(() =>
    windowing.isWindowed()
      ? Math.max(0, items().length - windowing.endIndex()) * estimatedRowHeight()
      : 0,
  );

  const syncWindowToViewport = (projectedScrollDelta = 0, measureRows = false) => {
    if (typeof window === 'undefined' || !windowing.isWindowed()) return;
    const body = bodyRef();
    if (!body) return;
    if (measureRows) {
      const measuredHeight = body
        .querySelector<HTMLTableRowElement>(':scope > tr[data-summary-series-id]')
        ?.getBoundingClientRect().height;
      if (measuredHeight && measuredHeight > 0) setEstimatedRowHeight(measuredHeight);
    }

    const bodyRect = body.getBoundingClientRect();
    const scrollContainer = findScrollContainer(body);
    if (scrollContainer) {
      const containerRect = scrollContainer.getBoundingClientRect();
      windowing.onScroll(
        Math.max(0, containerRect.top - bodyRect.top + projectedScrollDelta),
        scrollContainer.clientHeight || window.innerHeight,
        estimatedRowHeight(),
      );
      return;
    }
    windowing.onScroll(
      Math.max(0, -bodyRect.top + projectedScrollDelta),
      window.innerHeight,
      estimatedRowHeight(),
    );
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    items().length;
    const body = bodyRef();
    if (!body || !windowing.isWindowed()) return;

    let lastTouchY: number | null = null;
    const scrollContainer = findScrollContainer(body);
    const scrollTarget: HTMLElement | Window = scrollContainer ?? window;
    const viewportHeight = () =>
      scrollContainer?.clientHeight || window.innerHeight || estimatedRowHeight();
    const handleScroll = () => syncWindowToViewport();
    const handleWheel = (event: Event) => {
      const wheelEvent = event as WheelEvent;
      if (wheelEvent.deltaY === 0) return;
      syncWindowToViewport(wheelDeltaInPixels(wheelEvent, viewportHeight()));
    };
    const handleTouchStart = (event: Event) => {
      lastTouchY = (event as TouchEvent).touches.item(0)?.clientY ?? null;
    };
    const handleTouchMove = (event: Event) => {
      const nextTouchY = (event as TouchEvent).touches.item(0)?.clientY ?? null;
      if (nextTouchY == null || lastTouchY == null) {
        lastTouchY = nextTouchY;
        return;
      }
      const deltaY = lastTouchY - nextTouchY;
      lastTouchY = nextTouchY;
      if (deltaY !== 0) syncWindowToViewport(deltaY);
    };
    const handleTouchEnd = () => {
      lastTouchY = null;
    };
    const handleResize = () => syncWindowToViewport(0, true);

    handleResize();
    scrollTarget.addEventListener('scroll', handleScroll, { passive: true });
    // Pre-position the bounded row window before native scrolling advances the viewport.
    scrollTarget.addEventListener('wheel', handleWheel, { passive: false });
    scrollTarget.addEventListener('touchstart', handleTouchStart, { passive: true });
    scrollTarget.addEventListener('touchmove', handleTouchMove, { passive: false });
    scrollTarget.addEventListener('touchend', handleTouchEnd, { passive: true });
    scrollTarget.addEventListener('touchcancel', handleTouchEnd, { passive: true });
    window.addEventListener('resize', handleResize);
    onCleanup(() => {
      scrollTarget.removeEventListener('scroll', handleScroll);
      scrollTarget.removeEventListener('wheel', handleWheel);
      scrollTarget.removeEventListener('touchstart', handleTouchStart);
      scrollTarget.removeEventListener('touchmove', handleTouchMove);
      scrollTarget.removeEventListener('touchend', handleTouchEnd);
      scrollTarget.removeEventListener('touchcancel', handleTouchEnd);
      window.removeEventListener('resize', handleResize);
    });
  });

  return {
    bottomSpacerHeight,
    isWindowed: windowing.isWindowed,
    setBodyRef,
    topSpacerHeight,
    totalCount: () => items().length,
    visibleItems,
  } as const;
};
