import { batch, createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';
import { createStore, reconcile, type SetStoreFunction } from 'solid-js/store';

import { useTableWindowing } from '@/components/Infrastructure/useTableWindowing';
import {
  bindWindowedPageScrollEvents,
  findWindowedPageScrollContainer,
  wheelDeltaInPixels,
} from '@/components/shared/windowedPageScroll';
import type { StorageRecord } from '@/features/storageBackups/models';
import type { StoragePoolsTableGroupModel } from '@/features/storageBackups/storagePoolsTablePresentation';

const STORAGE_POOL_WINDOW_SIZE = 72;
const STORAGE_POOL_ESTIMATED_ROW_HEIGHT = 32;
const STORAGE_POOL_TABLE_DIVIDER_HEIGHT = 1;

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

type UseStoragePoolsTableWindowingOptions = {
  groups: Accessor<readonly StoragePoolsTableGroupModel[]>;
  expandedPoolId: Accessor<string | null>;
};

type StableStoragePoolsTableItem = {
  item: StoragePoolsTableItem;
  setItem: SetStoreFunction<StoragePoolsTableItem>;
};

export const useStoragePoolsTableWindowing = (options: UseStoragePoolsTableWindowingOptions) => {
  const [bodyRef, setBodyRef] = createSignal<HTMLTableSectionElement | null>(null);
  const [estimatedRowHeight, setEstimatedRowHeight] = createSignal(
    STORAGE_POOL_ESTIMATED_ROW_HEIGHT,
  );
  const itemCache = new Map<string, StableStoragePoolsTableItem>();
  const stabilizeItems = (nextItems: readonly StoragePoolsTableItem[]) => {
    const liveKeys = new Set(nextItems.map((item) => item.key));
    const stableItems = nextItems.map((nextItem) => {
      const cached = itemCache.get(nextItem.key);
      if (cached && cached.item.kind === nextItem.kind) {
        cached.setItem(reconcile(nextItem));
        return cached.item;
      }
      const [item, setItem] = createStore<StoragePoolsTableItem>(nextItem);
      itemCache.set(nextItem.key, { item, setItem });
      return item;
    });
    for (const key of itemCache.keys()) {
      if (!liveKeys.has(key)) itemCache.delete(key);
    }
    return stableItems;
  };
  const [items, setItems] = createSignal<readonly StoragePoolsTableItem[]>(
    stabilizeItems(buildStoragePoolsTableItems(options.groups())),
  );
  createEffect(() => {
    const nextItems = buildStoragePoolsTableItems(options.groups());
    batch(() => setItems(stabilizeItems(nextItems)));
  });
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
    const scrollContainer = findWindowedPageScrollContainer(body);
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

    const scrollContainer = findWindowedPageScrollContainer(body);
    const scrollTarget: HTMLElement | Window = scrollContainer ?? window;
    const viewportHeight = () =>
      scrollContainer?.clientHeight || window.innerHeight || estimatedRowHeight();
    const handleScroll = () => syncWindowToViewport();
    const handleWheel = (event: Event) => {
      const wheelEvent = event as WheelEvent;
      if (wheelEvent.deltaY === 0) return;
      syncWindowToViewport(wheelDeltaInPixels(wheelEvent, viewportHeight()));
    };
    const handleResize = () => syncWindowToViewport(0, true);

    handleResize();
    onCleanup(
      bindWindowedPageScrollEvents({
        scrollTarget,
        onScroll: handleScroll,
        onWheel: handleWheel,
        onResize: handleResize,
      }),
    );
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
