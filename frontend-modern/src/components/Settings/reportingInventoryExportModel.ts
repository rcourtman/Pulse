export interface ReportingInventoryExportRequestDefinition {
  filename: string;
  request: {
    url: string;
  };
}

export function buildVMInventoryExportFilename(now: Date): string {
  const date = now.toISOString().split('T')[0];
  return `vm-inventory-${date}.csv`;
}

export function buildVMInventoryExportRequest(
  now: Date,
): ReportingInventoryExportRequestDefinition {
  const params = new URLSearchParams({ format: 'csv' });
  return {
    filename: buildVMInventoryExportFilename(now),
    request: {
      url: `/api/admin/reports/inventory/vms/export?${params.toString()}`,
    },
  };
}
