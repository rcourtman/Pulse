import { describe, expect, it } from 'vitest';
import type { StorageGroupKey } from '@/components/Storage/useStorageModel';
import { buildStorageSummaryGroupId } from '@/components/Storage/storageSummaryGroups';

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
