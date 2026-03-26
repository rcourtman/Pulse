import { describe, expect, it } from 'vitest';
import {
  buildVMInventoryExportFilename,
  buildVMInventoryExportRequest,
  parseVMInventoryExportDefinition,
} from '../reportingInventoryExportModel';

describe('reporting inventory export model', () => {
  it('builds a VM inventory export filename', () => {
    const now = new Date('2026-03-20T12:34:56.000Z');
    expect(buildVMInventoryExportFilename(now, 'vm-inventory', 'utc_yyyymmdd')).toBe('vm-inventory-20260320.csv');
  });

  it('builds the canonical VM inventory export request', () => {
    const now = new Date('2026-03-20T12:34:56.000Z');
    const request = buildVMInventoryExportRequest(now, {
      exportEndpoint: '/api/admin/reports/inventory/vms/export',
      filenamePrefix: 'vm-inventory',
      filenameDateStyle: 'utc_yyyymmdd',
    });

    expect(request.filename).toBe('vm-inventory-20260320.csv');
    expect(request.request.url).toBe('/api/admin/reports/inventory/vms/export?format=csv');
  });

  it('parses the canonical VM inventory export definition payload', () => {
    const definition = parseVMInventoryExportDefinition({
      id: 'vm_inventory',
      title: 'VM Inventory Export',
      description: 'Current-state VM inventory',
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

    expect(definition.id).toBe('vm_inventory');
    expect(definition.exportEndpoint).toBe('/api/admin/reports/inventory/vms/export');
    expect(definition.columns[0]).toEqual({
      key: 'pool',
      label: 'Pool',
      description: 'Canonical Proxmox pool membership.',
    });
  });

  it('rejects inventory definitions without declared columns', () => {
    expect(() =>
      parseVMInventoryExportDefinition({
        id: 'vm_inventory',
        title: 'VM Inventory Export',
        description: 'Current-state VM inventory',
        format: 'csv',
        exportEndpoint: '/api/admin/reports/inventory/vms/export',
        filenamePrefix: 'vm-inventory',
        filenameDateStyle: 'utc_yyyymmdd',
        columns: [],
      }),
    ).toThrow('Invalid VM inventory export definition payload');
  });
});
