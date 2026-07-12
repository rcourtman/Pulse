/**
 * Branch-coverage tests for the still-uncovered named helpers in
 * reportingCatalogModel:
 *   parseReportingFormatDefinition, parseReportingRangeDefinition,
 *   parseReportingPerformanceReportDefinition, parseReportingCatalog,
 *   parseReportingLockedStateDefinition, parseReportingGuidanceDefinition.
 *
 * None of those six helpers are exported from the module, so every branch is
 * driven through the single exported entry point `parseReportingCatalog` —
 * the only public caller — exactly as the sibling `reportingCatalogModel.test.ts`
 * reaches them. Each block targets one helper and drives both arms (happy path
 * + each defensive throw) of every conditional / typeof / Number.isFinite /
 * Array.isArray guard, asserting concrete emitted shapes (never truthiness-only).
 *
 * Import path mirrors the sibling test (alias `@/...`).
 */
import { describe, expect, it } from 'vitest';
import {
  parseReportingCatalog,
  type ReportingCatalog,
} from '@/components/Settings/reportingCatalogModel';

type CatalogInput = Parameters<typeof parseReportingCatalog>[0];

const INVALID = 'Invalid reporting catalog payload';
const expectInvalid = (input: CatalogInput) =>
  expect(() => parseReportingCatalog(input)).toThrow(INVALID);

// ---- Fixtures ---------------------------------------------------------------
// Plain (non-`as const`) builders so each test can spread + override a single
// field to drive exactly one branch under test.

const baseRange = () => ({
  key: '24h',
  label: 'Last 24 Hours',
  description: 'Daily review',
  windowHours: 24,
});

const basePerformanceReport = () => ({
  id: 'performance_reports',
  title: 'Performance Reports',
  description: 'Historical performance reporting',
  singleResourceEndpoint: '/api/admin/reports/generate',
  multiResourceEndpoint: '/api/admin/reports/generate-multi',
  singleFilenamePrefix: 'report',
  singleFilenameSubject: 'resource_id',
  multiFilenamePrefix: 'fleet-report',
  filenameDateStyle: 'utc_yyyymmdd',
  formats: [
    { value: 'pdf', label: 'PDF Report' },
    { value: 'csv', label: 'CSV Data' },
  ],
  defaultFormat: 'pdf',
  ranges: [baseRange()],
  defaultRange: '24h',
  multiResourceMax: 50,
  supportsMetricFilter: true,
  supportsCustomTitle: true,
});

const baseVmInventoryExport = () => ({
  id: 'vm_inventory',
  title: 'VM Inventory Export',
  description: 'Current-state inventory',
  format: 'csv',
  exportEndpoint: '/api/admin/reports/inventory/vms/export',
  filenamePrefix: 'vm-inventory',
  filenameDateStyle: 'utc_yyyymmdd',
  columns: [
    {
      key: 'pool',
      label: 'Pool',
      description: 'Canonical Proxmox pool membership.',
    },
  ],
});

const baseCatalog = () => ({
  id: 'advanced_reporting',
  title: 'Detailed Reporting',
  description: 'Canonical reporting surfaces',
  lockedState: {
    title: 'Advanced Reporting',
    description: 'Canonical locked reporting teaser',
  },
  guidance: {
    title: 'Advanced Insights',
    description: 'Catalog-owned reporting guidance',
  },
  performanceReport: basePerformanceReport(),
  vmInventoryExport: null,
});

// ---- parseReportingCatalog --------------------------------------------------

describe('parseReportingCatalog (entry-point branches)', () => {
  it('throws on a non-object input (null / undefined / primitive arms)', () => {
    expectInvalid(null);
    expectInvalid(undefined);
    expectInvalid('not-an-object');
    expectInvalid(42);
    expectInvalid(true);
  });

  it('throws when id is not a string', () => {
    expectInvalid({ ...baseCatalog(), id: 123 });
  });

  it('throws when title is not a string', () => {
    expectInvalid({ ...baseCatalog(), title: null });
  });

  it('throws when description is not a string', () => {
    expectInvalid({ ...baseCatalog(), description: 7 });
  });

  it('round-trips the full catalog shape when vmInventoryExport is null (null arm)', () => {
    const catalog = parseReportingCatalog(baseCatalog()) as ReportingCatalog;
    expect(catalog).toStrictEqual({
      id: 'advanced_reporting',
      title: 'Detailed Reporting',
      description: 'Canonical reporting surfaces',
      lockedState: {
        title: 'Advanced Reporting',
        description: 'Canonical locked reporting teaser',
      },
      guidance: {
        title: 'Advanced Insights',
        description: 'Catalog-owned reporting guidance',
      },
      performanceReport: basePerformanceReport(),
      vmInventoryExport: null,
    });
    // Explicit null-arm assertion.
    expect(catalog.vmInventoryExport).toBeNull();
  });

  it('parses the inventory export when vmInventoryExport is present (non-null arm)', () => {
    const catalog = parseReportingCatalog({
      ...baseCatalog(),
      vmInventoryExport: baseVmInventoryExport(),
    }) as ReportingCatalog;
    expect(catalog.vmInventoryExport).not.toBeNull();
    expect(catalog.vmInventoryExport).toStrictEqual({
      id: 'vm_inventory',
      title: 'VM Inventory Export',
      description: 'Current-state inventory',
      format: 'csv',
      exportEndpoint: '/api/admin/reports/inventory/vms/export',
      filenamePrefix: 'vm-inventory',
      filenameDateStyle: 'utc_yyyymmdd',
      columns: [
        {
          key: 'pool',
          label: 'Pool',
          description: 'Canonical Proxmox pool membership.',
        },
      ],
    });
  });
});

// ---- parseReportingLockedStateDefinition -----------------------------------

describe('parseReportingLockedStateDefinition (via catalog.lockedState)', () => {
  it('throws when lockedState is not an object (null / primitive arms)', () => {
    expectInvalid({ ...baseCatalog(), lockedState: null });
    expectInvalid({ ...baseCatalog(), lockedState: 'nope' });
    expectInvalid({ ...baseCatalog(), lockedState: 5 });
  });

  it('throws when lockedState.title is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      lockedState: { title: 9, description: 'ok' },
    });
  });

  it('throws when lockedState.description is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      lockedState: { title: 'Advanced Reporting', description: null },
    });
  });

  it('round-trips a fully-populated locked state', () => {
    const catalog = parseReportingCatalog({
      ...baseCatalog(),
      lockedState: { title: 'Locked', description: 'Teaser copy' },
    }) as ReportingCatalog;
    expect(catalog.lockedState).toStrictEqual({
      title: 'Locked',
      description: 'Teaser copy',
    });
  });
});

// ---- parseReportingGuidanceDefinition --------------------------------------

describe('parseReportingGuidanceDefinition (via catalog.guidance)', () => {
  it('throws when guidance is not an object (null / primitive arms)', () => {
    expectInvalid({ ...baseCatalog(), guidance: null });
    expectInvalid({ ...baseCatalog(), guidance: 'nope' });
    expectInvalid({ ...baseCatalog(), guidance: 5 });
  });

  it('throws when guidance.title is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      guidance: { title: 9, description: 'ok' },
    });
  });

  it('throws when guidance.description is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      guidance: { title: 'Advanced Insights', description: null },
    });
  });

  it('round-trips a fully-populated guidance block', () => {
    const catalog = parseReportingCatalog({
      ...baseCatalog(),
      guidance: { title: 'Guided', description: 'Helpful copy' },
    }) as ReportingCatalog;
    expect(catalog.guidance).toStrictEqual({
      title: 'Guided',
      description: 'Helpful copy',
    });
  });
});

// ---- parseReportingFormatDefinition ----------------------------------------

describe('parseReportingFormatDefinition (via performanceReport.formats[])', () => {
  it('throws when a format entry is not an object (null / primitive arms)', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        formats: [null, { value: 'csv', label: 'CSV Data' }],
      },
    });
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        formats: ['pdf-only' as unknown as object, { value: 'csv', label: 'CSV Data' }],
      },
    });
  });

  it('throws when a format value is neither "pdf" nor "csv"', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        formats: [
          { value: 'xlsx', label: 'XLSX' },
          { value: 'csv', label: 'CSV Data' },
        ],
        defaultFormat: 'csv',
      },
    });
  });

  it('throws when a format label is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        formats: [
          { value: 'pdf', label: 42 },
          { value: 'csv', label: 'CSV Data' },
        ],
      },
    });
  });

  it('round-trips both pdf and csv format entries unchanged', () => {
    const catalog = parseReportingCatalog({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        formats: [
          { value: 'pdf', label: 'PDF Report' },
          { value: 'csv', label: 'CSV Data' },
        ],
      },
    }) as ReportingCatalog;
    expect(catalog.performanceReport.formats).toStrictEqual([
      { value: 'pdf', label: 'PDF Report' },
      { value: 'csv', label: 'CSV Data' },
    ]);
  });
});

// ---- parseReportingRangeDefinition -----------------------------------------

describe('parseReportingRangeDefinition (via performanceReport.ranges[])', () => {
  it('throws when a range entry is not an object (null / primitive arms)', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: [null],
      },
    });
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: ['24h' as unknown as object],
      },
    });
  });

  it('throws when key is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: [{ ...baseRange(), key: 24 }],
      },
    });
  });

  it('throws when label is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: [{ ...baseRange(), label: null }],
      },
    });
  });

  it('throws when description is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: [{ ...baseRange(), description: 1 }],
      },
    });
  });

  it('throws when windowHours is not a number', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: [{ ...baseRange(), windowHours: '24' }],
      },
    });
  });

  it('throws when windowHours is not finite (NaN / Infinity)', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: [{ ...baseRange(), windowHours: Number.NaN }],
      },
    });
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: [{ ...baseRange(), windowHours: Number.POSITIVE_INFINITY }],
      },
    });
  });

  it('throws when windowHours is <= 0 (zero and negative arms)', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: [{ ...baseRange(), windowHours: 0 }],
      },
    });
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: [{ ...baseRange(), windowHours: -12 }],
      },
    });
  });

  it('round-trips a fully-populated range unchanged', () => {
    const catalog = parseReportingCatalog({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        ranges: [
          {
            key: '7d',
            label: 'Last 7 Days',
            description: 'Weekly trend',
            windowHours: 168,
          },
        ],
        defaultRange: '7d',
      },
    }) as ReportingCatalog;
    expect(catalog.performanceReport.ranges).toStrictEqual([
      {
        key: '7d',
        label: 'Last 7 Days',
        description: 'Weekly trend',
        windowHours: 168,
      },
    ]);
  });
});

// ---- parseReportingPerformanceReportDefinition -----------------------------

describe('parseReportingPerformanceReportDefinition (via catalog.performanceReport)', () => {
  it('throws when performanceReport is not an object (null / primitive arms)', () => {
    expectInvalid({ ...baseCatalog(), performanceReport: null });
    expectInvalid({ ...baseCatalog(), performanceReport: 'nope' });
  });

  it('throws when id is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), id: 1 },
    });
  });

  it('throws when title is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), title: null },
    });
  });

  it('throws when description is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), description: 2 },
    });
  });

  it('throws when singleResourceEndpoint is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), singleResourceEndpoint: 3 },
    });
  });

  it('throws when multiResourceEndpoint is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), multiResourceEndpoint: null },
    });
  });

  it('throws when singleFilenamePrefix is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), singleFilenamePrefix: 4 },
    });
  });

  it('throws when singleFilenameSubject is not "resource_id"', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        singleFilenameSubject: 'resource_name',
      },
    });
  });

  it('throws when multiFilenamePrefix is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), multiFilenamePrefix: null },
    });
  });

  it('throws when filenameDateStyle is not "utc_yyyymmdd"', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        filenameDateStyle: 'local_yyyy_mm_dd',
      },
    });
  });

  it('throws when formats is not an array', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), formats: null },
    });
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), formats: 'pdf,csv' },
    });
  });

  it('throws when defaultFormat is neither "pdf" nor "csv"', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), defaultFormat: 'xlsx' },
    });
  });

  it('throws when ranges is not an array', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), ranges: null },
    });
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), ranges: '24h' },
    });
  });

  it('throws when defaultRange is not a string', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), defaultRange: 24 },
    });
  });

  it('throws when multiResourceMax is not a number', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), multiResourceMax: '50' },
    });
  });

  it('throws when multiResourceMax is not finite (NaN / Infinity)', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), multiResourceMax: Number.NaN },
    });
    expectInvalid({
      ...baseCatalog(),
      performanceReport: {
        ...basePerformanceReport(),
        multiResourceMax: Number.POSITIVE_INFINITY,
      },
    });
  });

  it('throws when multiResourceMax is <= 0 (zero and negative arms)', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), multiResourceMax: 0 },
    });
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), multiResourceMax: -5 },
    });
  });

  it('throws when supportsMetricFilter is not a boolean', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), supportsMetricFilter: 'yes' },
    });
  });

  it('throws when supportsCustomTitle is not a boolean', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), supportsCustomTitle: 1 },
    });
  });

  it('throws when the formats array is empty (length === 0 arm)', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), formats: [] },
    });
  });

  it('throws when the ranges array is empty (length === 0 arm)', () => {
    expectInvalid({
      ...baseCatalog(),
      performanceReport: { ...basePerformanceReport(), ranges: [] },
    });
  });

  it('round-trips a fully-populated performance report unchanged', () => {
    const expected = basePerformanceReport();
    const catalog = parseReportingCatalog(baseCatalog()) as ReportingCatalog;
    expect(catalog.performanceReport).toStrictEqual(expected);
    // Spot-check the parsed literal unions.
    expect(catalog.performanceReport.defaultFormat).toBe('pdf');
    expect(catalog.performanceReport.defaultRange).toBe('24h');
    expect(catalog.performanceReport.singleFilenameSubject).toBe('resource_id');
    expect(catalog.performanceReport.filenameDateStyle).toBe('utc_yyyymmdd');
  });
});
