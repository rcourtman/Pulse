import { describe, expect, it } from 'vitest';
import {
  getStoragePoolColumnWidthPercent,
  getStoragePoolTableColumns,
  getStoragePoolTableLayoutModeForContainer,
  getStorageTableHeading,
  isStoragePoolColumnVisible,
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
    expect(getStoragePoolTableColumns('Growth (24h)').map((column) => column.id)).toEqual([
      'name',
      'state',
      'type',
      'host',
      'protection',
      'usage',
      'growth',
    ]);
  });

  it('selects pool layouts from the rendered table width', () => {
    expect(getStoragePoolTableLayoutModeForContainer(559)).toBe('compact');
    expect(getStoragePoolTableLayoutModeForContainer(560)).toBe('operational');
    expect(getStoragePoolTableLayoutModeForContainer(1_039)).toBe('operational');
    expect(getStoragePoolTableLayoutModeForContainer(1_040)).toBe('full');
    expect(isStoragePoolColumnVisible('operational', 'host')).toBe(true);
    expect(isStoragePoolColumnVisible('operational', 'growth')).toBe(false);
    expect(getStoragePoolColumnWidthPercent('operational', 'usage')).toBe(21);
    expect(isStoragePoolColumnVisible('compact', 'host')).toBe(true);
    expect(isStoragePoolColumnVisible('compact', 'protection')).toBe(true);
    expect(getStoragePoolColumnWidthPercent('compact', 'name')).toBe(30);
    expect(getStoragePoolColumnWidthPercent('compact', 'usage')).toBe(13);
    expect(getStoragePoolColumnWidthPercent('compact', 'growth')).toBe(0);
  });
});
