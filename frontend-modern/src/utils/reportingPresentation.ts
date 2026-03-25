export interface ReportingRangeOption {
  value: '24h' | '7d' | '30d';
  label: string;
}

export const REPORTING_RANGE_OPTIONS: readonly ReportingRangeOption[] = [
  { value: '24h', label: 'Last 24 Hours' },
  { value: '7d', label: 'Last 7 Days' },
  { value: '30d', label: 'Last 30 Days' },
];

export function getReportingGenerateSelectionRequiredMessage(): string {
  return 'Please select at least one resource';
}

export function getReportingGenerateSuccessMessage(): string {
  return 'Report generated successfully';
}

export function getReportingGenerateErrorMessage(): string {
  return 'Failed to generate report';
}

export function getReportingCatalogErrorMessage(): string {
  return 'Failed to load reporting surfaces';
}

export function getReportingInventoryExportSuccessMessage(): string {
  return 'VM inventory export generated successfully';
}

export function getReportingInventoryExportErrorMessage(): string {
  return 'Failed to generate VM inventory export';
}
