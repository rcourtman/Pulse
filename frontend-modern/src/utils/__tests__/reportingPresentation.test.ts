import { describe, expect, it } from 'vitest';
import {
  getReportingToggleButtonClass,
  REPORTING_RANGE_OPTIONS,
} from '@/utils/reportingPresentation';

describe('reportingPresentation', () => {
  it('returns canonical reporting range options', () => {
    expect(REPORTING_RANGE_OPTIONS).toEqual([
      { value: '24h', label: 'Last 24 Hours' },
      { value: '7d', label: 'Last 7 Days' },
      { value: '30d', label: 'Last 30 Days' },
    ]);
  });

  it('returns canonical reporting toggle button classes', () => {
    expect(getReportingToggleButtonClass(true)).toContain('bg-blue-50');
    expect(getReportingToggleButtonClass(false)).toContain('border-border');
  });
});
