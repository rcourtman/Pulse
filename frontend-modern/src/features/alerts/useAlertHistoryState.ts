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

import { alertTypeDisplayLabel, unifiedTypeToAlertDisplayType } from './helpers';
import { useAlertIncidentTimelineState } from './useAlertIncidentTimelineState';
import { INCIDENT_EVENT_TYPES } from './types';

const MS_PER_HOUR = 60 * 60 * 1000;

type AlertHistoryRange = '24h' | '7d' | '30d' | 'all';
type AlertSeverityFilter = 'all' | 'warning' | 'critical';

export interface UseAlertHistoryStateProps {
  activeAlerts: Accessor<Record<string, Alert>>;
  getResource: (resourceId: string) => Resource | undefined;
  allResources: () => Resource[];
}

interface HistoryItem {
  id: string;
  source: 'alert' | 'ai';
  status: string;
  startTime: string;
  endTime?: string;
  duration: string;
  resourceName: string;
  resourceType: string;
  resourceId?: string;
  node?: string;
  nodeDisplayName?: string;
  severity: string;
  title: string;
  rawAlertType?: string;
  description?: string;
  acknowledged?: boolean;
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

  const buildHistoryParams = (range: AlertHistoryRange) => {
    const params: { limit?: number; startTime?: string } = {};
    const now = Date.now();

    switch (range) {
      case '24h':
        params.limit = 2000;
        params.startTime = new Date(now - 24 * MS_PER_HOUR).toISOString();
        break;
      case '7d':
        params.limit = 10000;
        params.startTime = new Date(now - 7 * 24 * MS_PER_HOUR).toISOString();
        break;
      case '30d':
        params.limit = 10000;
        params.startTime = new Date(now - 30 * 24 * MS_PER_HOUR).toISOString();
        break;
      case 'all':
        params.limit = 0;
        break;
      default:
        params.limit = 1000;
    }

    return params;
  };

  let fetchRequestId = 0;
  const fetchHistory = async (range: AlertHistoryRange) => {
    const requestId = ++fetchRequestId;
    setLoading(true);

    try {
      const alertHistoryData = await AlertsAPI.getHistory(buildHistoryParams(range));
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

  const formatDuration = (startTime: string, endTime?: string) => {
    const start = new Date(startTime).getTime();
    const end = endTime ? new Date(endTime).getTime() : Date.now();
    const duration = end - start;

    if (duration < 0) return '0m';

    const minutes = Math.floor(duration / 60000);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days}d ${hours % 24}h`;
    if (hours > 0) return `${hours}h ${minutes % 60}m`;
    return `${minutes}m`;
  };

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

  const formatBucketRange = (startMs: number, endMs: number) => {
    const start = new Date(startMs);
    const end = new Date(endMs);
    const sameDay =
      start.getFullYear() === end.getFullYear() &&
      start.getMonth() === end.getMonth() &&
      start.getDate() === end.getDate();

    const startDay = start.toLocaleDateString(userLocale, {
      month: 'short',
      day: 'numeric',
      year: start.getFullYear() !== end.getFullYear() ? 'numeric' : undefined,
    });
    const endDay = end.toLocaleDateString(userLocale, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
    const timeFormatter: Intl.DateTimeFormatOptions = {
      hour: 'numeric',
      minute: '2-digit',
    };

    const startTimeStr = start.toLocaleTimeString(userLocale, timeFormatter);
    const endTimeStr = end.toLocaleTimeString(userLocale, timeFormatter);

    if (sameDay) {
      return `${startDay}, ${startTimeStr} – ${endTimeStr}`;
    }

    return `${startDay}, ${startTimeStr} → ${endDay}, ${endTimeStr}`;
  };

  const getResourceType = (
    resourceName: string,
    metadata?: Record<string, unknown> | undefined,
    resourceId?: string,
  ) => {
    const metadataType =
      typeof metadata?.resourceType === 'string' ? (metadata.resourceType as string) : undefined;
    if (metadataType && metadataType.trim().length > 0) {
      return metadataType;
    }

    if (resourceId) {
      const resource = props.getResource(resourceId);
      if (resource) {
        return unifiedTypeToAlertDisplayType(resource.type);
      }
    }

    const byName = props
      .allResources()
      .find((resource) => resource.name === resourceName || resource.displayName === resourceName);
    if (byName) {
      return unifiedTypeToAlertDisplayType(byName.type);
    }

    return 'Unknown';
  };

  const allHistoryData = createMemo<HistoryItem[]>(() => {
    const items: HistoryItem[] = [];
    const activeAlerts = props.activeAlerts() || {};

    Object.values(activeAlerts).forEach((alert) => {
      items.push({
        id: alert.id,
        source: 'alert',
        status: 'active',
        startTime: alert.startTime,
        duration: formatDuration(alert.startTime),
        resourceName: alert.resourceName,
        resourceType: getResourceType(alert.resourceName, alert.metadata, alert.resourceId),
        resourceId: alert.resourceId,
        node: alert.node,
        nodeDisplayName: alert.nodeDisplayName,
        severity: alert.level,
        title: alertTypeDisplayLabel(alert.type),
        rawAlertType: alert.type,
        description: alert.message,
        acknowledged: false,
      });
    });

    const activeAlertIds = new Set(Object.keys(activeAlerts));
    alertHistory().forEach((alert) => {
      if (activeAlertIds.has(alert.id)) return;

      items.push({
        id: alert.id,
        source: 'alert',
        status: alert.acknowledged ? 'acknowledged' : 'resolved',
        startTime: alert.startTime,
        endTime: alert.lastSeen,
        duration: formatDuration(alert.startTime, alert.lastSeen),
        resourceName: alert.resourceName,
        resourceType: getResourceType(alert.resourceName, alert.metadata, alert.resourceId),
        resourceId: alert.resourceId,
        node: alert.node,
        nodeDisplayName: alert.nodeDisplayName,
        severity: alert.level,
        title: alertTypeDisplayLabel(alert.type),
        rawAlertType: alert.type,
        description: alert.message,
        acknowledged: alert.acknowledged,
      });
    });

    return items;
  });

  const severityAndSearchFilteredItems = createMemo(() => {
    let filtered = allHistoryData();

    if (severityFilter() !== 'all') {
      filtered = filtered.filter((item) => item.severity === severityFilter());
    }

    if (searchTerm()) {
      const term = searchTerm().toLowerCase();
      filtered = filtered.filter((item) => {
        const name = item.resourceName?.toLowerCase() ?? '';
        const title = item.title?.toLowerCase() ?? '';
        const description = item.description?.toLowerCase() ?? '';
        const nodeName = item.node?.toLowerCase() ?? '';
        return (
          name.includes(term) ||
          title.includes(term) ||
          description.includes(term) ||
          nodeName.includes(term)
        );
      });
    }

    return filtered;
  });

  const alertData = createMemo(() => {
    let filtered = severityAndSearchFilteredItems();
    const currentTimeFilter = timeFilter();

    if (selectedBarIndex() !== null) {
      const trends = alertTrends();
      const index = selectedBarIndex()!;
      const bucketStart = trends.bucketTimes[index];
      const bucketEnd = bucketStart + trends.bucketSize * MS_PER_HOUR;

      filtered = filtered.filter((alert) => {
        const alertTime = new Date(alert.startTime).getTime();
        return alertTime >= bucketStart && alertTime < bucketEnd;
      });
    } else if (currentTimeFilter !== 'all') {
      const now = Date.now();
      const cutoffMap: Record<'24h' | '7d' | '30d', number> = {
        '24h': now - 24 * 60 * 60 * 1000,
        '7d': now - 7 * 24 * 60 * 60 * 1000,
        '30d': now - 30 * 24 * 60 * 60 * 1000,
      };
      const cutoff = cutoffMap[currentTimeFilter];
      if (cutoff) {
        filtered = filtered.filter((alert) => new Date(alert.startTime).getTime() > cutoff);
      }
    }

    return [...filtered].sort(
      (a, b) => new Date(b.startTime).getTime() - new Date(a.startTime).getTime(),
    );
  });

  const monthNames = [
    'January',
    'February',
    'March',
    'April',
    'May',
    'June',
    'July',
    'August',
    'September',
    'October',
    'November',
    'December',
  ];

  const getDaySuffix = (day: number) => {
    if (day >= 11 && day <= 13) return 'th';
    switch (day % 10) {
      case 1:
        return 'st';
      case 2:
        return 'nd';
      case 3:
        return 'rd';
      default:
        return 'th';
    }
  };

  const formatAlertGroupLabel = (date: Date, todayStart: number, yesterdayStart: number) => {
    const month = monthNames[date.getMonth()];
    const day = date.getDate();
    const suffix = getDaySuffix(day);
    const absoluteDate = `${month} ${day}${suffix}`;

    if (date.getTime() === todayStart) return `Today (${absoluteDate})`;
    if (date.getTime() === yesterdayStart) return `Yesterday (${absoluteDate})`;
    return absoluteDate;
  };

  const getIncidentRowKey = (alert: HistoryItem) => `${alert.id}::${alert.startTime}`;

  const groupedAlerts = createMemo(() => {
    const now = new Date();
    const todayDate = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const todayStart = todayDate.getTime();
    const yesterdayDate = new Date(todayDate);
    yesterdayDate.setDate(yesterdayDate.getDate() - 1);
    const yesterdayStart = yesterdayDate.getTime();

    const groups = new Map<
      number,
      {
        date: Date;
        label: string;
        fullLabel: string;
        alerts: HistoryItem[];
      }
    >();

    alertData().forEach((alert) => {
      const alertDate = new Date(alert.startTime);
      const dateOnly = new Date(alertDate.getFullYear(), alertDate.getMonth(), alertDate.getDate());
      const dateKey = dateOnly.getTime();

      if (!groups.has(dateKey)) {
        groups.set(dateKey, {
          date: dateOnly,
          label: formatAlertGroupLabel(dateOnly, todayStart, yesterdayStart),
          fullLabel: dateOnly.toLocaleDateString('en-US', {
            weekday: 'long',
            year: 'numeric',
            month: 'long',
            day: 'numeric',
          }),
          alerts: [],
        });
      }

      groups.get(dateKey)!.alerts.push(alert);
    });

    return Array.from(groups.values()).sort((a, b) => b.date.getTime() - a.date.getTime());
  });

  const alertTrends = createMemo(() => {
    const now = Date.now();
    const filteredAlerts = severityAndSearchFilteredItems();
    const niceBucketSizes = [1, 2, 3, 6, 12, 24, 48, 72, 168, 336, 720, 1440];
    const maxBuckets = 30;

    let bucketSizeHours: number;
    let computedRangeHours: number;
    let startTime: number;

    const filter = timeFilter();
    if (filter === '24h') {
      bucketSizeHours = 1;
      computedRangeHours = 24;
      startTime = now - computedRangeHours * MS_PER_HOUR;
    } else if (filter === '7d') {
      bucketSizeHours = 6;
      computedRangeHours = 7 * 24;
      startTime = now - computedRangeHours * MS_PER_HOUR;
    } else if (filter === '30d') {
      bucketSizeHours = 24;
      computedRangeHours = 30 * 24;
      startTime = now - computedRangeHours * MS_PER_HOUR;
    } else if (!filteredAlerts.length) {
      bucketSizeHours = 24;
      computedRangeHours = 24;
      startTime = now - computedRangeHours * MS_PER_HOUR;
    } else {
      const earliest = filteredAlerts.reduce((min, alert) => {
        const alertTime = new Date(alert.startTime).getTime();
        return Math.min(min, alertTime);
      }, now);
      const rawRangeHours = Math.max(1, Math.ceil((now - earliest) / MS_PER_HOUR));
      const rawBucketSize = Math.max(1, Math.ceil(rawRangeHours / maxBuckets));
      bucketSizeHours = niceBucketSizes.find((size) => size >= rawBucketSize) ?? rawBucketSize;
      computedRangeHours = Math.max(rawRangeHours, bucketSizeHours);
      const bucketsNeeded = Math.min(
        Math.max(1, Math.ceil(computedRangeHours / bucketSizeHours)),
        maxBuckets,
      );
      startTime = now - bucketsNeeded * bucketSizeHours * MS_PER_HOUR;
    }

    const bucketCount = Math.min(
      Math.max(1, Math.ceil(computedRangeHours / bucketSizeHours)),
      maxBuckets,
    );
    startTime = Math.min(startTime, now - bucketCount * bucketSizeHours * MS_PER_HOUR);

    const buckets = new Array(bucketCount).fill(0);
    const bucketTimes = new Array(bucketCount)
      .fill(0)
      .map((_, index) => startTime + index * bucketSizeHours * MS_PER_HOUR);

    const windowStart = startTime;
    const windowEnd = now;

    filteredAlerts.forEach((alert) => {
      const alertTime = new Date(alert.startTime).getTime();
      if (alertTime < windowStart || alertTime > windowEnd) return;

      const rawIndex = Math.floor((alertTime - windowStart) / (bucketSizeHours * MS_PER_HOUR));
      const bucketIndex = Math.min(bucketCount - 1, Math.max(0, rawIndex));
      if (bucketIndex >= 0 && bucketIndex < bucketCount) {
        buckets[bucketIndex]++;
      }
    });

    return {
      buckets,
      max: Math.max(...buckets, 1),
      bucketSize: bucketSizeHours,
      bucketTimes,
      rangeStart: windowStart,
      rangeHours: bucketCount * bucketSizeHours,
    };
  });

  const bucketDurationLabel = createMemo(() => {
    const bucketHours = alertTrends().bucketSize;
    if (!Number.isFinite(bucketHours) || bucketHours <= 0) return '—';
    if (bucketHours % 24 === 0) {
      const days = bucketHours / 24;
      return `${days} day${days === 1 ? '' : 's'}`;
    }
    return `${bucketHours} hour${bucketHours === 1 ? '' : 's'}`;
  });

  const formatAxisTickLabel = (
    timestamp: number,
    bucketHours: number,
    totalHours: number,
    isEnd = false,
  ) => {
    if (!Number.isFinite(timestamp)) return '—';

    if (isEnd && Math.abs(Date.now() - timestamp) < bucketHours * MS_PER_HOUR * 0.75) {
      return 'Now';
    }

    const date = new Date(timestamp);
    const options: Intl.DateTimeFormatOptions = {};

    if (totalHours <= 48) {
      options.month = 'short';
      options.day = 'numeric';
      options.hour = '2-digit';
      options.minute = '2-digit';
    } else if (totalHours <= 24 * 90) {
      options.month = 'short';
      options.day = 'numeric';
      if (bucketHours <= 12 || totalHours <= 24 * 14) {
        options.hour = '2-digit';
      }
    } else {
      options.year = 'numeric';
      options.month = 'short';
      options.day = 'numeric';
    }

    return date.toLocaleString(userLocale, options);
  };

  const rangeSummary = createMemo(() => {
    const trends = alertTrends();
    if (!trends.bucketTimes.length || trends.bucketSize <= 0) return null;

    const bucketHours = trends.bucketSize;
    const totalHours = Math.max(trends.rangeHours ?? bucketHours, bucketHours);
    const start = trends.bucketTimes[0];
    const end = start + trends.buckets.length * bucketHours * MS_PER_HOUR;

    return {
      startLabel: formatAxisTickLabel(start, bucketHours, totalHours),
      endLabel: formatAxisTickLabel(end, bucketHours, totalHours, true),
    };
  });

  const axisTicks = createMemo(() => {
    const trends = alertTrends();
    if (!trends.bucketTimes.length || trends.bucketSize <= 0) {
      return [] as Array<{ position: number; label: string; align: 'start' | 'center' | 'end' }>;
    }

    const bucketHours = trends.bucketSize;
    const totalHours = Math.max(trends.rangeHours ?? bucketHours, bucketHours);
    const start = trends.bucketTimes[0];
    const totalDurationMs = Math.max(
      trends.buckets.length * bucketHours * MS_PER_HOUR,
      bucketHours * MS_PER_HOUR,
    );
    const end = start + totalDurationMs;
    const desiredTicks = Math.min(5, trends.bucketTimes.length + 1);
    const step = Math.max(1, Math.round(trends.bucketTimes.length / Math.max(1, desiredTicks - 1)));
    const ticks: Array<{ position: number; label: string }> = [];

    for (let index = 0; index < trends.bucketTimes.length; index += step) {
      const ts = trends.bucketTimes[index];
      const position = Math.min(1, Math.max(0, (ts - start) / (totalDurationMs || 1)));
      ticks.push({
        position,
        label: formatAxisTickLabel(ts, bucketHours, totalHours),
      });
    }

    if (!ticks.length || ticks[0].position > 0.01) {
      ticks.unshift({
        position: 0,
        label: formatAxisTickLabel(start, bucketHours, totalHours),
      });
    } else {
      ticks[0] = {
        position: 0,
        label: formatAxisTickLabel(start, bucketHours, totalHours),
      };
    }

    const lastTick = ticks[ticks.length - 1];
    if (!lastTick || Math.abs(lastTick.position - 1) > 0.01) {
      ticks.push({
        position: 1,
        label: formatAxisTickLabel(end, bucketHours, totalHours, true),
      });
    } else {
      ticks[ticks.length - 1] = {
        position: 1,
        label: formatAxisTickLabel(end, bucketHours, totalHours, true),
      };
    }

    return ticks.map((tick, index, arr) => ({
      position: tick.position,
      label: tick.label,
      align: index === 0 ? 'start' : index === arr.length - 1 ? 'end' : 'center',
    }));
  });

  const selectedBucketDetails = createMemo(() => {
    const index = selectedBarIndex();
    if (index === null) return null;

    const trends = alertTrends();
    const bucketStart = trends.bucketTimes[index];
    const bucketEnd = bucketStart + trends.bucketSize * MS_PER_HOUR;
    return {
      rangeLabel: formatBucketRange(bucketStart, bucketEnd),
      start: bucketStart,
      end: bucketEnd,
    };
  });

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
    formatBucketRange,
    getIncidentRowKey,
    clearAlertHistory,
  };
}
