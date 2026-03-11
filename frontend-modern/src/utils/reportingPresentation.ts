export interface ReportingRangeOption {
  value: '24h' | '7d' | '30d';
  label: string;
}

export const REPORTING_RANGE_OPTIONS: readonly ReportingRangeOption[] = [
  { value: '24h', label: 'Last 24 Hours' },
  { value: '7d', label: 'Last 7 Days' },
  { value: '30d', label: 'Last 30 Days' },
];
