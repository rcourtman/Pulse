export interface ReportingRangeOption {
  value: '24h' | '7d' | '30d';
  label: string;
}

export const REPORTING_RANGE_OPTIONS: readonly ReportingRangeOption[] = [
  { value: '24h', label: 'Last 24 Hours' },
  { value: '7d', label: 'Last 7 Days' },
  { value: '30d', label: 'Last 30 Days' },
];

export function getReportingToggleButtonClass(selected: boolean): string {
  return `w-full sm:w-auto min-h-10 sm:min-h-9 rounded-md border px-4 py-2.5 transition-all ${
    selected
      ? 'bg-blue-50 border-blue-500 text-blue-700 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-500'
      : 'border-border text-base-content hover:bg-surface-alt'
  }`;
}
