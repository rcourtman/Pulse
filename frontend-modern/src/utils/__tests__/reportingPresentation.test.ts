import { describe, expect, it } from 'vitest';
import {
  getReportingCatalogErrorMessage,
  getReportingGenerateErrorMessage,
  getReportingGenerateSelectionRequiredMessage,
  getReportingGenerateSuccessMessage,
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

  it('returns canonical reporting status copy', () => {
    expect(getReportingGenerateSelectionRequiredMessage()).toBe(
      'Please select at least one resource',
    );
    expect(getReportingCatalogErrorMessage()).toBe('Failed to load reporting surfaces');
    expect(getReportingGenerateSuccessMessage()).toBe('Report generated successfully');
    expect(getReportingGenerateErrorMessage()).toBe('Failed to generate report');
  });
});
