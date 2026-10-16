import type { Accessor } from 'solid-js';

import { usePlatformWindowedItems } from '@/features/platformPage/usePlatformWindowedItems';

const BACKUP_TABLE_ESTIMATED_ROW_HEIGHT = 32;

// One logical point has a summary and, on phones, a full-width provenance
// row. Include both, and wrapped paths, without counting runway spacers or
// treating the second DOM row as a second item. Day headers are real items.
export function measureProxmoxBackupItemHeight(root: HTMLElement): number {
  const body = root.querySelector('tbody');
  const count = body?.querySelectorAll(
    '[data-proxmox-backup-row="recoverable"], [data-proxmox-backup-row="day"]',
  ).length;
  if (!body || !count) return 0;
  const spacers = Array.from(body.querySelectorAll(':scope > tr[aria-hidden="true"]')).reduce(
    (height, row) => height + row.getBoundingClientRect().height,
    0,
  );
  return Math.max(0, body.getBoundingClientRect().height - spacers) / count;
}

export function useProxmoxBackupTableWindowing<T>(
  items: Accessor<readonly T[]>,
  measurementKey?: Accessor<unknown>,
) {
  const windowing = usePlatformWindowedItems({
    items,
    estimatedItemHeight: BACKUP_TABLE_ESTIMATED_ROW_HEIGHT,
    measureItemHeight: measurementKey ? measureProxmoxBackupItemHeight : undefined,
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
