import { describe, expect, it } from 'vitest';
import {
  buildReportingCatalogRequest,
  parseReportingCatalog,
} from '../reportingCatalogModel';

const baseCatalogPayload = {
  id: 'advanced_reporting',
  title: 'Detailed Reporting',
  description: 'Canonical reporting surfaces',
  lockedState: {
    title: 'Advanced Reporting (Pro)',
    description: 'Canonical locked reporting teaser',
  },
  guidance: {
    title: 'Advanced Insights',
    description: 'Catalog-owned reporting guidance',
  },
  performanceReport: {
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
    ranges: [
      {
        key: '24h',
        label: 'Last 24 Hours',
        description: 'Daily review',
        windowHours: 24,
      },
    ],
    defaultRange: '24h',
    multiResourceMax: 50,
    supportsMetricFilter: true,
    supportsCustomTitle: true,
  },
  vmInventoryExport: {
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
  },
} as const;

describe('reporting catalog model', () => {
  it('builds the canonical reporting catalog request', () => {
    expect(buildReportingCatalogRequest()).toEqual({
      url: '/api/admin/reports/catalog',
    });
  });

  it('parses the canonical reporting catalog payload', () => {
    const catalog = parseReportingCatalog(baseCatalogPayload);

    expect(catalog.lockedState.title).toBe('Advanced Reporting (Pro)');
    expect(catalog.guidance.title).toBe('Advanced Insights');
    expect(catalog.performanceReport.defaultFormat).toBe('pdf');
    expect(catalog.performanceReport.ranges[0].windowHours).toBe(24);
    expect(catalog.vmInventoryExport.exportEndpoint).toBe(
      '/api/admin/reports/inventory/vms/export',
    );
  });

  it('rejects a catalog whose default format is not in the supported formats', () => {
    expect(() =>
      parseReportingCatalog({
        ...baseCatalogPayload,
        performanceReport: {
          ...baseCatalogPayload.performanceReport,
          defaultFormat: 'pdf',
          formats: [{ value: 'csv', label: 'CSV Data' }],
        },
      }),
    ).toThrow('Invalid reporting catalog payload');
  });

  it('rejects a catalog whose default range is not in the supported ranges', () => {
    expect(() =>
      parseReportingCatalog({
        ...baseCatalogPayload,
        performanceReport: {
          ...baseCatalogPayload.performanceReport,
          defaultRange: '7d',
          ranges: [
            {
              key: '24h',
              label: 'Last 24 Hours',
              description: 'Daily review',
              windowHours: 24,
            },
          ],
        },
      }),
    ).toThrow('Invalid reporting catalog payload');
  });

  it('rejects a catalog whose filename date style is unsupported', () => {
    expect(() =>
      parseReportingCatalog({
        ...baseCatalogPayload,
        performanceReport: {
          ...baseCatalogPayload.performanceReport,
          filenameDateStyle: 'local_yyyy_mm_dd',
        },
      }),
    ).toThrow('Invalid reporting catalog payload');
  });

  it('rejects a catalog whose single filename subject is unsupported', () => {
    expect(() =>
      parseReportingCatalog({
        ...baseCatalogPayload,
        performanceReport: {
          ...baseCatalogPayload.performanceReport,
          singleFilenameSubject: 'resource_name',
        },
      }),
    ).toThrow('Invalid reporting catalog payload');
  });

  it('rejects a catalog whose locked-state teaser is incomplete', () => {
    expect(() =>
      parseReportingCatalog({
        ...baseCatalogPayload,
        lockedState: {
          title: 'Advanced Reporting (Pro)',
        },
      }),
    ).toThrow('Invalid reporting catalog payload');
  });

  it('rejects a catalog whose guidance copy is incomplete', () => {
    expect(() =>
      parseReportingCatalog({
        ...baseCatalogPayload,
        guidance: {
          title: 'Advanced Insights',
        },
      }),
    ).toThrow('Invalid reporting catalog payload');
  });
});
