import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import type { Accessor } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';

import { AlertsAPI } from '@/api/alerts';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { logger } from '@/utils/logger';
import {
  getAlertAdministrationClearHistoryError,
  getAlertAdministrationClearHistoryConfirmation,
} from '@/utils/alertAdministrationPresentation';

import {
  applyAlertHistoryWindow,
  buildAlertAxisTicks,
  buildAlertHistoryItems,
  buildAlertHistoryParams,
  buildAlertRangeSummary,
  buildAlertTrends,
  buildSelectedBucketDetails,
  filterAlertHistoryItems,
  formatAlertBucketRange,
  getAlertBucketDurationLabel,
  getIncidentRowKey,
  groupAlertHistoryItems,
  type AlertHistoryRange,
  type AlertSeverityFilter,
  type HistoryItem,
} from './alertHistoryModel';
import { useAlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import { useAlertResourceIncidentsState } from './useAlertResourceIncidentsState';

export interface UseAlertHistoryStateProps {
  activeAlerts: Accessor<Record<string, Alert>>;
  getResource: (resourceId: string) => Resource | undefined;
  allResources: () => Resource[];
}

const DEFAULT_TIME_FILTER: AlertHistoryRange = '7d';
const DEFAULT_SEVERITY_FILTER: AlertSeverityFilter = 'all';

const parsePeriod = (raw: string | null | undefined): AlertHistoryRange =>
  raw === '24h' || raw === '7d' || raw === '30d' || raw === 'all' ? raw : DEFAULT_TIME_FILTER;

const parseSeverity = (raw: string | null | undefined): AlertSeverityFilter =>
  raw === 'warning' || raw === 'critical' ? raw : DEFAULT_SEVERITY_FILTER;

export function useAlertHistoryState(props: UseAlertHistoryStateProps) {
  const location = useLocation();
  const navigate = useNavigate();

  const timeFilter: Accessor<AlertHistoryRange> = () =>
    parsePeriod(new URLSearchParams(location.search).get('period'));
  const severityFilter: Accessor<AlertSeverityFilter> = () =>
    parseSeverity(new URLSearchParams(location.search).get('severity'));
  const searchTerm: Accessor<string> = () =>
    new URLSearchParams(location.search).get('q') ?? '';

  const updateSearchParam = (
    mutate: (params: URLSearchParams) => void,
  ): void => {
    const params = new URLSearchParams(location.search);
    mutate(params);
    const query = params.toString();
    navigate(`${location.pathname}${query ? `?${query}` : ''}`, { replace: true });
  };

  const setTimeFilter = (value: AlertHistoryRange): void => {
    updateSearchParam((params) => {
      if (value === DEFAULT_TIME_FILTER) {
        params.delete('period');
      } else {
        params.set('period', value);
      }
    });
  };

  const setSeverityFilter = (value: AlertSeverityFilter): void => {
    updateSearchParam((params) => {
      if (value === DEFAULT_SEVERITY_FILTER) {
        params.delete('severity');
      } else {
        params.set('severity', value);
      }
    });
  };

  const setSearchTerm = (value: string): void => {
    updateSearchParam((params) => {
      if (value === '') {
        params.delete('q');
      } else {
        params.set('q', value);
      }
    });
  };

  const [alertHistory, setAlertHistory] = createSignal<Alert[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [selectedBarIndex, setSelectedBarIndex] = createSignal<number | null>(null);
  const resourceIncidentsState = useAlertResourceIncidentsState();

  const {
    incidentTimelines,
    incidentLoading,
    incidentErrors,
    expandedIncidents,
    incidentNoteDrafts,
    incidentNoteSaving,
    eventFilters,
    setEventFilters,
    resetState,
    loadIncidentTimeline,
    toggleIncidentTimeline,
    setIncidentNoteDraft,
    saveIncidentNote,
  } = useAlertIncidentTimelineState();

  const activeFilterCount = createMemo(() => {
    let count = 0;
    if (timeFilter() !== '7d') count++;
    if (severityFilter() !== 'all') count++;
    return count;
  });

  const userLocale =
    Intl.DateTimeFormat().resolvedOptions().locale ||
    (typeof navigator !== 'undefined' ? navigator.language : undefined) ||
    'en-US';

  let fetchRequestId = 0;
  const fetchHistory = async (range: AlertHistoryRange) => {
    const requestId = ++fetchRequestId;
    setLoading(true);

    try {
      const alertHistoryData = await AlertsAPI.getHistory(buildAlertHistoryParams(range));
      if (requestId === fetchRequestId) {
        setAlertHistory(alertHistoryData);
      }
    } catch (error) {
      if (requestId === fetchRequestId) {
        logger.error('Failed to load history:', error);
      }
    } finally {
      if (requestId === fetchRequestId) {
        setLoading(false);
      }
    }
  };

  let lastTimeFilterValue: string | null = null;
  createEffect(() => {
    const current = timeFilter();
    if (lastTimeFilterValue !== null && current !== lastTimeFilterValue) {
      setSelectedBarIndex(null);
    }
    lastTimeFilterValue = current;
  });

  let lastSeverityFilterValue: string | null = null;
  createEffect(() => {
    const current = severityFilter();
    if (lastSeverityFilterValue !== null && current !== lastSeverityFilterValue) {
      setSelectedBarIndex(null);
    }
    lastSeverityFilterValue = current;
  });

  onMount(() => {
    // Migrate legacy localStorage-backed filter prefs into the URL on first
    // visit after upgrade. Keeps the dead localStorage keys in place rather
    // than evicting them; harmless and avoids an eviction migration.
    if (typeof window !== 'undefined') {
      const params = new URLSearchParams(window.location.search);
      let mutated = false;

      if (!params.has('period')) {
        const legacy = window.localStorage.getItem('alertHistoryTimeFilter');
        const parsed = parsePeriod(legacy);
        if (parsed !== DEFAULT_TIME_FILTER && legacy === parsed) {
          params.set('period', parsed);
          mutated = true;
        }
      }

      if (!params.has('severity')) {
        const legacy = window.localStorage.getItem('alertHistorySeverityFilter');
        const parsed = parseSeverity(legacy);
        if (parsed !== DEFAULT_SEVERITY_FILTER && legacy === parsed) {
          params.set('severity', parsed);
          mutated = true;
        }
      }

      if (mutated) {
        navigate(`${location.pathname}?${params.toString()}`, { replace: true });
      }
    }

    void fetchHistory(timeFilter());

    const unsubscribeOrgSwitched = eventBus.on('org_switched', () => {
      setAlertHistory([]);
      setSelectedBarIndex(null);
      resourceIncidentsState.resetResourceIncidentsState();
      resetState();
      void fetchHistory(timeFilter());
    });

    onCleanup(() => {
      unsubscribeOrgSwitched();
      fetchRequestId++;
    });
  });

  let skipInitialFetchEffect = true;
  createEffect(() => {
    const range = timeFilter();
    if (skipInitialFetchEffect) {
      skipInitialFetchEffect = false;
      return;
    }
    void fetchHistory(range);
  });

  const allHistoryData = createMemo<HistoryItem[]>(() => {
    return buildAlertHistoryItems({
      activeAlerts: props.activeAlerts() || {},
      alertHistory: alertHistory(),
      getResource: props.getResource,
      allResources: props.allResources(),
    });
  });

  const severityAndSearchFilteredItems = createMemo(() => {
    return filterAlertHistoryItems(allHistoryData(), severityFilter(), searchTerm());
  });

  const alertTrends = createMemo(() => {
    return buildAlertTrends(severityAndSearchFilteredItems(), timeFilter());
  });

  const alertData = createMemo(() => {
    return applyAlertHistoryWindow({
      filteredItems: severityAndSearchFilteredItems(),
      timeFilter: timeFilter(),
      selectedBarIndex: selectedBarIndex(),
      trends: alertTrends(),
    });
  });

  const groupedAlerts = createMemo(() => {
    return groupAlertHistoryItems(alertData());
  });

  const bucketDurationLabel = createMemo(() => getAlertBucketDurationLabel(alertTrends().bucketSize));

  const rangeSummary = createMemo(() => {
    return buildAlertRangeSummary(alertTrends(), userLocale);
  });

  const axisTicks = createMemo(() => buildAlertAxisTicks(alertTrends(), userLocale));

  const selectedBucketDetails = createMemo(() =>
    buildSelectedBucketDetails(selectedBarIndex(), alertTrends(), userLocale),
  );

  const clearAlertHistory = async () => {
    if (!confirm(getAlertAdministrationClearHistoryConfirmation())) {
      return;
    }

    try {
      await AlertsAPI.clearHistory();
      setAlertHistory([]);
    } catch (error) {
      logger.error(getAlertAdministrationClearHistoryError(), error);
      notificationStore.error(getAlertAdministrationClearHistoryError());
    }
  };

  return {
    STORAGE_KEYS,
    timeFilter,
    setTimeFilter,
    severityFilter,
    setSeverityFilter,
    searchTerm,
    setSearchTerm,
    alertHistory,
    loading,
    selectedBarIndex,
    setSelectedBarIndex,
    resourceIncidentPanel: resourceIncidentsState.resourceIncidentPanel,
    setResourceIncidentPanel: resourceIncidentsState.setResourceIncidentPanel,
    resourceIncidents: resourceIncidentsState.resourceIncidents,
    resourceIncidentLoading: resourceIncidentsState.resourceIncidentLoading,
    expandedResourceIncidentIds: resourceIncidentsState.expandedResourceIncidentIds,
    resourceIncidentEventFilters: resourceIncidentsState.resourceIncidentEventFilters,
    setResourceIncidentEventFilters: resourceIncidentsState.setResourceIncidentEventFilters,
    activeFilterCount,
    incidentTimelines,
    incidentLoading,
    incidentErrors,
    expandedIncidents,
    incidentNoteDrafts,
    incidentNoteSaving,
    historyIncidentEventFilters: eventFilters,
    setHistoryIncidentEventFilters: setEventFilters,
    loadIncidentTimeline,
    toggleIncidentTimeline,
    setIncidentNoteDraft,
    saveIncidentNote,
    openResourceIncidentPanel: resourceIncidentsState.openResourceIncidentPanel,
    refreshResourceIncidentPanel: resourceIncidentsState.refreshResourceIncidentPanel,
    toggleResourceIncidentDetails: resourceIncidentsState.toggleResourceIncidentDetails,
    alertData,
    groupedAlerts,
    alertTrends,
    bucketDurationLabel,
    rangeSummary,
    axisTicks,
    selectedBucketDetails,
    formatBucketRange: (startMs: number, endMs: number) =>
      formatAlertBucketRange(startMs, endMs, userLocale),
    getIncidentRowKey,
    clearAlertHistory,
  };
}

export type AlertHistoryState = ReturnType<typeof useAlertHistoryState>;
