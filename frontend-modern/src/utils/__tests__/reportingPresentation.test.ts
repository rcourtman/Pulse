import { describe, expect, it } from 'vitest';
import {
  getReportingCatalogErrorMessage,
  getReportingGenerateErrorMessage,
  getReportingGenerateSelectionRequiredMessage,
  getReportingGenerateSuccessMessage,
} from '@/utils/reportingPresentation';

describe('reportingPresentation', () => {
  it('returns canonical reporting status copy', () => {
    expect(getReportingGenerateSelectionRequiredMessage()).toBe(
      'Please select at least one resource',
    );
    expect(getReportingCatalogErrorMessage()).toBe('Failed to load reporting surfaces');
    expect(getReportingGenerateSuccessMessage()).toBe('Report generated successfully');
    expect(getReportingGenerateErrorMessage()).toBe('Failed to generate report');
  });
});
