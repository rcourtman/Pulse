import { describe, expect, it } from 'vitest';
import {
  STORAGE_DETAIL_CELL_CLASS,
  STORAGE_DETAIL_FULL_WIDTH_ROW_CLASS,
  STORAGE_DETAIL_HEADER_ROW_CLASS,
  STORAGE_DETAIL_LINKED_DISK_MODEL_CLASS,
  STORAGE_DETAIL_LINKED_DISKS_LIST_CLASS,
  STORAGE_DETAIL_MUTED_TEXT_CLASS,
  STORAGE_DETAIL_ROOT_GRID_CLASS,
  STORAGE_DETAIL_ROW_CLASS,
  STORAGE_DISK_DETAIL_ATTRIBUTE_GRID_CLASS,
  STORAGE_DISK_DETAIL_HEADER_CLASS,
  STORAGE_DISK_DETAIL_HISTORY_GRID_CLASS,
  STORAGE_DISK_DETAIL_HISTORY_SELECT_WRAP_CLASS,
  STORAGE_DISK_DETAIL_LIVE_GRID_CLASS,
  STORAGE_DISK_DETAIL_MODEL_CLASS,
  STORAGE_DISK_DETAIL_NODE_CLASS,
  STORAGE_DISK_DETAIL_SECTION_HEADING_CLASS,
} from '@/features/storageBackups/detailPresentation';

describe('detailPresentation', () => {
  it('exports canonical storage detail shell classes', () => {
    expect(STORAGE_DETAIL_ROW_CLASS).toBe('border-t border-border');
    expect(STORAGE_DETAIL_CELL_CLASS).toBe('bg-surface-alt px-4 py-4');
    expect(STORAGE_DETAIL_ROOT_GRID_CLASS).toContain('md:grid-cols-2');
    expect(STORAGE_DETAIL_HEADER_ROW_CLASS).toContain('justify-between');
    expect(STORAGE_DETAIL_LINKED_DISKS_LIST_CLASS).toBe('space-y-1');
    expect(STORAGE_DETAIL_FULL_WIDTH_ROW_CLASS).toBe('col-span-2');
    expect(STORAGE_DETAIL_LINKED_DISK_MODEL_CLASS).toContain('flex-1');
    expect(STORAGE_DETAIL_MUTED_TEXT_CLASS).toBe('text-muted');
    expect(STORAGE_DISK_DETAIL_HEADER_CLASS).toContain('border-b');
    expect(STORAGE_DISK_DETAIL_MODEL_CLASS).toContain('font-semibold');
    expect(STORAGE_DISK_DETAIL_NODE_CLASS).toBe('text-muted');
    expect(STORAGE_DISK_DETAIL_HISTORY_SELECT_WRAP_CLASS).toBe('relative');
    expect(STORAGE_DISK_DETAIL_ATTRIBUTE_GRID_CLASS).toContain('min-w-[120px]');
    expect(STORAGE_DISK_DETAIL_LIVE_GRID_CLASS).toContain('sm:grid-cols-3');
    expect(STORAGE_DISK_DETAIL_HISTORY_GRID_CLASS).toContain('min-w-[250px]');
    expect(STORAGE_DISK_DETAIL_SECTION_HEADING_CLASS).toBe('flex items-center gap-2');
  });
});
