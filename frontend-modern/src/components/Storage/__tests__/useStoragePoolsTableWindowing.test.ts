import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';

import type { StorageRecord } from '@/features/storageBackups/models';
import type { StoragePoolsTableGroupModel } from '@/features/storageBackups/storagePoolsTablePresentation';
import {
  buildStoragePoolsTableItems,
  useStoragePoolsTableWindowing,
} from '@/components/Storage/useStoragePoolsTableWindowing';

const makeRecord = (index: number): StorageRecord =>
  ({
    id: `storage-${index}`,
    name: `pool-${index}`,
  }) as StorageRecord;

const makeGroup = (
  key: string,
  records: StorageRecord[],
  options: { expanded?: boolean; showHeader?: boolean } = {},
): StoragePoolsTableGroupModel => ({
  key,
  items: records,
  stats: {
    totalBytes: 0,
    usedBytes: 0,
    usagePercent: 0,
    byHealth: { healthy: 0, warning: 0, critical: 0, offline: 0, unknown: 0 },
  },
  expanded: options.expanded ?? true,
  showHeader: options.showHeader ?? true,
});

describe('useStoragePoolsTableWindowing', () => {
  it('flattens only visible group headers and expanded pool rows', () => {
    const items = buildStoragePoolsTableItems([
      makeGroup('open', [makeRecord(1), makeRecord(2)]),
      makeGroup('closed', [makeRecord(3)], { expanded: false }),
      makeGroup('ungrouped', [makeRecord(4)], { showHeader: false }),
    ]);

    expect(items.map((item) => item.key)).toEqual([
      'group:open',
      'record:storage-1',
      'record:storage-2',
      'group:closed',
      'record:storage-4',
    ]);
  });

  it('keeps a large estate to a bounded 72-item DOM window and reveals expansion targets', () => {
    const [expandedPoolId, setExpandedPoolId] = createSignal<string | null>(null);
    const records = Array.from({ length: 180 }, (_, index) => makeRecord(index));
    const groups = () => [makeGroup('estate', records, { showHeader: false })];
    const { result } = renderHook(() => useStoragePoolsTableWindowing({ groups, expandedPoolId }));

    expect(result.isWindowed()).toBe(true);
    expect(result.totalCount()).toBe(180);
    expect(result.visibleItems()).toHaveLength(72);
    expect(result.visibleItems()[0]?.key).toBe('record:storage-0');

    setExpandedPoolId('storage-150');

    expect(result.visibleItems()).toHaveLength(72);
    expect(result.visibleItems().some((item) => item.key === 'record:storage-150')).toBe(true);
  });
});
