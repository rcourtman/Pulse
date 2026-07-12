import { describe, expect, it } from 'vitest';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import type {
  StorageGroupedRecords,
  StorageGroupKey,
} from '@/components/Storage/useStorageModel';
import {
  buildStorageSummaryGroupId,
  buildStorageSummaryGroupScope,
} from '@/components/Storage/storageSummaryGroups';

const emptyByHealth = (): Record<NormalizedHealth, number> => ({
  healthy: 0,
  warning: 0,
  critical: 0,
  offline: 0,
  unknown: 0,
});

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'pool-1',
  category: 'pool',
  health: 'healthy',
  location: { label: 'pve1', scope: 'node' },
  capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
  capabilities: [],
  source: {
    platform: 'truenas',
    family: 'generic',
    origin: 'resource',
    adapterId: 'adapter-1',
  },
  observedAt: 0,
  ...overrides,
});

const makeGroup = (overrides: Partial<StorageGroupedRecords> = {}): StorageGroupedRecords => ({
  key: 'pve1',
  items: [],
  stats: {
    totalBytes: 0,
    usedBytes: 0,
    usagePercent: 0,
    byHealth: emptyByHealth(),
  },
  ...overrides,
});

describe('buildStorageSummaryGroupId (branch coverage)', () => {
  it('returns null when groupBy is "none" (first guard true arm)', () => {
    // `if (groupBy === 'none')` -> true: short-circuits before touching groupKey.
    expect(buildStorageSummaryGroupId('none', 'pve1')).toBeNull();
    // Even a would-be-valid key is irrelevant once groupBy is 'none'.
    expect(buildStorageSummaryGroupId('none', 'anything')).toBeNull();
  });

  it('returns null when groupBy is not "none" but groupKey trims to empty', () => {
    // First guard false arm + second guard (`!normalizedGroupKey`) true arm.
    // `normalizeStorageSummaryGroupKey` is `value.trim()`; each of these trims to ''.
    expect(buildStorageSummaryGroupId('node', '')).toBeNull();
    expect(buildStorageSummaryGroupId('type', '   ')).toBeNull();
    expect(buildStorageSummaryGroupId('status', '\t\n')).toBeNull();
  });

  it.each<[StorageGroupKey, string]>([
    ['node', 'pve1'],
    ['type', 'zfs'],
    ['status', 'healthy'],
  ])('composes the id as storage:%s:<trimmed key> for groupBy %s (happy path)', (groupBy, key) => {
    // Both guards false -> final return statement.
    expect(buildStorageSummaryGroupId(groupBy, key)).toBe(`storage:${groupBy}:${key}`);
  });

  it('uses the trimmed groupKey in the composed id, not the raw whitespace-padded input', () => {
    // Confirms normalizeStorageSummaryGroupKey (trim) is applied before interpolation.
    expect(buildStorageSummaryGroupId('node', '  pve1  ')).toBe('storage:node:pve1');
    expect(buildStorageSummaryGroupId('type', '\tZFS\n')).toBe('storage:type:ZFS');
  });
});

describe('buildStorageSummaryGroupScope (branch coverage)', () => {
  it('returns null when the resolved id is null because groupBy is "none"', () => {
    // `if (!id)` true arm, reached via buildStorageSummaryGroupId -> 'none' guard.
    const group = makeGroup({ items: [makeRecord()] });
    expect(buildStorageSummaryGroupScope(group, 'none')).toBeNull();
  });

  it('returns null when the resolved id is null because group.key trims to empty', () => {
    // `if (!id)` true arm, reached via the empty-normalized-key guard inside buildStorageSummaryGroupId.
    const group = makeGroup({ key: '   ', items: [makeRecord()] });
    expect(buildStorageSummaryGroupScope(group, 'node')).toBeNull();
  });

  it('returns null when the group has no items (seriesIds.length === 0)', () => {
    // `if (seriesIds.length === 0)` true arm: items.map -> [] -> Set -> [] -> length 0.
    const group = makeGroup({ items: [] });
    expect(buildStorageSummaryGroupScope(group, 'node')).toBeNull();
  });

  it('returns null when every item resolves to an empty/falsy metric resource id', () => {
    // `if (seriesIds.length === 0)` true arm, but items is non-empty: each record's
    // resolveStorageRecordMetricResourceId falls through metricsTarget/refs to id,
    // and `.filter(Boolean)` strips the empty strings.
    const blankRecord = makeRecord({ id: '' });
    const group = makeGroup({ items: [blankRecord, makeRecord({ id: '   ' })] });
    expect(buildStorageSummaryGroupScope(group, 'node')).toBeNull();
  });

  it('builds a scope using the record id fallback when items have no metricsTarget/refs', () => {
    // Happy path: resolveStorageRecordMetricResourceId's third operand (record.id).
    const group = makeGroup({
      key: 'pve1',
      items: [makeRecord({ id: 'storage-1' }), makeRecord({ id: 'storage-2' })],
    });
    expect(buildStorageSummaryGroupScope(group, 'node')).toStrictEqual({
      id: 'storage:node:pve1',
      label: 'pve1 (2 storage items)',
      seriesIds: ['storage-1', 'storage-2'],
    });
  });

  it('prefers metricsTarget.resourceId over refs.resourceId and record.id in the emitted seriesIds', () => {
    // resolveStorageRecordMetricResourceId's first operand wins: metricsTarget.resourceId.
    const group = makeGroup({
      key: 'node-a',
      items: [
        makeRecord({
          id: 'storage-1',
          metricsTarget: { resourceType: 'pool', resourceId: 'series-via-metrics' },
          refs: { resourceId: 'series-via-refs' },
        }),
      ],
    });
    expect(buildStorageSummaryGroupScope(group, 'node')).toStrictEqual({
      id: 'storage:node:node-a',
      label: 'node-a (1 storage item)',
      seriesIds: ['series-via-metrics'],
    });
  });

  it('falls back to refs.resourceId when metricsTarget.resourceId is absent/blank', () => {
    // resolveStorageRecordMetricResourceId's second operand: refs.resourceId.
    const group = makeGroup({
      key: 'node-b',
      items: [
        makeRecord({
          id: 'storage-1',
          metricsTarget: { resourceType: 'pool', resourceId: '   ' },
          refs: { resourceId: 'series-via-refs' },
        }),
      ],
    });
    expect(buildStorageSummaryGroupScope(group, 'type')).toStrictEqual({
      id: 'storage:type:node-b',
      label: 'node-b (1 storage item)',
      seriesIds: ['series-via-refs'],
    });
  });

  it('deduplicates seriesIds that collapse to the same resource id (new Set de-dup arm)', () => {
    // `new Set(...)` collapses duplicates while preserving first-seen order.
    const group = makeGroup({
      key: 'pve1',
      items: [
        makeRecord({ id: 'r1', metricsTarget: { resourceType: 'pool', resourceId: 'series-a' } }),
        makeRecord({ id: 'r2', refs: { resourceId: 'series-a' } }),
        makeRecord({ id: 'r3', metricsTarget: { resourceType: 'pool', resourceId: 'series-b' } }),
      ],
    });
    expect(buildStorageSummaryGroupScope(group, 'status')!.seriesIds).toStrictEqual([
      'series-a',
      'series-b',
    ]);
  });

  it('drops blank resolved ids while keeping non-blank ones (filter(Boolean) arm)', () => {
    // A single group with a mix: one record resolves to '' (metricsTarget, refs AND id all
    // blank/whitespace so every operand of resolveStorageRecordMetricResourceId is falsy),
    // another resolves to 'series-a'. filter(Boolean) must drop only the blank one.
    const group = makeGroup({
      key: 'pve1',
      items: [
        makeRecord({
          id: '   ',
          metricsTarget: { resourceType: 'pool', resourceId: '   ' },
          refs: { resourceId: '   ' },
        }),
        makeRecord({ id: 'real', metricsTarget: { resourceType: 'pool', resourceId: 'series-a' } }),
      ],
    });
    expect(buildStorageSummaryGroupScope(group, 'node')!.seriesIds).toStrictEqual(['series-a']);
  });

  it('uses the plural count label when the group has more than one item', () => {
    // getStorageGroupPoolCountLabel: count === 1 ? 'storage item' : 'storage items'.
    const group = makeGroup({
      key: 'pve1',
      items: [
        makeRecord({ id: 'a', metricsTarget: { resourceType: 'pool', resourceId: 's1' } }),
        makeRecord({ id: 'b', metricsTarget: { resourceType: 'pool', resourceId: 's2' } }),
        makeRecord({ id: 'c', metricsTarget: { resourceType: 'pool', resourceId: 's3' } }),
      ],
    });
    expect(buildStorageSummaryGroupScope(group, 'node')!.label).toBe('pve1 (3 storage items)');
  });

  it('propagates a whitespace-padded group key untrimmed into the label but trimmed into the id', () => {
    // Documents current behaviour: the scope `id` is built from the *trimmed* key
    // (via buildStorageSummaryGroupId), but the label is built from
    // buildStorageGroupRowPresentation, which reads `group.key` verbatim.
    const group = makeGroup({
      key: '  pve1  ',
      items: [makeRecord({ id: 'storage-1' })],
    });
    expect(buildStorageSummaryGroupScope(group, 'node')).toStrictEqual({
      id: 'storage:node:pve1',
      label: '  pve1   (1 storage item)',
      seriesIds: ['storage-1'],
    });
  });
});
