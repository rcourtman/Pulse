import { describe, expect, it } from 'vitest';
import { getTableSortIndicator } from '@/components/shared/tableSortPresentation';

describe('getTableSortIndicator', () => {
  it('keeps inactive sortable headers visually quiet', () => {
    expect(getTableSortIndicator(false, 'asc')).toBeNull();
    expect(getTableSortIndicator(false, 'desc')).toBeNull();
  });

  it('shows only the active direction', () => {
    expect(getTableSortIndicator(true, 'asc')).toBe('▲');
    expect(getTableSortIndicator(true, 'desc')).toBe('▼');
  });
});
