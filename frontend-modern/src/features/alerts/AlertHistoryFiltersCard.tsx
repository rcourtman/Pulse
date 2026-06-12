import { FilterBar, filterChipStatusDot, type FilterDef } from '@/components/shared/FilterBar';
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
        { value: 'all', label: 'All' },
        {
          value: 'critical',
          label: 'Critical',
          leading: filterChipStatusDot('bg-red-500'),
          tone: 'danger',
        },
        {
          value: 'warning',
          label: 'Warning',
          leading: filterChipStatusDot('bg-amber-500'),
          tone: 'warning',
        },
      ],
    },
  ];

  return (
    <FilterBar
      role="group"
      ariaLabel="Alert history filters"
      isMobile={() => props.isMobile}
      savedViewsKey="alerts-history"
      search={{
        value: props.state.searchTerm,
        setValue: props.state.setSearchTerm,
        placeholder: getAlertHistorySearchPlaceholder(),
        historyKey: STORAGE_KEYS.ALERTS_SEARCH_HISTORY,
        clearOnEscape: true,
      }}
      filters={buildFilters()}
    />
  );
}
