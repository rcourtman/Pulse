import { describe, expect, it } from 'vitest';
import {
  getStoragePoolTableColumns,
  getStorageTableHeading,
  STORAGE_VIEW_OPTIONS,
} from '@/features/storageBackups/storagePagePresentation';

describe('storagePagePresentation', () => {
  it('formats storage table headings canonically', () => {
    expect(getStorageTableHeading('pools')).toBe('Storage');
    expect(getStorageTableHeading('disks')).toBe('Physical Disks');
  });

  it('exports canonical storage view and table column contracts', () => {
    expect(STORAGE_VIEW_OPTIONS).toEqual([
      { value: 'pools', label: 'Storage' },
      { value: 'disks', label: 'Physical Disks' },
    ]);
    expect(getStoragePoolTableColumns('Growth (24h)').map((column) => column.label)).toEqual([
      'Storage',
      'State',
      'Type',
      'Host',
      'Protection',
      'Usage',
      'Growth (24h)',
    ]);
    expect(getStoragePoolTableColumns('Growth (24h)').map((column) => column.compactLabel)).toEqual(
      ['Storage', 'State', 'Type', 'Host', 'Prot', 'Used', '24h'],
    );
    expect(getStoragePoolTableColumns('Growth (24h)')[0].colClassName).toContain('xl:w-[20%]');
    expect(getStoragePoolTableColumns('Growth (24h)')[2].className).toContain(
      'hidden xl:table-cell',
    );
  });
});
