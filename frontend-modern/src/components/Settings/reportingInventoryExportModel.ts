import { formatReportingFilenameDate } from '@/utils/reportingPresentation';
import type { ReportingFilenameDateStyle } from '@/components/Settings/reportingCatalogModel';

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
  exportEndpoint: string;
  filenamePrefix: string;
  filenameDateStyle: ReportingFilenameDateStyle;
  columns: ReportingInventoryExportColumnDefinition[];
}

export function buildVMInventoryExportFilename(
  now: Date,
  filenamePrefix: string,
  filenameDateStyle: ReportingFilenameDateStyle,
): string {
  const date = formatReportingFilenameDate(now, filenameDateStyle);
  return `${filenamePrefix}-${date}.csv`;
}

export function buildVMInventoryExportRequest(
  now: Date,
  definition: Pick<
    ReportingInventoryExportDefinition,
    'exportEndpoint' | 'filenamePrefix' | 'filenameDateStyle'
  >,
): ReportingInventoryExportRequestDefinition {
  const params = new URLSearchParams({ format: 'csv' });
  return {
    filename: buildVMInventoryExportFilename(
      now,
      definition.filenamePrefix,
      definition.filenameDateStyle,
    ),
    request: {
      url: `${definition.exportEndpoint}?${params.toString()}`,
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
    typeof candidate.exportEndpoint !== 'string' ||
    typeof candidate.filenamePrefix !== 'string' ||
    candidate.filenameDateStyle !== 'utc_yyyymmdd' ||
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

  if (columns.length === 0) {
    throw new Error('Invalid VM inventory export definition payload');
  }

  return {
    id: candidate.id,
    title: candidate.title,
    description: candidate.description,
    format: 'csv',
    exportEndpoint: candidate.exportEndpoint,
    filenamePrefix: candidate.filenamePrefix,
    filenameDateStyle: candidate.filenameDateStyle,
    columns,
  };
}
