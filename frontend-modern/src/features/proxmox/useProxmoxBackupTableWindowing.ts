import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';

import { usePlatformWindowedItems } from '@/features/platformPage/usePlatformWindowedItems';

const BACKUP_TABLE_ESTIMATED_ROW_HEIGHT = 32;

// One logical point contains its summary and optional phone provenance row.
// Day headers have their own keys. Neither a runway spacer nor a tall neighbour
// can lend its height to another point with differently wrapped identity.
export function measureProxmoxBackupBlocks(root: HTMLElement): ReadonlyMap<string, number> {
  const heights = new Map<string, number>();
  const body = root.querySelector('tbody');
  if (!body) return heights;
  for (const row of body.querySelectorAll<HTMLElement>(':scope > tr[data-proxmox-backup-key]')) {
    const key = row.dataset.proxmoxBackupKey;
    const location = row.nextElementSibling;
    const height =
      row.getBoundingClientRect().height +
      (location?.getAttribute('data-proxmox-backup-row') === 'location'
        ? location.getBoundingClientRect().height
        : 0);
    if (key && Number.isFinite(height) && height > 0) heights.set(key, height);
  }
  return heights;
}

export function useProxmoxBackupTableWindowing<T extends { key: string }>(
  items: Accessor<readonly T[]>,
  measurementKey: Accessor<unknown>,
  layoutKey: Accessor<unknown>,
) {
  const [heights, setHeights] = createSignal<ReadonlyMap<string, number>>(new Map());
  // Width changes invalidate off-screen wrapping as well as the mounted rows.
  createEffect(() => {
    layoutKey();
    setHeights(new Map());
  });
  createEffect(() => {
    const keys = new Set(items().map((item) => item.key));
    setHeights((current) =>
      [...current.keys()].every((key) => keys.has(key))
        ? current
        : new Map([...current].filter(([key]) => keys.has(key))),
    );
  });
  const offsets = createMemo(() => {
    const measured = heights();
    const estimate = measured.size
      ? [...measured.values()].reduce((sum, height) => sum + height, 0) / measured.size
      : BACKUP_TABLE_ESTIMATED_ROW_HEIGHT;
    const result = [0];
    for (const item of items())
      result.push(result[result.length - 1]! + (measured.get(item.key) ?? estimate));
    return result;
  });
  const measure = (root: HTMLElement) => {
    const observed = measureProxmoxBackupBlocks(root);
    setHeights((current) => {
      let next: Map<string, number> | undefined;
      for (const [key, height] of observed) {
        if (current.get(key) === height) continue;
        next ??= new Map(current);
        next.set(key, height);
      }
      return next ?? current;
    });
  };
  const windowing = usePlatformWindowedItems({
    items,
    estimatedItemHeight: BACKUP_TABLE_ESTIMATED_ROW_HEIGHT,
    itemGeometry: { offsets, measure },
    measurementKey,
  });

  return {
    isWindowed: windowing.isWindowed,
    visibleItems: windowing.visibleItems,
    topSpacerHeight: windowing.topSpacerHeight,
    bottomSpacerHeight: windowing.bottomSpacerHeight,
    setRootRef: windowing.setAnchorRef,
  };
}
