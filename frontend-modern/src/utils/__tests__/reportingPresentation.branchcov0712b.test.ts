import { describe, expect, it } from 'vitest';
import {
  getReportingInventoryExportErrorMessage,
  getReportingInventoryExportSuccessMessage,
  resolveReportingDownloadFilename,
} from '@/utils/reportingPresentation';

describe('getReportingInventoryExportSuccessMessage', () => {
  it('returns the canonical VM inventory export success copy', () => {
    expect(getReportingInventoryExportSuccessMessage()).toBe(
      'VM inventory export generated successfully',
    );
  });
});

describe('getReportingInventoryExportErrorMessage', () => {
  it('returns the canonical VM inventory export error copy', () => {
    expect(getReportingInventoryExportErrorMessage()).toBe(
      'Failed to generate VM inventory export',
    );
  });
});

describe('resolveReportingDownloadFilename', () => {
  type ContentDisposition = Parameters<typeof resolveReportingDownloadFilename>[0];
  const fallback = 'fallback-report.csv';

  it('returns the fallback when content-disposition is null', () => {
    expect(resolveReportingDownloadFilename(null, fallback)).toBe(fallback);
  });

  it('returns the fallback when content-disposition is not a string (defensive typeof guard)', () => {
    expect(
      resolveReportingDownloadFilename(undefined as unknown as ContentDisposition, fallback),
    ).toBe(fallback);
    expect(resolveReportingDownloadFilename(42 as unknown as ContentDisposition, fallback)).toBe(
      fallback,
    );
  });

  it('returns the fallback when content-disposition is an empty string', () => {
    expect(resolveReportingDownloadFilename('', fallback)).toBe(fallback);
  });

  it('returns the fallback when content-disposition is only whitespace', () => {
    expect(resolveReportingDownloadFilename('   \t  ', fallback)).toBe(fallback);
  });

  it('decodes RFC 5987 encoded filenames (decodeURIComponent success path)', () => {
    expect(
      resolveReportingDownloadFilename(
        "attachment; filename*=UTF-8''fleet%20report%20march.csv",
        fallback,
      ),
    ).toBe('fleet report march.csv');
  });

  it('prefers the RFC 5987 encoded filename over a quoted filename', () => {
    expect(
      resolveReportingDownloadFilename(
        'attachment; filename="quoted.pdf"; filename*=UTF-8\'\'encoded%20report.csv',
        fallback,
      ),
    ).toBe('encoded report.csv');
  });

  it('returns the raw encoded token when it cannot be URI-decoded (decodeRFC5987Value catch path)', () => {
    // '%' is not a valid URI escape, so decodeURIComponent throws and the helper returns the original token.
    expect(resolveReportingDownloadFilename("attachment; filename*=UTF-8''%", fallback)).toBe('%');
  });

  it('returns the quoted filename when a quoted directive is present', () => {
    expect(
      resolveReportingDownloadFilename('attachment; filename="canonical-report.pdf"', fallback),
    ).toBe('canonical-report.pdf');
  });

  it('falls back to the plain (unquoted) filename when no quoted directive is present', () => {
    expect(
      resolveReportingDownloadFilename('attachment; filename=plain-report.csv', fallback),
    ).toBe('plain-report.csv');
  });

  it('trims surrounding whitespace from the plain filename', () => {
    expect(
      resolveReportingDownloadFilename(
        'attachment; filename=plain-report.csv ; size=1234',
        fallback,
      ),
    ).toBe('plain-report.csv');
  });

  it('returns the fallback when no filename directive is present at all', () => {
    expect(resolveReportingDownloadFilename('attachment; size=1234', fallback)).toBe(fallback);
  });
});
