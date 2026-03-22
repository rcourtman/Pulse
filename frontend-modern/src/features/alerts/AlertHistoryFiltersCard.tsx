import { Card } from '@/components/shared/Card';
import { LabeledFilterSelect } from '@/components/shared/FilterToolbar';
import { PageControls } from '@/components/shared/PageControls';
import { SearchInput } from '@/components/shared/SearchInput';
import { getAlertHistorySearchPlaceholder } from '@/utils/alertOverviewPresentation';

import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertHistoryFiltersCardProps {
  state: AlertHistoryState;
  isMobile: boolean;
}

export function AlertHistoryFiltersCard(props: AlertHistoryFiltersCardProps) {
  return (
    <Card padding="sm" class="mb-4">
      <PageControls
        search={
          <SearchInput
            value={props.state.searchTerm}
            onChange={props.state.setSearchTerm}
            placeholder={getAlertHistorySearchPlaceholder()}
            class="w-full"
            clearOnEscape
            history={{ storageKey: props.state.STORAGE_KEYS.ALERTS_SEARCH_HISTORY }}
          />
        }
        mobileFilters={{
          enabled: props.isMobile,
          onToggle: () => props.state.setFiltersOpen((open) => !open),
          count: props.state.activeFilterCount(),
        }}
        showFilters={!props.isMobile || props.state.filtersOpen()}
      >
        <LabeledFilterSelect
          id="alert-time-filter"
          label="Period"
          value={props.state.timeFilter()}
          onChange={(event) =>
            props.state.setTimeFilter(event.currentTarget.value as '24h' | '7d' | '30d' | 'all')
          }
          selectClass="min-w-[7rem]"
        >
          <option value="24h">Last 24h</option>
          <option value="7d">Last 7d</option>
          <option value="30d">Last 30d</option>
          <option value="all">All Time</option>
        </LabeledFilterSelect>
        <LabeledFilterSelect
          id="alert-severity-filter"
          label="Severity"
          value={props.state.severityFilter()}
          onChange={(event) =>
            props.state.setSeverityFilter(
              event.currentTarget.value as 'warning' | 'critical' | 'all',
            )
          }
          selectClass="min-w-[7rem]"
        >
          <option value="all">All</option>
          <option value="critical">Critical</option>
          <option value="warning">Warning</option>
        </LabeledFilterSelect>
      </PageControls>
    </Card>
  );
}
