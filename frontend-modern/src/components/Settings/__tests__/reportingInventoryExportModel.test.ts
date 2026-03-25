import { describe, expect, it } from 'vitest';
import {
  buildVMInventoryExportFilename,
  buildVMInventoryExportRequest,
} from '../reportingInventoryExportModel';

describe('reporting inventory export model', () => {
  it('builds a VM inventory export filename', () => {
    const now = new Date('2026-03-20T12:34:56.000Z');
    expect(buildVMInventoryExportFilename(now)).toBe('vm-inventory-2026-03-20.csv');
  });

  it('builds the canonical VM inventory export request', () => {
    const now = new Date('2026-03-20T12:34:56.000Z');
    const request = buildVMInventoryExportRequest(now);

    expect(request.filename).toBe('vm-inventory-2026-03-20.csv');
    expect(request.request.url).toBe('/api/admin/reports/inventory/vms/export?format=csv');
  });
});
