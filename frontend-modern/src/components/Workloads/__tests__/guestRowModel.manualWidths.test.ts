import { describe, expect, it } from 'vitest';

import type { ColumnDef } from '@/hooks/useColumnVisibility';

import {
  GUEST_COLUMNS,
  getGuestColumnStyle,
  getGuestColumnWidthStyle,
  getWorkloadVisibleColumnsForLayout,
} from '../guestRowModel';

const columnsFor = (ids: readonly string[]): ColumnDef[] =>
  ids.map((id) => GUEST_COLUMNS.find((column) => column.id === id)!).filter(Boolean);

// The wide-only trio is what operators lose first on a laptop-width table.
const SELECTED_COLUMN_IDS = [
  'name',
  'type',
  'info',
  'cpu',
  'memory',
  'disk',
  'uptime',
  'tags',
  'netIo',
  'diskIo',
];

describe('workload column manual sizing', () => {
  describe('getWorkloadVisibleColumnsForLayout', () => {
    it('drops wide-only columns on a compact table by default', () => {
      const visible = getWorkloadVisibleColumnsForLayout(
        columnsFor(SELECTED_COLUMN_IDS),
        'compact',
      ).map((column) => column.id);

      expect(visible).not.toContain('netIo');
      expect(visible).not.toContain('diskIo');
      expect(visible).not.toContain('tags');
      expect(visible).toContain('cpu');
    });

    it('keeps every selected column once the operator has pinned a width', () => {
      const visible = getWorkloadVisibleColumnsForLayout(
        columnsFor(SELECTED_COLUMN_IDS),
        'compact',
        { manualSizing: true },
      ).map((column) => column.id);

      expect(visible).toEqual(SELECTED_COLUMN_IDS);
    });

    it('still drops wide-only columns on touch layouts even with widths persisted', () => {
      for (const layoutMode of ['narrow', 'phone', 'mobile'] as const) {
        const visible = getWorkloadVisibleColumnsForLayout(
          columnsFor(SELECTED_COLUMN_IDS),
          layoutMode,
          { manualSizing: true },
        ).map((column) => column.id);

        expect(visible, layoutMode).not.toContain('netIo');
      }
    });

    it('is unchanged when manualSizing is absent or false', () => {
      const base = getWorkloadVisibleColumnsForLayout(columnsFor(SELECTED_COLUMN_IDS), 'compact');
      const explicitlyOff = getWorkloadVisibleColumnsForLayout(
        columnsFor(SELECTED_COLUMN_IDS),
        'compact',
        { manualSizing: false },
      );

      expect(explicitlyOff.map((column) => column.id)).toEqual(base.map((column) => column.id));
    });
  });

  describe('getGuestColumnStyle', () => {
    it('leaves the responsive weights untouched when nothing is pinned', () => {
      const withoutOverrides = getGuestColumnStyle('name', false, 'compact', SELECTED_COLUMN_IDS);
      const withEmptyOverrides = getGuestColumnStyle(
        'name',
        false,
        'compact',
        SELECTED_COLUMN_IDS,
        {},
      );

      expect(withEmptyOverrides).toEqual(withoutOverrides);
      expect(String(withoutOverrides?.width)).toContain('%');
    });

    it('pins width, min-width and max-width together so table-fixed cannot redistribute', () => {
      const style = getGuestColumnStyle('name', false, 'compact', SELECTED_COLUMN_IDS, {
        name: 240,
      });

      expect(style).toMatchObject({
        width: '240px',
        'min-width': '240px',
        'max-width': '240px',
      });
    });

    it('only overrides the pinned column', () => {
      const overrides = { name: 240 };
      const cpu = getGuestColumnStyle('cpu', false, 'compact', SELECTED_COLUMN_IDS, overrides);

      expect(String(cpu?.width)).toContain('%');
    });

    it('feeds the same pinned width to the colgroup', () => {
      expect(
        getGuestColumnWidthStyle('netIo', false, 'compact', SELECTED_COLUMN_IDS, { netIo: 210 }),
      ).toEqual({ width: '210px' });
    });
  });
});
