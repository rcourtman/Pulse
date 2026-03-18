import { describe, expect, it } from 'vitest';
import {
  buildStorageGroupRowPresentation,
  getStorageGroupChevronClass,
  getStorageGroupHealthCountPresentation,
  getStorageGroupPoolCountLabel,
  getStorageGroupUsagePercentLabel,
  STORAGE_GROUP_ROW_CHEVRON_BASE_CLASS,
  STORAGE_GROUP_ROW_CLASS,
  STORAGE_GROUP_ROW_HEALTH_DOT_CLASS,
  STORAGE_GROUP_ROW_HEALTH_WRAP_CLASS,
  STORAGE_GROUP_ROW_LABEL_CLASS,
} from '@/features/storageBackups/groupPresentation';

describe('storage group presentation', () => {
  it('formats group pool counts canonically', () => {
    expect(STORAGE_GROUP_ROW_CLASS).toContain('cursor-pointer');
    expect(STORAGE_GROUP_ROW_LABEL_CLASS).toContain('font-semibold');
    expect(STORAGE_GROUP_ROW_HEALTH_WRAP_CLASS).toContain('ml-auto');
    expect(STORAGE_GROUP_ROW_CHEVRON_BASE_CLASS).toContain('transition-transform');
    expect(STORAGE_GROUP_ROW_HEALTH_DOT_CLASS).toBe('w-2 h-2 rounded-full');
    expect(getStorageGroupPoolCountLabel(1)).toBe('1 pool');
    expect(getStorageGroupPoolCountLabel(2)).toBe('2 pools');
  });

  it('formats usage with the shared percent formatter', () => {
    expect(getStorageGroupUsagePercentLabel(40)).toBe('40%');
  });

  it('returns visible health counts in canonical order', () => {
    expect(
      getStorageGroupHealthCountPresentation({
        healthy: 1,
        warning: 0,
        critical: 2,
        offline: 0,
        unknown: 1,
      }),
    ).toEqual([
      {
        health: 'healthy',
        count: 1,
        label: 'Healthy',
        dotClass: 'bg-green-500',
        countClass: 'text-muted',
      },
      {
        health: 'critical',
        count: 2,
        label: 'Critical',
        dotClass: 'bg-red-500',
        countClass: 'text-red-600 dark:text-red-400',
      },
      {
        health: 'unknown',
        count: 1,
        label: 'Unknown',
        dotClass: 'bg-slate-300',
        countClass: 'text-muted',
      },
    ]);
  });

  it('builds group row presentation canonically', () => {
    expect(
      buildStorageGroupRowPresentation({
        key: 'tower',
        items: [{ id: 'pool-1' }, { id: 'pool-2' }] as any,
        stats: {
          totalBytes: 100,
          usedBytes: 40,
          usagePercent: 40,
          byHealth: {
            healthy: 1,
            warning: 1,
            critical: 0,
            offline: 0,
            unknown: 0,
          },
        },
      } as any),
    ).toMatchObject({
      label: 'tower',
      showUsage: true,
      usagePercentLabel: '40%',
      poolCountLabel: '2 pools',
    });
    expect(getStorageGroupChevronClass(true)).toContain('rotate-90');
  });
});
