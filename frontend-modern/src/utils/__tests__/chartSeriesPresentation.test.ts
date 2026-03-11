import { describe, expect, it } from 'vitest';
import { getChartSeriesColor } from '@/utils/chartSeriesPresentation';

describe('chartSeriesPresentation', () => {
  it('returns stable chart series colors by index', () => {
    expect(getChartSeriesColor(0)).toBe('#3b82f6');
    expect(getChartSeriesColor(1)).toBe('#8b5cf6');
    expect(getChartSeriesColor(7)).toBe('#ef4444');
  });

  it('wraps the palette when the index exceeds the set length', () => {
    expect(getChartSeriesColor(8)).toBe('#3b82f6');
    expect(getChartSeriesColor(9)).toBe('#8b5cf6');
  });
});
