import { createEffect, createMemo, createSignal, For, onCleanup, onMount, Show } from 'solid-js';

import type { Alert, Incident } from '@/types/api';
import type { Resource } from '@/types/resource';
import { AlertsAPI } from '@/api/alerts';
import { useWebSocket } from '@/App';
import { IncidentEventFilters } from '@/components/Alerts/IncidentEventFilters';
import { IncidentTimelineEventCard } from '@/components/Alerts/IncidentTimelineEventCard';
import { IncidentTimelinePanel } from '@/components/Alerts/IncidentTimelinePanel';
import { InvestigateAlertButton } from '@/components/Alerts/InvestigateAlertButton';
import { Card } from '@/components/shared/Card';
import { LabeledFilterSelect } from '@/components/shared/FilterToolbar';
import { PageControls } from '@/components/shared/PageControls';
import { SearchInput } from '@/components/shared/SearchInput';
import { SectionHeader } from '@/components/shared/SectionHeader';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { usePersistentSignal } from '@/hooks/usePersistentSignal';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { logger } from '@/utils/logger';
import { hideTooltip, showTooltip } from '@/components/shared/Tooltip';
import {
  getAlertAdministrationClearHistoryConfirmation,
  getAlertAdministrationClearHistoryError,
  getAlertAdministrationClearHistoryLabel,
  getAlertAdministrationSectionDescription,
  getAlertAdministrationSectionTitle,
} from '@/utils/alertAdministrationPresentation';
import {
  getAlertBucketCountLabel,
  getAlertHistoryEmptyState,
  getAlertHistoryLoadingState,
  getAlertHistorySearchPlaceholder,
} from '@/utils/alertOverviewPresentation';
import {
  getAlertFrequencyClearFilterButtonClass,
  getAlertFrequencySelectionPresentation,
} from '@/utils/alertFrequencyPresentation';
import {
  getAlertHistoryResourceTypeBadgeClass,
  getAlertHistorySourcePresentation,
} from '@/utils/alertHistoryPresentation';
import {
  getAlertHistoryStatusPresentation,
  getAlertIncidentLevelBadgeClass,
  getAlertIncidentStatusPresentation,
  getAlertIncidentTimelineHeadingClass,
  getAlertIncidentTimelineMetaRowClass,
  getAlertIncidentTimelineOutputClass,
  getAlertResourceIncidentAcknowledgedByLabel,
  getAlertResourceIncidentActivityChipClass,
  getAlertResourceIncidentActivitySummaryClass,
  getAlertResourceIncidentCardClass,
  getAlertResourceIncidentCountLabel,
  getAlertResourceIncidentEmptyState,
  getAlertResourceIncidentFilteredEventsEmptyState,
  getAlertResourceIncidentLoadFailure,
  getAlertResourceIncidentLoadingState,
  getAlertResourceIncidentPanelTitle,
  getAlertResourceIncidentRefreshLabel,
  getAlertResourceIncidentSummaryRowClass,
  getAlertResourceIncidentToggleButtonClass,
  getAlertResourceIncidentToggleLabel,
  getAlertResourceIncidentTruncatedEventsLabel,
  getAlertResourceIncidentViewTitle,
} from '@/utils/alertIncidentPresentation';
import { getAlertSeverityDotClass } from '@/utils/alertSeverityPresentation';
import { getTypeColumnLabel } from '@/utils/typeColumnPresentation';

import {
  alertTypeDisplayLabel,
  unifiedTypeToAlertDisplayType,
} from '../helpers';
import { useAlertIncidentTimelineState } from '../useAlertIncidentTimelineState';
import {
  filterIncidentEvents,
  INCIDENT_EVENT_TYPES,
  summarizeIncidentEvents,
} from '../types';

const MS_PER_HOUR = 60 * 60 * 1000;

export interface HistoryTabProps {
  hasAIAlertsFeature: () => boolean;
  licenseLoading: () => boolean;
  getResource: (resourceId: string) => Resource | undefined;
  allResources: () => Resource[];
}

export function HistoryTab(props: HistoryTabProps) {
  const { activeAlerts } = useWebSocket();
  const alertFrequencySelectionPresentation = createMemo(() =>
    getAlertFrequencySelectionPresentation(),
  );

  const [timeFilter, setTimeFilter] = usePersistentSignal<'24h' | '7d' | '30d' | 'all'>(
    'alertHistoryTimeFilter',
    '7d',
    {
      deserialize: (raw) =>
        raw === '24h' || raw === '7d' || raw === '30d' || raw === 'all' ? raw : '7d',
    },
  );
  const [severityFilter, setSeverityFilter] = usePersistentSignal<'all' | 'warning' | 'critical'>(
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
  const { isMobile } = useBreakpoint();
  const [filtersOpen, setFiltersOpen] = createSignal(false);
  const {
    incidentTimelines,
    incidentLoading,
    incidentErrors,
    expandedIncidents,
    incidentNoteDrafts,
    incidentNoteSaving,
    eventFilters: historyIncidentEventFilters,
    setEventFilters: setHistoryIncidentEventFilters,
    resetState: resetIncidentTimelineState,
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

  const buildHistoryParams = (range: string) => {
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
  const fetchHistory = async (range: string) => {
    const requestId = ++fetchRequestId;
    setLoading(true);

    try {
      const params = buildHistoryParams(range);
      const alertHistoryData = await AlertsAPI.getHistory(params);

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
      resetIncidentTimelineState();
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

    if (duration < 0) {
      return '0m';
    }

    const minutes = Math.floor(duration / 60000);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days}d ${hours % 24}h`;
    if (hours > 0) return `${hours}h ${minutes % 60}m`;
    return `${minutes}m`;
  };

  const loadResourceIncidents = async (resourceId: string, limit = 10) => {
    if (!resourceId) {
      return;
    }
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
    if (!resourceId) {
      return;
    }
    setResourceIncidentPanel({ resourceId, resourceName });
    setExpandedResourceIncidentIds(new Set<string>());
    if (!(resourceId in resourceIncidents())) {
      await loadResourceIncidents(resourceId);
    }
  };

  const refreshResourceIncidentPanel = async () => {
    const selection = resourceIncidentPanel();
    if (!selection) {
      return;
    }
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

  type HistoryItemSource = 'alert' | 'ai';
  interface HistoryItem {
    id: string;
    source: HistoryItemSource;
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

  const allHistoryData = createMemo(() => {
    const items: HistoryItem[] = [];

    Object.values(activeAlerts || {}).forEach((alert) => {
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

    const activeAlertIds = new Set(Object.keys(activeAlerts || {}));

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
      const currentSeverityFilter = severityFilter();
      filtered = filtered.filter((item) => item.severity === currentSeverityFilter);
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

    if (date.getTime() === todayStart) {
      return `Today (${absoluteDate})`;
    }

    if (date.getTime() === yesterdayStart) {
      return `Yesterday (${absoluteDate})`;
    }

    return absoluteDate;
  };

  type AlertHistoryRow = ReturnType<typeof alertData>[number];
  const getIncidentRowKey = (alert: AlertHistoryRow) => `${alert.id}::${alert.startTime}`;

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
        alerts: AlertHistoryRow[];
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
    } else {
      if (!filteredAlerts.length) {
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
      if (alertTime < windowStart || alertTime > windowEnd) {
        return;
      }
      const rawIndex = Math.floor((alertTime - windowStart) / (bucketSizeHours * MS_PER_HOUR));
      const bucketIndex = Math.min(bucketCount - 1, Math.max(0, rawIndex));
      if (bucketIndex >= 0 && bucketIndex < bucketCount) {
        buckets[bucketIndex]++;
      }
    });

    const max = Math.max(...buckets, 1);

    return {
      buckets,
      max,
      bucketSize: bucketSizeHours,
      bucketTimes,
      rangeStart: windowStart,
      rangeHours: bucketCount * bucketSizeHours,
    };
  });

  const bucketDurationLabel = createMemo(() => {
    const bucketHours = alertTrends().bucketSize;
    if (!Number.isFinite(bucketHours) || bucketHours <= 0) {
      return '—';
    }
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
    if (!trends.bucketTimes.length || trends.bucketSize <= 0) {
      return null;
    }

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

  return (
    <div class="space-y-4">
      <Card padding="md">
        <div class="mb-3 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
          <SectionHeader
            title="Alert frequency"
            description={<span class="text-xs text-muted">{alertData().length} alerts</span>}
            size="sm"
            class="flex-1"
          />
          <div class="flex flex-col items-start gap-2 sm:items-end">
            <Show when={selectedBucketDetails()}>
              {(selection) => (
                <div class={alertFrequencySelectionPresentation().containerClass}>
                  <span class={alertFrequencySelectionPresentation().labelClass}>
                    Filtered Range
                  </span>
                  <span class="font-mono text-[11px]">{selection().rangeLabel}</span>
                </div>
              )}
            </Show>
            <div class="flex flex-col items-start gap-1 text-xs text-muted sm:items-end">
              <div>
                <span class="font-medium text-muted">Bar size:</span> {bucketDurationLabel()}
              </div>
              <Show when={rangeSummary()}>
                {(summary) => (
                  <div class="flex items-center gap-1 whitespace-nowrap">
                    <span class="font-medium text-muted">Range:</span>
                    <span>{summary().startLabel}</span>
                    <span class="text-muted">→</span>
                    <span>{summary().endLabel}</span>
                  </div>
                )}
              </Show>
            </div>
            <div class="flex flex-wrap items-center justify-end gap-2">
              <Show when={selectedBarIndex() !== null}>
                <button
                  type="button"
                  onClick={() => setSelectedBarIndex(null)}
                  class={getAlertFrequencyClearFilterButtonClass()}
                >
                  Clear filter
                </button>
              </Show>
              <div class="flex items-center gap-2 text-xs text-muted">
                <span class="flex items-center gap-1">
                  <div class={getAlertSeverityDotClass('warning')}></div>
                  {alertData().filter((alert) => alert.severity === 'warning').length} warnings
                </span>
                <span class="flex items-center gap-1">
                  <div class={getAlertSeverityDotClass('critical')}></div>
                  {alertData().filter((alert) => alert.severity === 'critical').length} critical
                </span>
              </div>
            </div>
          </div>
        </div>

        <div class="mb-1 text-[10px] text-muted">
          Showing {alertTrends().buckets.length} time periods ({bucketDurationLabel()} each) ·
          Total: {alertData().length} alerts
        </div>

        {(() => {
          const trends = alertTrends();
          return (
            <div class="rounded bg-surface-alt p-1">
              <div class="flex h-12 items-end gap-1">
                {trends.buckets.map((value, index) => {
                  const scaledHeight =
                    value > 0 ? Math.min(100, Math.max(20, Math.log(value + 1) * 20)) : 0;
                  const pixelHeight = value > 0 ? Math.max(8, (scaledHeight / 100) * 40) : 0;
                  const isSelected = selectedBarIndex() === index;
                  const bucketStart = trends.bucketTimes[index];
                  const bucketEnd = bucketStart + trends.bucketSize * MS_PER_HOUR;
                  const bucketRangeLabel = formatBucketRange(bucketStart, bucketEnd);
                  const bucketDurationText =
                    trends.bucketSize % 24 === 0
                      ? `${trends.bucketSize / 24} day${trends.bucketSize / 24 === 1 ? '' : 's'}`
                      : `${trends.bucketSize} hour${trends.bucketSize === 1 ? '' : 's'}`;
                  const countLabel = getAlertBucketCountLabel(value);
                  const tooltipContent = [
                    countLabel,
                    `${bucketDurationText} period`,
                    bucketRangeLabel,
                  ].join('\n');
                  return (
                    <div
                      class="relative flex flex-1 cursor-pointer items-end"
                      role="button"
                      tabIndex={0}
                      aria-pressed={isSelected}
                      aria-label={`${countLabel} between ${bucketRangeLabel}`}
                      onClick={() => setSelectedBarIndex(index === selectedBarIndex() ? null : index)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          setSelectedBarIndex(index === selectedBarIndex() ? null : index);
                        }
                      }}
                    >
                      <div class="absolute bottom-0 h-1 w-full rounded-full bg-slate-300 opacity-30"></div>
                      <div
                        class="relative w-full rounded-sm transition-all"
                        style={{
                          height: `${pixelHeight}px`,
                          'background-color':
                            value > 0 ? (isSelected ? '#2563eb' : '#3b82f6') : 'transparent',
                          opacity: isSelected ? '1' : '0.8',
                          'box-shadow': isSelected ? '0 0 0 2px rgba(37, 99, 235, 0.4)' : 'none',
                        }}
                        title={bucketRangeLabel}
                        onMouseEnter={(event) => {
                          if (value <= 0) {
                            hideTooltip();
                            return;
                          }
                          const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
                          showTooltip(tooltipContent, rect.left + rect.width / 2, rect.top, {
                            align: 'center',
                            direction: 'up',
                          });
                        }}
                        onMouseLeave={() => hideTooltip()}
                      />
                    </div>
                  );
                })}
              </div>
            </div>
          );
        })()}

        <Show when={axisTicks().length > 0}>
          <div class="relative mt-3 h-10">
            <div class="absolute inset-x-0 top-0 h-px bg-surface-hover"></div>
            <For each={axisTicks()}>
              {(tick) => (
                <div
                  class="pointer-events-none absolute top-0 flex h-full flex-col items-center"
                  style={{ left: `${tick.position * 100}%` }}
                >
                  <div class="h-3 w-px bg-slate-300"></div>
                  <div
                    class="mt-1 whitespace-nowrap text-[10px] text-muted transform"
                    classList={{
                      '-translate-x-1/2': tick.align === 'center',
                      '-translate-x-full': tick.align === 'end',
                    }}
                  >
                    {tick.label}
                  </div>
                </div>
              )}
            </For>
          </div>
        </Show>
      </Card>

      <Card padding="sm" class="mb-4">
        <PageControls
          search={
            <SearchInput
              value={searchTerm}
              onChange={setSearchTerm}
              placeholder={getAlertHistorySearchPlaceholder()}
              class="w-full"
              clearOnEscape
              history={{ storageKey: STORAGE_KEYS.ALERTS_SEARCH_HISTORY }}
            />
          }
          mobileFilters={{
            enabled: isMobile(),
            onToggle: () => setFiltersOpen((open) => !open),
            count: activeFilterCount(),
          }}
          showFilters={!isMobile() || filtersOpen()}
        >
          <LabeledFilterSelect
            id="alert-time-filter"
            label="Period"
            value={timeFilter()}
            onChange={(event) =>
              setTimeFilter(event.currentTarget.value as '24h' | '7d' | '30d' | 'all')
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
            value={severityFilter()}
            onChange={(event) =>
              setSeverityFilter(event.currentTarget.value as 'warning' | 'critical' | 'all')
            }
            selectClass="min-w-[7rem]"
          >
            <option value="all">All</option>
            <option value="critical">Critical</option>
            <option value="warning">Warning</option>
          </LabeledFilterSelect>
        </PageControls>
      </Card>

      <Show when={resourceIncidentPanel()}>
        {(selection) => {
          const resourceId = selection().resourceId;
          const incidents = () => resourceIncidents()[resourceId] || [];
          const isLoading = () => resourceIncidentLoading()[resourceId];
          return (
            <Card padding="md">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <h3 class="text-sm font-semibold text-base-content">
                    {getAlertResourceIncidentPanelTitle()}
                  </h3>
                  <p class="text-xs text-muted">
                    {selection().resourceName}
                    <Show when={incidents().length > 0}>
                      <span>
                        {' '}
                        · {getAlertResourceIncidentCountLabel(incidents().length)}
                      </span>
                    </Show>
                  </p>
                </div>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    class="px-2 py-1 text-xs border rounded-md border-border text-muted hover:bg-surface-hover disabled:opacity-50"
                    disabled={isLoading()}
                    onClick={() => {
                      void refreshResourceIncidentPanel();
                    }}
                  >
                    {getAlertResourceIncidentRefreshLabel(isLoading())}
                  </button>
                  <button
                    type="button"
                    class="px-2 py-1 text-xs border rounded-md border-border text-muted hover:bg-surface-hover"
                    onClick={() => setResourceIncidentPanel(null)}
                  >
                    Close
                  </button>
                </div>
              </div>
              <Show when={isLoading()}>
                <p class="mt-2 text-xs text-muted">{getAlertResourceIncidentLoadingState().text}</p>
              </Show>
              <Show when={!isLoading()}>
                <Show when={incidents().length > 0}>
                  <div class="mt-2">
                    <IncidentEventFilters
                      filters={resourceIncidentEventFilters}
                      setFilters={setResourceIncidentEventFilters}
                      variant="compact"
                      showQuickSelection
                    />
                  </div>
                </Show>
                <Show
                  when={incidents().length > 0}
                  fallback={
                    <p class="mt-2 text-xs text-muted">
                      {getAlertResourceIncidentEmptyState().text}
                    </p>
                  }
                >
                  <div class="mt-3 space-y-3">
                    <For each={incidents()}>
                      {(incident) => {
                        const statusPresentation = getAlertIncidentStatusPresentation(
                          incident.status,
                          incident.acknowledged,
                        );
                        const isExpanded = expandedResourceIncidentIds().has(incident.id);
                        const events = incident.events || [];
                        const filteredEvents = filterIncidentEvents(
                          events,
                          resourceIncidentEventFilters(),
                        );
                        const eventSummary = summarizeIncidentEvents(filteredEvents);
                        const recentEvents =
                          filteredEvents.length > 6
                            ? filteredEvents.slice(filteredEvents.length - 6)
                            : filteredEvents;
                        const lastEvent =
                          filteredEvents.length > 0
                            ? filteredEvents[filteredEvents.length - 1]
                            : undefined;
                        const filteredLabel =
                          filteredEvents.length !== events.length
                            ? `${filteredEvents.length}/${events.length}`
                            : `${events.length}`;
                        return (
                          <div class={getAlertResourceIncidentCardClass()}>
                            <div class={getAlertIncidentTimelineMetaRowClass()}>
                              <span class={getAlertIncidentTimelineHeadingClass()}>
                                {incident.alertType}
                              </span>
                              <span class={getAlertIncidentLevelBadgeClass(incident.level)}>
                                {incident.level}
                              </span>
                              <span class={statusPresentation.className}>
                                {statusPresentation.label}
                              </span>
                              <span>opened {new Date(incident.openedAt).toLocaleString()}</span>
                              <Show when={incident.closedAt}>
                                <span>
                                  closed {new Date(incident.closedAt as string).toLocaleString()}
                                </span>
                              </Show>
                            </div>
                            <Show when={incident.message}>
                              <p class={getAlertIncidentTimelineOutputClass()}>{incident.message}</p>
                            </Show>
                            <Show when={incident.acknowledged && incident.ackUser}>
                              <p class={getAlertIncidentTimelineOutputClass()}>
                                {getAlertResourceIncidentAcknowledgedByLabel(
                                  incident.ackUser ?? '',
                                )}
                              </p>
                            </Show>
                            <Show when={events.length > 0}>
                              <div class={getAlertResourceIncidentSummaryRowClass()}>
                                <Show
                                  when={filteredEvents.length > 0}
                                  fallback={
                                    <span>
                                      {getAlertResourceIncidentFilteredEventsEmptyState().text}
                                    </span>
                                  }
                                >
                                  <div class={getAlertResourceIncidentActivitySummaryClass()}>
                                    <span class="text-[10px] font-medium uppercase tracking-wide text-muted">
                                      Activity
                                    </span>
                                    <For each={eventSummary}>
                                      {(summary) => (
                                        <span class={getAlertResourceIncidentActivityChipClass()}>
                                          {summary.label} {summary.count}
                                        </span>
                                      )}
                                    </For>
                                    <span>
                                      {filteredEvents.length !== events.length
                                        ? `${filteredEvents.length}/${events.length} events`
                                        : `${events.length} event${events.length === 1 ? '' : 's'}`}
                                    </span>
                                    <Show when={lastEvent}>
                                      <span>Latest: {lastEvent?.summary}</span>
                                    </Show>
                                  </div>
                                </Show>
                                <button
                                  type="button"
                                  class={getAlertResourceIncidentToggleButtonClass()}
                                  onClick={() => toggleResourceIncidentDetails(incident.id)}
                                >
                                  {getAlertResourceIncidentToggleLabel(isExpanded, filteredLabel)}
                                </button>
                              </div>
                            </Show>
                            <Show when={isExpanded}>
                              <div class="mt-2 space-y-2">
                                <Show
                                  when={filteredEvents.length > 0}
                                  fallback={
                                    <p class="text-[10px] text-muted">
                                      {getAlertResourceIncidentFilteredEventsEmptyState().text}
                                    </p>
                                  }
                                >
                                  <For each={recentEvents}>
                                    {(event) => (
                                      <IncidentTimelineEventCard event={event} variant="alt" />
                                    )}
                                  </For>
                                  <Show when={filteredEvents.length > 0}>
                                    <p class="text-[10px] text-muted">
                                      {getAlertResourceIncidentTruncatedEventsLabel(
                                        recentEvents.length,
                                        filteredEvents.length,
                                      )}
                                    </p>
                                  </Show>
                                </Show>
                              </div>
                            </Show>
                          </div>
                        );
                      }}
                    </For>
                  </div>
                </Show>
              </Show>
            </Card>
          );
        }}
      </Show>

      <Show
        when={loading()}
        fallback={
          <Show
            when={alertData().length > 0}
            fallback={
              <div class="py-12 text-center text-muted">
                <p class="text-sm">{getAlertHistoryEmptyState().title}</p>
                <p class="mt-1 text-xs">{getAlertHistoryEmptyState().description}</p>
              </div>
            }
          >
            <div class="mb-2 overflow-hidden rounded border border-border">
              <div class="overflow-x-auto">
                <Table class="w-full min-w-[max-content] text-[11px] sm:text-sm">
                  <TableHeader>
                    <TableRow class="border-b border-border bg-surface-hover text-muted">
                      <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Timestamp
                      </TableHead>
                      <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Source
                      </TableHead>
                      <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Resource
                      </TableHead>
                      <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        {getTypeColumnLabel()}
                      </TableHead>
                      <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Severity
                      </TableHead>
                      <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Message
                      </TableHead>
                      <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Duration
                      </TableHead>
                      <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Status
                      </TableHead>
                      <TableHead class="p-1 px-1 text-left text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Node
                      </TableHead>
                      <TableHead class="p-1 px-1 text-center text-[10px] font-medium uppercase tracking-wider sm:p-1.5 sm:px-2 sm:text-xs">
                        Actions
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <For each={groupedAlerts()}>
                      {(group) => (
                        <>
                          <TableRow class="bg-surface-alt">
                            <TableCell
                              colspan={10}
                              class="py-1.5 pr-3 pl-4 text-[12px] font-semibold sm:text-sm"
                            >
                              <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
                                <span class="truncate" title={group.fullLabel}>
                                  {group.label}
                                </span>
                                <span class="text-[10px] font-medium text-muted">
                                  {(() => {
                                    const alertCount = group.alerts.filter(
                                      (alert) => alert.source === 'alert',
                                    ).length;
                                    const aiCount = group.alerts.filter(
                                      (alert) => alert.source === 'ai',
                                    ).length;
                                    const parts = [];
                                    if (alertCount > 0) {
                                      parts.push(
                                        `${alertCount} alert${alertCount === 1 ? '' : 's'}`,
                                      );
                                    }
                                    if (aiCount > 0) {
                                      parts.push(
                                        `${aiCount} patrol insight${aiCount === 1 ? '' : 's'}`,
                                      );
                                    }
                                    return (
                                      parts.join(', ') ||
                                      `${group.alerts.length} item${group.alerts.length === 1 ? '' : 's'}`
                                    );
                                  })()}
                                </span>
                              </div>
                            </TableCell>
                          </TableRow>

                          <For each={group.alerts}>
                            {(alert) => {
                              const rowKey = getIncidentRowKey(alert);
                              const historyStatusPresentation = getAlertHistoryStatusPresentation(
                                alert.status,
                              );
                              const sourcePresentation = getAlertHistorySourcePresentation(
                                alert.source,
                              );
                              return (
                                <>
                                  <TableRow
                                    class={`border-b border-border hover:bg-surface-hover ${historyStatusPresentation.rowClassName}`}
                                  >
                                    <TableCell class="p-1 px-1 font-mono whitespace-nowrap text-muted sm:p-1.5 sm:px-2">
                                      {new Date(alert.startTime).toLocaleTimeString('en-US', {
                                        hour: '2-digit',
                                        minute: '2-digit',
                                      })}
                                    </TableCell>

                                    <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
                                      <span class={sourcePresentation.className}>
                                        {sourcePresentation.label}
                                      </span>
                                    </TableCell>

                                    <TableCell class="max-w-[150px] truncate p-1 px-1 font-medium text-base-content sm:p-1.5 sm:px-2">
                                      {alert.resourceName}
                                    </TableCell>

                                    <TableCell class="p-1 px-1 sm:p-1.5 sm:px-2">
                                      <span class={getAlertHistoryResourceTypeBadgeClass(alert.resourceType)}>
                                        {alert.resourceType}
                                      </span>
                                    </TableCell>

                                    <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
                                      <span class={getAlertIncidentLevelBadgeClass(alert.severity)}>
                                        {alert.severity}
                                      </span>
                                    </TableCell>

                                    <TableCell
                                      class="max-w-[300px] truncate p-1 px-1 text-base-content sm:p-1.5 sm:px-2"
                                      title={alert.description}
                                    >
                                      {alert.description}
                                    </TableCell>

                                    <TableCell class="p-1 px-1 text-center text-muted sm:p-1.5 sm:px-2">
                                      {alert.duration}
                                    </TableCell>

                                    <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
                                      <span class={historyStatusPresentation.className}>
                                        {historyStatusPresentation.label}
                                      </span>
                                    </TableCell>

                                    <TableCell class="truncate p-1 px-1 text-muted sm:p-1.5 sm:px-2">
                                      {alert.nodeDisplayName || alert.node || '—'}
                                    </TableCell>

                                    <TableCell class="p-1 px-1 text-center sm:p-1.5 sm:px-2">
                                      <div class="flex items-center justify-center gap-1">
                                        <Show when={alert.source === 'alert'}>
                                          <button
                                            type="button"
                                            class="rounded-md border border-border px-2 py-1 text-[10px] text-muted hover:bg-surface-hover"
                                            onClick={() => {
                                              void toggleIncidentTimeline(
                                                rowKey,
                                                alert.id,
                                                alert.startTime,
                                              );
                                            }}
                                          >
                                            {expandedIncidents().has(rowKey) ? 'Hide' : 'Timeline'}
                                          </button>
                                        </Show>
                                        <Show when={alert.source === 'alert' && alert.resourceId}>
                                          <button
                                            type="button"
                                            class="rounded-md border border-border px-2 py-1 text-[10px] text-muted hover:bg-surface-hover"
                                            title={getAlertResourceIncidentViewTitle()}
                                            onClick={() => {
                                              void openResourceIncidentPanel(
                                                alert.resourceId as string,
                                                alert.resourceName,
                                              );
                                            }}
                                          >
                                            Resource
                                          </button>
                                        </Show>
                                        <Show
                                          when={
                                            alert.source === 'alert' &&
                                            (alert.status === 'active' ||
                                              alert.status === 'acknowledged')
                                          }
                                        >
                                          <InvestigateAlertButton
                                            alert={{
                                              id: alert.id,
                                              type: alert.rawAlertType || alert.title,
                                              level: alert.severity as 'warning' | 'critical',
                                              resourceId: alert.resourceId || '',
                                              resourceName: alert.resourceName,
                                              node: alert.node || '',
                                              nodeDisplayName: alert.nodeDisplayName,
                                              instance: '',
                                              message: alert.description || '',
                                              value: 0,
                                              threshold: 0,
                                              startTime: alert.startTime,
                                              lastSeen: alert.startTime,
                                              acknowledged: alert.status === 'acknowledged',
                                            }}
                                            resourceType={alert.resourceType}
                                            variant="icon"
                                            size="sm"
                                            licenseLocked={
                                              !props.hasAIAlertsFeature() && !props.licenseLoading()
                                            }
                                          />
                                        </Show>
                                      </div>
                                    </TableCell>
                                  </TableRow>
                                  <Show
                                    when={
                                      alert.source === 'alert' && expandedIncidents().has(rowKey)
                                    }
                                  >
                                    <TableRow class="border-b border-border bg-surface-alt">
                                      <TableCell colspan={11} class="p-3">
                                        <IncidentTimelinePanel
                                          loading={incidentLoading()[rowKey]}
                                          error={incidentErrors()[rowKey]}
                                          timeline={incidentTimelines()[rowKey]}
                                          filters={historyIncidentEventFilters}
                                          setFilters={setHistoryIncidentEventFilters}
                                          filterVariant="compact"
                                          eventCardVariant="surface"
                                          noteDraft={incidentNoteDrafts()[rowKey] || ''}
                                          onNoteDraftChange={(value) =>
                                            setIncidentNoteDraft(rowKey, value)
                                          }
                                          noteSaving={incidentNoteSaving().has(rowKey)}
                                          onSaveNote={() => {
                                            void saveIncidentNote(rowKey, alert.id, alert.startTime);
                                          }}
                                          onRetry={() => {
                                            void loadIncidentTimeline(
                                              rowKey,
                                              alert.id,
                                              alert.startTime,
                                            );
                                          }}
                                        />
                                      </TableCell>
                                    </TableRow>
                                  </Show>
                                </>
                              );
                            }}
                          </For>
                        </>
                      )}
                    </For>
                  </TableBody>
                </Table>
              </div>
            </div>
          </Show>
        }
      >
        <div class="py-12 text-center text-muted">
          <p class="text-sm">{getAlertHistoryLoadingState().text}</p>
        </div>
      </Show>

      <Show when={alertHistory().length > 0}>
        <div class="mt-8 border-t border-border pt-6">
          <div class="rounded-md bg-surface-alt p-4">
            <div class="flex items-start justify-between">
              <div>
                <h4 class="mb-1 text-sm font-medium text-base-content">
                  {getAlertAdministrationSectionTitle()}
                </h4>
                <p class="text-xs text-muted">{getAlertAdministrationSectionDescription()}</p>
              </div>
              <button
                type="button"
                onClick={async () => {
                  if (confirm(getAlertAdministrationClearHistoryConfirmation())) {
                    try {
                      await AlertsAPI.clearHistory();
                      setAlertHistory([]);
                    } catch (error) {
                      logger.error(getAlertAdministrationClearHistoryError(), error);
                      notificationStore.error(getAlertAdministrationClearHistoryError());
                    }
                  }
                }}
                class="flex-shrink-0 rounded-md border border-red-300 px-3 py-2 text-xs text-red-600 transition-colors hover:bg-red-50 dark:border-red-600 dark:text-red-400 dark:hover:bg-red-900"
              >
                {getAlertAdministrationClearHistoryLabel()}
              </button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
}
