import { describe, expect, it } from 'vitest';
import {
  STORAGE_FILTER_COMPACT_SELECT_CLASS,
  STORAGE_FILTER_RESET_ACTION_CLASS,
  STORAGE_FILTER_SEGMENTED_WRAP_CLASS,
  STORAGE_FILTER_SORT_DIRECTION_BUTTON_CLASS,
  STORAGE_FILTER_SORT_ICON_CLASS,
  STORAGE_FILTER_SORT_WRAP_CLASS,
  STORAGE_FILTER_SORT_SELECT_CLASS,
  getNextStorageSortDirection,
  getStorageSortDirectionIconClass,
  getStorageSortDirectionTitle,
} from '@/features/storageBackups/storageFilterPresentation';

describe('storage filter presentation', () => {
  it('centralizes storage sort-direction semantics', () => {
    expect(STORAGE_FILTER_SORT_SELECT_CLASS).toContain('focus:ring-blue-500');
    expect(STORAGE_FILTER_SORT_DIRECTION_BUTTON_CLASS).toContain('hover:bg-surface-hover');
    expect(STORAGE_FILTER_COMPACT_SELECT_CLASS).toBe('min-w-[8rem]');
    expect(STORAGE_FILTER_SEGMENTED_WRAP_CLASS).toContain('overflow-x-auto');
    expect(STORAGE_FILTER_SORT_WRAP_CLASS).toContain('gap-1.5');
    expect(STORAGE_FILTER_SORT_ICON_CLASS).toContain('transition-transform');
    expect(STORAGE_FILTER_RESET_ACTION_CLASS).toBe('text-base-content');
    expect(getStorageSortDirectionTitle('asc')).toBe('Sort descending');
    expect(getNextStorageSortDirection('asc')).toBe('desc');
    expect(getStorageSortDirectionIconClass('asc')).toBe('rotate-180');
    expect(getStorageSortDirectionIconClass('desc')).toBe('');
  });
});
