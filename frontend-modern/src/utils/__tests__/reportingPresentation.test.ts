import { describe, expect, it } from 'vitest';
import {
  getReportingCatalogErrorMessage,
  getReportingGenerateErrorMessage,
  getReportingGenerateSelectionRequiredMessage,
  getReportingGenerateSuccessMessage,
  resolveReportingDownloadFilename,
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

  it('prefers backend attachment filenames and falls back locally when needed', () => {
    expect(
      resolveReportingDownloadFilename(
        'attachment; filename="canonical-report.pdf"',
        'fallback-report.pdf',
      ),
    ).toBe('canonical-report.pdf');
    expect(
      resolveReportingDownloadFilename(
        "attachment; filename*=UTF-8''fleet%20report%20march.csv",
        'fallback-report.csv',
      ),
    ).toBe('fleet report march.csv');
    expect(resolveReportingDownloadFilename(null, 'fallback-report.csv')).toBe(
      'fallback-report.csv',
    );
  });
});
