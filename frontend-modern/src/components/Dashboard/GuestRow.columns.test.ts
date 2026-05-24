import { describe, expect, it } from 'vitest';

import { GUEST_COLUMNS, getGuestColumnWidth, getGuestTableWidthPx } from './GuestRow';

describe('guest table column sizing', () => {
  it('keeps the desktop table width pinned to the visible column widths', () => {
    const defaultVisibleColumns = GUEST_COLUMNS.filter((column) => column.id !== 'os' && column.id !== 'ip');

    expect(getGuestTableWidthPx(defaultVisibleColumns, false)).toBe(1178);
  });

  it('uses compact metric columns on mobile while preserving the mobile minimum width', () => {
    const columns = GUEST_COLUMNS.filter((column) => ['name', 'cpu', 'memory', 'disk'].includes(column.id));

    expect(getGuestColumnWidth(GUEST_COLUMNS.find((column) => column.id === 'cpu')!, true)).toBe('60px');
    expect(getGuestTableWidthPx(columns, true)).toBe(800);
  });

  it('keeps sparse desktop tables wide enough for stable layout', () => {
    const columns = GUEST_COLUMNS.filter((column) => ['name', 'type', 'vmid'].includes(column.id));

    expect(getGuestTableWidthPx(columns, false)).toBe(900);
  });
});
