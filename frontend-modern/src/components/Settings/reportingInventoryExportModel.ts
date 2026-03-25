export interface ReportingInventoryExportRequestDefinition {
  filename: string;
  request: {
    url: string;
  };
}

export interface ReportingInventoryExportColumnDefinition {
  key: string;
  label: string;
  description: string;
}

export interface ReportingInventoryExportDefinition {
  id: string;
  title: string;
  description: string;
  format: 'csv';
  filenamePrefix: string;
  columns: ReportingInventoryExportColumnDefinition[];
}

export function buildVMInventoryExportFilename(now: Date, filenamePrefix = 'vm-inventory'): string {
  const date = now.toISOString().split('T')[0];
  return `${filenamePrefix}-${date}.csv`;
}

export function buildVMInventoryExportDefinitionRequest(): { url: string } {
  return {
    url: '/api/admin/reports/inventory/vms/definition',
  };
}

export function buildVMInventoryExportRequest(
  now: Date,
  definition?: Pick<ReportingInventoryExportDefinition, 'filenamePrefix'> | null,
): ReportingInventoryExportRequestDefinition {
  const params = new URLSearchParams({ format: 'csv' });
  return {
    filename: buildVMInventoryExportFilename(now, definition?.filenamePrefix ?? 'vm-inventory'),
    request: {
      url: `/api/admin/reports/inventory/vms/export?${params.toString()}`,
    },
  };
}

export function parseVMInventoryExportDefinition(
  input: unknown,
): ReportingInventoryExportDefinition {
  if (!input || typeof input !== 'object') {
    throw new Error('Invalid VM inventory export definition payload');
  }

  const candidate = input as Partial<ReportingInventoryExportDefinition>;
  if (
    typeof candidate.id !== 'string' ||
    typeof candidate.title !== 'string' ||
    typeof candidate.description !== 'string' ||
    candidate.format !== 'csv' ||
    typeof candidate.filenamePrefix !== 'string' ||
    !Array.isArray(candidate.columns)
  ) {
    throw new Error('Invalid VM inventory export definition payload');
  }

  const columns = candidate.columns.map((column) => {
    if (
      !column ||
      typeof column !== 'object' ||
      typeof column.key !== 'string' ||
      typeof column.label !== 'string' ||
      typeof column.description !== 'string'
    ) {
      throw new Error('Invalid VM inventory export definition payload');
    }

    return {
      key: column.key,
      label: column.label,
      description: column.description,
    };
  });

  return {
    id: candidate.id,
    title: candidate.title,
    description: candidate.description,
    format: 'csv',
    filenamePrefix: candidate.filenamePrefix,
    columns,
  };
}
