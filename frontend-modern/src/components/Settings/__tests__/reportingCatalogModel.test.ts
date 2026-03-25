import { describe, expect, it } from 'vitest';
import {
  buildReportingCatalogRequest,
  parseReportingCatalog,
} from '../reportingCatalogModel';

describe('reporting catalog model', () => {
  it('builds the canonical reporting catalog request', () => {
    expect(buildReportingCatalogRequest()).toEqual({
      url: '/api/admin/reports/catalog',
    });
  });

  it('parses the canonical reporting catalog payload', () => {
    const catalog = parseReportingCatalog({
      id: 'advanced_reporting',
      title: 'Detailed Reporting',
      description: 'Canonical reporting surfaces',
      performanceReport: {
        id: 'performance_reports',
        title: 'Performance Reports',
        description: 'Historical performance reporting',
        singleResourceEndpoint: '/api/admin/reports/generate',
        multiResourceEndpoint: '/api/admin/reports/generate-multi',
        singleFilenamePrefix: 'report',
        multiFilenamePrefix: 'fleet-report',
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
        columns: [
          {
            key: 'pool',
            label: 'Pool',
            description: 'Canonical Proxmox pool membership.',
          },
        ],
      },
    });

    expect(catalog.performanceReport.defaultFormat).toBe('pdf');
    expect(catalog.performanceReport.ranges[0].windowHours).toBe(24);
    expect(catalog.vmInventoryExport.exportEndpoint).toBe(
      '/api/admin/reports/inventory/vms/export',
    );
  });
});
