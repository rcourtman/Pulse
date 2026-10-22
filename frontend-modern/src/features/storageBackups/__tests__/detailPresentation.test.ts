import { describe, expect, it } from 'vitest';
import {
  STORAGE_DETAIL_EMPTY_CLASS,
  STORAGE_DETAIL_HEADER_ROW_CLASS,
  STORAGE_DISK_DETAIL_ATTRIBUTE_GRID_CLASS,
  STORAGE_DISK_DETAIL_HEADER_CLASS,
  STORAGE_DISK_DETAIL_HISTORY_GRID_CLASS,
  STORAGE_DISK_DETAIL_LIVE_GRID_CLASS,
  STORAGE_DISK_DETAIL_MODEL_CLASS,
  STORAGE_DISK_DETAIL_NODE_CLASS,
  STORAGE_DISK_DETAIL_SECTION_HEADING_CLASS,
} from '@/features/storageBackups/detailPresentation';

describe('detailPresentation', () => {
  it('exports the embedded disk detail shell classes', () => {
    expect(STORAGE_DETAIL_HEADER_ROW_CLASS).toContain('justify-between');
    expect(STORAGE_DETAIL_EMPTY_CLASS).toContain('text-muted');
    expect(STORAGE_DISK_DETAIL_HEADER_CLASS).toContain('border-b');
    expect(STORAGE_DISK_DETAIL_MODEL_CLASS).toContain('font-semibold');
    expect(STORAGE_DISK_DETAIL_NODE_CLASS).toBe('text-muted');
    expect(STORAGE_DISK_DETAIL_ATTRIBUTE_GRID_CLASS).toContain('min-w-[120px]');
    expect(STORAGE_DISK_DETAIL_LIVE_GRID_CLASS).toContain('sm:grid-cols-3');
    expect(STORAGE_DISK_DETAIL_HISTORY_GRID_CLASS).toContain('min-w-[250px]');
    expect(STORAGE_DISK_DETAIL_SECTION_HEADING_CLASS).toBe('flex items-center gap-2');
  });
});
