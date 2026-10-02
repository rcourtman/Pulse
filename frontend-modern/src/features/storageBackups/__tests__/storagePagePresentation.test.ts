import { describe, expect, it } from 'vitest';
import {
  getStoragePoolCellPaddingClass,
  getStoragePoolColumnWidthStyle,
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
    expect(getStoragePoolTableLayoutModeForContainer(0)).toBe('compact');
    expect(getStoragePoolTableLayoutModeForContainer(359)).toBe('narrow');
    expect(getStoragePoolTableLayoutModeForContainer(360)).toBe('compact');
    // The compact/operational boundary is the shared phone container query
    // (34rem), so the usage bar label and the column set change together.
    expect(getStoragePoolTableLayoutModeForContainer(543)).toBe('compact');
    expect(getStoragePoolTableLayoutModeForContainer(544)).toBe('operational');
    expect(getStoragePoolTableLayoutModeForContainer(1_039)).toBe('operational');
    expect(getStoragePoolTableLayoutModeForContainer(1_040)).toBe('full');
    expect(isStoragePoolColumnVisible('operational', 'host')).toBe(true);
    expect(isStoragePoolColumnVisible('operational', 'growth')).toBe(false);
    expect(getStoragePoolColumnWidthStyle('operational', 'usage')).toEqual({ width: '25%' });
    expect(getStoragePoolColumnWidthStyle('operational', 'name')).toEqual({ width: '27%' });
    expect(isStoragePoolColumnVisible('compact', 'host')).toBe(true);
    expect(isStoragePoolColumnVisible('compact', 'type')).toBe(false);
    expect(isStoragePoolColumnVisible('compact', 'protection')).toBe(false);
    expect(getStoragePoolColumnWidthStyle('compact', 'name')).toEqual({ width: '37%' });
    expect(getStoragePoolColumnWidthStyle('compact', 'host')).toEqual({ width: '31.5%' });
    expect(getStoragePoolColumnWidthStyle('compact', 'usage')).toEqual({ width: '13%' });
    expect(getStoragePoolColumnWidthStyle('compact', 'growth')).toEqual({ width: '0%' });
    expect(isStoragePoolColumnVisible('narrow', 'type')).toBe(false);
    expect(isStoragePoolColumnVisible('narrow', 'protection')).toBe(false);
    expect(getStoragePoolColumnWidthStyle('narrow', 'name')).toEqual({ width: '35%' });
    expect(getStoragePoolColumnWidthStyle('narrow', 'usage')).toEqual({ width: '13%' });
  });

  it('sheds the desktop cell gutter only on the phone pool layouts', () => {
    expect(getStoragePoolCellPaddingClass('narrow')).toBe('!px-1');
    expect(getStoragePoolCellPaddingClass('compact')).toBe('!px-1');
    expect(getStoragePoolCellPaddingClass('operational')).toBe('');
    expect(getStoragePoolCellPaddingClass('full')).toBe('');
  });

  it('keeps the desktop pool column widths on the canonical weighted helper', () => {
    const fullWidths = ['name', 'state', 'type', 'host', 'protection', 'usage', 'growth'].map(
      (column) =>
        getStoragePoolColumnWidthStyle(
          'full',
          column as 'name' | 'state' | 'type' | 'host' | 'protection' | 'usage' | 'growth',
        ).width,
    );
    expect(fullWidths).toEqual(['20%', '14%', '10%', '12%', '13%', '20%', '11%']);
  });
});
