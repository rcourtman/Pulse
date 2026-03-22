import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import type { Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import type { Alert, Incident } from '@/types/api';
import type { Resource } from '@/types/resource';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { logger } from '@/utils/logger';
import {
  getAlertAdministrationClearHistoryError,
  getAlertAdministrationClearHistoryConfirmation,
} from '@/utils/alertAdministrationPresentation';
import { getAlertResourceIncidentLoadFailure } from '@/utils/alertIncidentPresentation';

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
import { INCIDENT_EVENT_TYPES } from './types';

export interface UseAlertHistoryStateProps {
  activeAlerts: Accessor<Record<string, Alert>>;
  getResource: (resourceId: string) => Resource | undefined;
  allResources: () => Resource[];
}

export function useAlertHistoryState(props: UseAlertHistoryStateProps) {
  const [timeFilter, setTimeFilter] = usePersistentSignal<AlertHistoryRange>(
    'alertHistoryTimeFilter',
    '7d',
    {
      deserialize: (raw) =>
        raw === '24h' || raw === '7d' || raw === '30d' || raw === 'all' ? raw : '7d',
    },
  );
  const [severityFilter, setSeverityFilter] = usePersistentSignal<AlertSeverityFilter>(
    'alertHistorySeverityFilter',
    'all',
    {
      deserialize: (raw) => (raw === 'warning' || raw === 'critical' ? raw : 'all'),
    },
  );
  const [searchTerm, setSearchTerm] = createSignal('');
  const [alertHistory, setAlertHistory] = createSignal<Alert[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [selectedBarIndex, setSelectedBarIndex] = createSignal<number | null>(null);
  const [resourceIncidentPanel, setResourceIncidentPanel] = createSignal<{
    resourceId: string;
    resourceName: string;
  } | null>(null);
  const [resourceIncidents, setResourceIncidents] = createSignal<Record<string, Incident[]>>({});
  const [resourceIncidentLoading, setResourceIncidentLoading] = createSignal<
    Record<string, boolean>
  >({});
  const [expandedResourceIncidentIds, setExpandedResourceIncidentIds] = createSignal<Set<string>>(
    new Set(),
  );
  const [resourceIncidentEventFilters, setResourceIncidentEventFilters] = createSignal<Set<string>>(
    new Set(INCIDENT_EVENT_TYPES),
  );
  const [filtersOpen, setFiltersOpen] = createSignal(false);

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
    void fetchHistory(timeFilter());

    const unsubscribeOrgSwitched = eventBus.on('org_switched', () => {
      setAlertHistory([]);
      setSelectedBarIndex(null);
      setResourceIncidentPanel(null);
      setResourceIncidents({});
      setResourceIncidentLoading({});
      setExpandedResourceIncidentIds(new Set<string>());
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

  const loadResourceIncidents = async (resourceId: string, limit = 10) => {
    if (!resourceId) return;

    setResourceIncidentLoading((prev) => ({ ...prev, [resourceId]: true }));
    try {
      const incidents = await AlertsAPI.getIncidentsForResource(resourceId, limit);
      setResourceIncidents((prev) => ({ ...prev, [resourceId]: incidents }));
    } catch (error) {
      logger.error(getAlertResourceIncidentLoadFailure(), error);
      notificationStore.error(getAlertResourceIncidentLoadFailure());
    } finally {
      setResourceIncidentLoading((prev) => ({ ...prev, [resourceId]: false }));
    }
  };

  const openResourceIncidentPanel = async (resourceId: string, resourceName: string) => {
    if (!resourceId) return;

    setResourceIncidentPanel({ resourceId, resourceName });
    setExpandedResourceIncidentIds(new Set<string>());
    if (!(resourceId in resourceIncidents())) {
      await loadResourceIncidents(resourceId);
    }
  };

  const refreshResourceIncidentPanel = async () => {
    const selection = resourceIncidentPanel();
    if (!selection) return;
    await loadResourceIncidents(selection.resourceId);
  };

  const toggleResourceIncidentDetails = (incidentId: string) => {
    setExpandedResourceIncidentIds((prev) => {
      const next = new Set(prev);
      if (next.has(incidentId)) {
        next.delete(incidentId);
      } else {
        next.add(incidentId);
      }
      return next;
    });
  };

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
    resourceIncidentPanel,
    setResourceIncidentPanel,
    resourceIncidents,
    resourceIncidentLoading,
    expandedResourceIncidentIds,
    resourceIncidentEventFilters,
    setResourceIncidentEventFilters,
    filtersOpen,
    setFiltersOpen,
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
    openResourceIncidentPanel,
    refreshResourceIncidentPanel,
    toggleResourceIncidentDetails,
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
