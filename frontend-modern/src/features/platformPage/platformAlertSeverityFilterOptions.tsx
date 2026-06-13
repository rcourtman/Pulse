import { filterChipStatusDot } from '@/components/shared/FilterBar';
import type { PlatformTableFilterOption } from '@/features/platformPage/sharedPlatformPage';

export type PlatformAlertSeverityFilterValue = 'all' | 'critical' | 'warning' | 'info';

const PLATFORM_ALERT_SEVERITY_FILTER_OPTIONS: PlatformTableFilterOption<PlatformAlertSeverityFilterValue>[] =
  [
    { value: 'all', label: 'All' },
    {
      value: 'critical',
      label: 'Critical',
      tone: 'danger',
      leading: filterChipStatusDot('bg-red-500'),
    },
    {
      value: 'warning',
      label: 'Warning',
      tone: 'warning',
      leading: filterChipStatusDot('bg-amber-500'),
    },
    {
      value: 'info',
      label: 'Info',
      tone: 'success',
      leading: filterChipStatusDot('bg-emerald-500'),
    },
  ];

export function getPlatformAlertSeverityFilterOptions<
  TFilter extends PlatformAlertSeverityFilterValue,
>(): PlatformTableFilterOption<TFilter>[] {
  return PLATFORM_ALERT_SEVERITY_FILTER_OPTIONS as PlatformTableFilterOption<TFilter>[];
}
