import { describe, expect, it } from 'vitest';
import { REPORTING_RANGE_OPTIONS } from '@/utils/reportingPresentation';

describe('reportingPresentation', () => {
  it('returns canonical reporting range options', () => {
    expect(REPORTING_RANGE_OPTIONS).toEqual([
      { value: '24h', label: 'Last 24 Hours' },
      { value: '7d', label: 'Last 7 Days' },
      { value: '30d', label: 'Last 30 Days' },
    ]);
  });
});
