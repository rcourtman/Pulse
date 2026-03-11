import { describe, expect, it } from 'vitest';
import {
  getThresholdSliderFillClass,
  getThresholdSliderTextClass,
} from '@/utils/thresholdSliderPresentation';

describe('thresholdSliderPresentation', () => {
  it('returns canonical text classes for threshold slider types', () => {
    expect(getThresholdSliderTextClass('cpu')).toBe('text-blue-500');
    expect(getThresholdSliderTextClass('memory')).toBe('text-green-500');
    expect(getThresholdSliderTextClass('disk')).toBe('text-amber-500');
    expect(getThresholdSliderTextClass('temperature')).toBe('text-rose-500');
  });

  it('returns canonical fill classes for threshold slider types', () => {
    expect(getThresholdSliderFillClass('cpu')).toBe('bg-blue-500');
    expect(getThresholdSliderFillClass('memory')).toBe('bg-green-500');
    expect(getThresholdSliderFillClass('disk')).toBe('bg-amber-500');
    expect(getThresholdSliderFillClass('temperature')).toBe('bg-rose-500');
  });
});
