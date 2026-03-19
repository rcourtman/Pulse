import { describe, expect, it } from 'vitest';

import {
  formatConfidenceLabel,
  formatConfidencePercentage,
} from '@/utils/confidencePresentation';

describe('confidencePresentation', () => {
  it('formats shared confidence percentages', () => {
    expect(formatConfidencePercentage(0.875)).toBe('88%');
    expect(formatConfidencePercentage(0)).toBe('0%');
  });

  it('formats mixed confidence labels', () => {
    expect(formatConfidenceLabel(0.875)).toBe('88%');
    expect(formatConfidenceLabel('high')).toBe('High');
    expect(formatConfidenceLabel(' medium ')).toBe('Medium');
    expect(formatConfidenceLabel(undefined)).toBe('');
  });
});
