import { describe, expect, it } from 'vitest';
import {
  getThresholdSliderFillClass,
  getThresholdSliderTextClass,
} from '@/utils/thresholdSliderPresentation';

describe('thresholdSliderPresentation', () => {
  it('returns canonical text classes for threshold slider types', () => {
    expect(getThresholdSliderTextClass('cpu')).toBe('text-blue-700 dark:text-blue-300');
    expect(getThresholdSliderTextClass('memory')).toBe('text-green-700 dark:text-green-300');
    expect(getThresholdSliderTextClass('disk')).toBe('text-amber-700 dark:text-amber-300');
    expect(getThresholdSliderTextClass('temperature')).toBe('text-rose-700 dark:text-rose-300');
  });

  it('returns canonical fill classes for threshold slider types', () => {
    expect(getThresholdSliderFillClass('cpu')).toBe('bg-blue-500');
    expect(getThresholdSliderFillClass('memory')).toBe('bg-green-500');
    expect(getThresholdSliderFillClass('disk')).toBe('bg-amber-500');
    expect(getThresholdSliderFillClass('temperature')).toBe('bg-rose-500');
  });
});
