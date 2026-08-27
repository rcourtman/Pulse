import { FilterBar, filterChipStatusDot, type FilterDef } from '@/components/shared/FilterBar';
import {
  PLATFORM_ESTATE_COUNTS_STORAGE_KEY,
  deserializePlatformEstateCountsVisibility,
} from '@/features/platformPage/platformEstateOverviewModel';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { STORAGE_KEYS } from '@/utils/localStorage';
import {
  ALERT_HISTORY_ALL_TIME_FILTER_LABEL,
  getAlertHistorySearchPlaceholder,
} from '@/utils/alertOverviewPresentation';
import type { AlertHistoryRange, AlertSeverityFilter } from './alertHistoryModel';
import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertHistoryFiltersCardProps {
  state: AlertHistoryState;
  isMobile: boolean;
}

export function AlertHistoryFiltersCard(props: AlertHistoryFiltersCardProps) {
  // Same estate-wide preference that gates the platform tables' chip counts,
  // so hiding inventory totals quiets this facet too.
  const [countsVisible] = usePersistentSignal(PLATFORM_ESTATE_COUNTS_STORAGE_KEY, true, {
    deserialize: deserializePlatformEstateCountsVisibility,
  });
  const severityCount = (value: AlertSeverityFilter): number | undefined =>
    countsVisible() ? props.state.countForSeverity(value) : undefined;
  const buildFilters = (): FilterDef[] => [
    {
      id: 'alert-period',
      label: 'Period',
      group: 'scope',
      inline: true,
      value: props.state.timeFilter,
      setValue: (value: string) => props.state.setTimeFilter(value as AlertHistoryRange),
      defaultValue: '7d',
      options: () => [
        { value: '24h', label: 'Last 24h' },
        { value: '7d', label: 'Last 7d' },
        { value: '30d', label: 'Last 30d' },
        { value: 'all', label: ALERT_HISTORY_ALL_TIME_FILTER_LABEL },
      ],
    },
    {
      id: 'alert-severity',
      label: 'Severity',
      group: 'status',
      inline: true,
      value: props.state.severityFilter,
      setValue: (value: string) => props.state.setSeverityFilter(value as AlertSeverityFilter),
      defaultValue: 'all',
      options: () => [
        { value: 'all', label: 'All', count: severityCount('all') },
        {
          value: 'critical',
          label: 'Critical',
          leading: filterChipStatusDot('bg-red-500'),
          tone: 'danger',
          count: severityCount('critical'),
        },
        {
          value: 'warning',
          label: 'Warning',
          leading: filterChipStatusDot('bg-amber-500'),
          tone: 'warning',
          count: severityCount('warning'),
        },
        {
          value: 'info',
          label: 'Info',
          leading: filterChipStatusDot('bg-blue-500'),
          count: severityCount('info'),
        },
      ],
    },
  ];

  return (
    <FilterBar
      role="group"
      ariaLabel="Alert history filters"
      isMobile={() => props.isMobile}
      search={{
        value: props.state.searchTerm,
        setValue: props.state.setSearchTerm,
        placeholder: getAlertHistorySearchPlaceholder(),
        historyKey: STORAGE_KEYS.ALERTS_SEARCH_HISTORY,
        clearOnEscape: true,
      }}
      filters={buildFilters()}
      onClearAll={props.state.clearFilters}
      showClearAll={() => props.state.activeFilterCount() > 0}
    />
  );
}
