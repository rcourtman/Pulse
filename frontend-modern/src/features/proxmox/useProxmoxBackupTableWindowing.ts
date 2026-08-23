import type { Accessor } from 'solid-js';

import { usePlatformWindowedItems } from '@/features/platformPage/usePlatformWindowedItems';

const BACKUP_TABLE_ESTIMATED_ROW_HEIGHT = 32;

export function useProxmoxBackupTableWindowing<T>(items: Accessor<readonly T[]>) {
  const windowing = usePlatformWindowedItems({
    items,
    estimatedItemHeight: BACKUP_TABLE_ESTIMATED_ROW_HEIGHT,
  });

  return {
    isWindowed: windowing.isWindowed,
    visibleItems: windowing.visibleItems,
    topSpacerHeight: windowing.topSpacerHeight,
    bottomSpacerHeight: windowing.bottomSpacerHeight,
    setRootRef: windowing.setAnchorRef,
  };
}
