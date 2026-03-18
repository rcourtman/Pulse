import { describe, expect, it } from 'vitest';
import {
  STORAGE_BAR_LABEL_TEXT_CLASS,
  STORAGE_BAR_LABEL_WRAP_CLASS,
  STORAGE_BAR_PROGRESS_CLASS,
  STORAGE_BAR_PULSE_OVERLAY_CLASS,
  STORAGE_BAR_ROOT_CLASS,
  STORAGE_BAR_TOOLTIP_LABEL_CLASS,
  STORAGE_BAR_TOOLTIP_TITLE_CLASS,
  STORAGE_BAR_TOOLTIP_VALUE_CLASS,
  STORAGE_BAR_TOOLTIP_WRAP_CLASS,
  STORAGE_BAR_ZFS_HEADING_CLASS,
  STORAGE_BAR_ZFS_SECTION_CLASS,
  STORAGE_BAR_ZFS_STATE_LABEL_CLASS,
  STORAGE_BAR_ZFS_STATE_ROW_CLASS,
  getStorageBarLabel,
  getStorageBarTooltipRowClass,
  getStorageBarTooltipRows,
  getStorageBarTooltipTitle,
  getStorageBarUsagePercent,
  getStorageBarZfsHeadingLabel,
  getStorageBarZfsSummary,
} from '@/features/storageBackups/storageBarPresentation';

describe('storageBarPresentation', () => {
  it('centralizes usage math and tooltip row formatting', () => {
    expect(STORAGE_BAR_ROOT_CLASS).toContain('metric-text');
    expect(STORAGE_BAR_PROGRESS_CLASS).toBe('h-full');
    expect(STORAGE_BAR_PULSE_OVERLAY_CLASS).toContain('animate-pulse');
    expect(STORAGE_BAR_LABEL_WRAP_CLASS).toContain('absolute inset-0');
    expect(STORAGE_BAR_LABEL_TEXT_CLASS).toContain('text-ellipsis');
    expect(STORAGE_BAR_TOOLTIP_WRAP_CLASS).toBe('min-w-[160px]');
    expect(STORAGE_BAR_TOOLTIP_TITLE_CLASS).toContain('border-b');
    expect(STORAGE_BAR_TOOLTIP_LABEL_CLASS).toBe('text-slate-400');
    expect(STORAGE_BAR_TOOLTIP_VALUE_CLASS).toBe('text-base-content');
    expect(STORAGE_BAR_ZFS_SECTION_CLASS).toContain('border-t');
    expect(STORAGE_BAR_ZFS_HEADING_CLASS).toContain('text-blue-300');
    expect(STORAGE_BAR_ZFS_STATE_ROW_CLASS).toContain('justify-between');
    expect(STORAGE_BAR_ZFS_STATE_LABEL_CLASS).toBe('text-slate-400');
    expect(getStorageBarUsagePercent(40, 100)).toBe(40);
    expect(getStorageBarLabel(40, 100)).toBe('40% (40.0 B/100 B)');
    expect(getStorageBarTooltipTitle()).toBe('Storage Details');
    expect(getStorageBarTooltipRowClass()).toBe('flex justify-between gap-3 py-0.5 ');
    expect(getStorageBarTooltipRowClass(true)).toBe(
      'flex justify-between gap-3 py-0.5 border-t border-border mt-0.5 pt-0.5',
    );
    expect(getStorageBarZfsHeadingLabel()).toBe('ZFS Status');
    expect(getStorageBarTooltipRows(40, 60, 100)).toEqual([
      { label: 'Used', value: '40.0 B' },
      { label: 'Free', value: '60.0 B' },
      { label: 'Total', value: '100 B', bordered: true },
    ]);
  });

  it('derives zfs summary state canonically', () => {
    expect(
      getStorageBarZfsSummary({
        state: 'DEGRADED',
        scan: 'resilver in progress',
        readErrors: 1,
        writeErrors: 2,
        checksumErrors: 3,
      } as any),
    ).toEqual({
      hasErrors: true,
      isScrubbing: false,
      isResilvering: true,
      state: 'DEGRADED',
      scan: 'resilver in progress',
      errorSummary: 'Errors: R:1 W:2 C:3',
    });
  });
});
