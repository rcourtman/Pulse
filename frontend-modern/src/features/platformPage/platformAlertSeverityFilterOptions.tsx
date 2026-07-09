import { filterChipStatusDot } from '@/components/shared/FilterBar';
import type { PlatformTableFilterOption } from '@/features/platformPage/sharedPlatformPage';

export type PlatformAlertSeverityFilterValue =
  'all' | 'attention' | 'critical' | 'warning' | 'info';

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

const PLATFORM_ALERT_ATTENTION_FILTER_OPTION: PlatformTableFilterOption<PlatformAlertSeverityFilterValue> =
  {
    value: 'attention',
    label: 'Attention',
    tone: 'warning',
    leading: filterChipStatusDot('bg-amber-500'),
  };

export function getPlatformAlertSeverityFilterOptions<
  TFilter extends PlatformAlertSeverityFilterValue,
>(options: { includeAttention?: boolean } = {}): PlatformTableFilterOption<TFilter>[] {
  const resolved = options.includeAttention
    ? [
        PLATFORM_ALERT_SEVERITY_FILTER_OPTIONS[0],
        PLATFORM_ALERT_ATTENTION_FILTER_OPTION,
        ...PLATFORM_ALERT_SEVERITY_FILTER_OPTIONS.slice(1),
      ]
    : PLATFORM_ALERT_SEVERITY_FILTER_OPTIONS;
  return resolved as PlatformTableFilterOption<TFilter>[];
}
