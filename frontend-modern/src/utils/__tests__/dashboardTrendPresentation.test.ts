import { describe, expect, it } from 'vitest';
import {
  getDashboardTrendColor,
  getDashboardTrendErrorState,
} from '@/utils/dashboardTrendPresentation';

describe('dashboardTrendPresentation', () => {
  it('returns stable dashboard trend colors by index', () => {
    expect(getDashboardTrendColor(0)).toBe('#3b82f6');
    expect(getDashboardTrendColor(1)).toBe('#8b5cf6');
    expect(getDashboardTrendColor(7)).toBe('#ef4444');
  });

  it('wraps the palette when the index exceeds the set length', () => {
    expect(getDashboardTrendColor(8)).toBe('#3b82f6');
    expect(getDashboardTrendColor(9)).toBe('#8b5cf6');
  });

  it('returns the dashboard trend error copy', () => {
    expect(getDashboardTrendErrorState()).toEqual({
      text: 'Unable to load trends',
    });
  });
});
