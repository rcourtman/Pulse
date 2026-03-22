import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';

import { alertTypeDisplayLabel, unifiedTypeToAlertDisplayType } from './helpers';

export const MS_PER_HOUR = 60 * 60 * 1000;

export type AlertHistoryRange = '24h' | '7d' | '30d' | 'all';
export type AlertSeverityFilter = 'all' | 'warning' | 'critical';

export interface HistoryItem {
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

export interface AlertTrendSeries {
  buckets: number[];
  max: number;
  bucketSize: number;
  bucketTimes: number[];
  rangeStart: number;
  rangeHours: number;
}

export interface AlertAxisTick {
  position: number;
  label: string;
  align: 'start' | 'center' | 'end';
}

export function buildAlertHistoryParams(range: AlertHistoryRange, now = Date.now()) {
  const params: { limit?: number; startTime?: string } = {};

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
}

export function formatAlertHistoryDuration(
  startTime: string,
  endTime?: string,
  now = Date.now(),
) {
  const start = new Date(startTime).getTime();
  const end = endTime ? new Date(endTime).getTime() : now;
  const duration = end - start;

  if (duration < 0) return '0m';

  const minutes = Math.floor(duration / 60000);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) return `${days}d ${hours % 24}h`;
  if (hours > 0) return `${hours}h ${minutes % 60}m`;
  return `${minutes}m`;
}

export function formatAlertBucketRange(startMs: number, endMs: number, locale: string) {
  const start = new Date(startMs);
  const end = new Date(endMs);
  const sameDay =
    start.getFullYear() === end.getFullYear() &&
    start.getMonth() === end.getMonth() &&
    start.getDate() === end.getDate();

  const startDay = start.toLocaleDateString(locale, {
    month: 'short',
    day: 'numeric',
    year: start.getFullYear() !== end.getFullYear() ? 'numeric' : undefined,
  });
  const endDay = end.toLocaleDateString(locale, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
  const timeFormatter: Intl.DateTimeFormatOptions = {
    hour: 'numeric',
    minute: '2-digit',
  };

  const startTimeStr = start.toLocaleTimeString(locale, timeFormatter);
  const endTimeStr = end.toLocaleTimeString(locale, timeFormatter);

  if (sameDay) {
    return `${startDay}, ${startTimeStr} – ${endTimeStr}`;
  }

  return `${startDay}, ${startTimeStr} → ${endDay}, ${endTimeStr}`;
}

export function resolveAlertHistoryResourceType({
  resourceName,
  metadata,
  resourceId,
  getResource,
  allResources,
}: {
  resourceName: string;
  metadata?: Record<string, unknown>;
  resourceId?: string;
  getResource: (resourceId: string) => Resource | undefined;
  allResources: Resource[];
}) {
  const metadataType =
    typeof metadata?.resourceType === 'string' ? (metadata.resourceType as string) : undefined;
  if (metadataType && metadataType.trim().length > 0) {
    return metadataType;
  }

  if (resourceId) {
    const resource = getResource(resourceId);
    if (resource) {
      return unifiedTypeToAlertDisplayType(resource.type);
    }
  }

  const byName = allResources.find(
    (resource) => resource.name === resourceName || resource.displayName === resourceName,
  );
  if (byName) {
    return unifiedTypeToAlertDisplayType(byName.type);
  }

  return 'Unknown';
}

export function buildAlertHistoryItems({
  activeAlerts,
  alertHistory,
  getResource,
  allResources,
  now = Date.now(),
}: {
  activeAlerts: Record<string, Alert>;
  alertHistory: Alert[];
  getResource: (resourceId: string) => Resource | undefined;
  allResources: Resource[];
  now?: number;
}) {
  const items: HistoryItem[] = [];

  Object.values(activeAlerts).forEach((alert) => {
    items.push({
      id: alert.id,
      source: 'alert',
      status: 'active',
      startTime: alert.startTime,
      duration: formatAlertHistoryDuration(alert.startTime, undefined, now),
      resourceName: alert.resourceName,
      resourceType: resolveAlertHistoryResourceType({
        resourceName: alert.resourceName,
        metadata: alert.metadata,
        resourceId: alert.resourceId,
        getResource,
        allResources,
      }),
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
  alertHistory.forEach((alert) => {
    if (activeAlertIds.has(alert.id)) return;

    items.push({
      id: alert.id,
      source: 'alert',
      status: alert.acknowledged ? 'acknowledged' : 'resolved',
      startTime: alert.startTime,
      endTime: alert.lastSeen,
      duration: formatAlertHistoryDuration(alert.startTime, alert.lastSeen, now),
      resourceName: alert.resourceName,
      resourceType: resolveAlertHistoryResourceType({
        resourceName: alert.resourceName,
        metadata: alert.metadata,
        resourceId: alert.resourceId,
        getResource,
        allResources,
      }),
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
}

export function filterAlertHistoryItems(
  items: HistoryItem[],
  severityFilter: AlertSeverityFilter,
  searchTerm: string,
) {
  let filtered = items;

  if (severityFilter !== 'all') {
    filtered = filtered.filter((item) => item.severity === severityFilter);
  }

  const normalizedSearch = searchTerm.trim().toLowerCase();
  if (normalizedSearch) {
    filtered = filtered.filter((item) => {
      const name = item.resourceName?.toLowerCase() ?? '';
      const title = item.title?.toLowerCase() ?? '';
      const description = item.description?.toLowerCase() ?? '';
      const nodeName = item.node?.toLowerCase() ?? '';
      return (
        name.includes(normalizedSearch) ||
        title.includes(normalizedSearch) ||
        description.includes(normalizedSearch) ||
        nodeName.includes(normalizedSearch)
      );
    });
  }

  return filtered;
}

export function buildAlertTrends(
  filteredAlerts: HistoryItem[],
  timeFilter: AlertHistoryRange,
  now = Date.now(),
): AlertTrendSeries {
  const niceBucketSizes = [1, 2, 3, 6, 12, 24, 48, 72, 168, 336, 720, 1440];
  const maxBuckets = 30;

  let bucketSizeHours: number;
  let computedRangeHours: number;
  let startTime: number;

  if (timeFilter === '24h') {
    bucketSizeHours = 1;
    computedRangeHours = 24;
    startTime = now - computedRangeHours * MS_PER_HOUR;
  } else if (timeFilter === '7d') {
    bucketSizeHours = 6;
    computedRangeHours = 7 * 24;
    startTime = now - computedRangeHours * MS_PER_HOUR;
  } else if (timeFilter === '30d') {
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
}

export function applyAlertHistoryWindow({
  filteredItems,
  timeFilter,
  selectedBarIndex,
  trends,
  now = Date.now(),
}: {
  filteredItems: HistoryItem[];
  timeFilter: AlertHistoryRange;
  selectedBarIndex: number | null;
  trends: AlertTrendSeries;
  now?: number;
}) {
  let filtered = filteredItems;

  if (selectedBarIndex !== null) {
    const bucketStart = trends.bucketTimes[selectedBarIndex];
    const bucketEnd = bucketStart + trends.bucketSize * MS_PER_HOUR;

    filtered = filtered.filter((alert) => {
      const alertTime = new Date(alert.startTime).getTime();
      return alertTime >= bucketStart && alertTime < bucketEnd;
    });
  } else if (timeFilter !== 'all') {
    const cutoffMap: Record<'24h' | '7d' | '30d', number> = {
      '24h': now - 24 * 60 * 60 * 1000,
      '7d': now - 7 * 24 * 60 * 60 * 1000,
      '30d': now - 30 * 24 * 60 * 60 * 1000,
    };
    const cutoff = cutoffMap[timeFilter];
    if (cutoff) {
      filtered = filtered.filter((alert) => new Date(alert.startTime).getTime() > cutoff);
    }
  }

  return [...filtered].sort(
    (a, b) => new Date(b.startTime).getTime() - new Date(a.startTime).getTime(),
  );
}

const MONTH_NAMES = [
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

export function getAlertHistoryDaySuffix(day: number) {
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
}

export function formatAlertHistoryGroupLabel(date: Date, todayStart: number, yesterdayStart: number) {
  const month = MONTH_NAMES[date.getMonth()];
  const day = date.getDate();
  const suffix = getAlertHistoryDaySuffix(day);
  const absoluteDate = `${month} ${day}${suffix}`;

  if (date.getTime() === todayStart) return `Today (${absoluteDate})`;
  if (date.getTime() === yesterdayStart) return `Yesterday (${absoluteDate})`;
  return absoluteDate;
}

export function getIncidentRowKey(alert: HistoryItem) {
  return `${alert.id}::${alert.startTime}`;
}

export function groupAlertHistoryItems(alertData: HistoryItem[]) {
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

  alertData.forEach((alert) => {
    const alertDate = new Date(alert.startTime);
    const dateOnly = new Date(alertDate.getFullYear(), alertDate.getMonth(), alertDate.getDate());
    const dateKey = dateOnly.getTime();

    if (!groups.has(dateKey)) {
      groups.set(dateKey, {
        date: dateOnly,
        label: formatAlertHistoryGroupLabel(dateOnly, todayStart, yesterdayStart),
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
}

export function getAlertBucketDurationLabel(bucketHours: number) {
  if (!Number.isFinite(bucketHours) || bucketHours <= 0) return '—';
  if (bucketHours % 24 === 0) {
    const days = bucketHours / 24;
    return `${days} day${days === 1 ? '' : 's'}`;
  }
  return `${bucketHours} hour${bucketHours === 1 ? '' : 's'}`;
}

export function formatAlertAxisTickLabel({
  timestamp,
  bucketHours,
  totalHours,
  locale,
  isEnd = false,
  now = Date.now(),
}: {
  timestamp: number;
  bucketHours: number;
  totalHours: number;
  locale: string;
  isEnd?: boolean;
  now?: number;
}) {
  if (!Number.isFinite(timestamp)) return '—';

  if (isEnd && Math.abs(now - timestamp) < bucketHours * MS_PER_HOUR * 0.75) {
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

  return date.toLocaleString(locale, options);
}

export function buildAlertRangeSummary(trends: AlertTrendSeries, locale: string) {
  if (!trends.bucketTimes.length || trends.bucketSize <= 0) return null;

  const bucketHours = trends.bucketSize;
  const totalHours = Math.max(trends.rangeHours ?? bucketHours, bucketHours);
  const start = trends.bucketTimes[0];
  const end = start + trends.buckets.length * bucketHours * MS_PER_HOUR;

  return {
    startLabel: formatAlertAxisTickLabel({
      timestamp: start,
      bucketHours,
      totalHours,
      locale,
    }),
    endLabel: formatAlertAxisTickLabel({
      timestamp: end,
      bucketHours,
      totalHours,
      locale,
      isEnd: true,
    }),
  };
}

export function buildAlertAxisTicks(trends: AlertTrendSeries, locale: string): AlertAxisTick[] {
  if (!trends.bucketTimes.length || trends.bucketSize <= 0) {
    return [];
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
      label: formatAlertAxisTickLabel({
        timestamp: ts,
        bucketHours,
        totalHours,
        locale,
      }),
    });
  }

  if (!ticks.length || ticks[0].position > 0.01) {
    ticks.unshift({
      position: 0,
      label: formatAlertAxisTickLabel({ timestamp: start, bucketHours, totalHours, locale }),
    });
  } else {
    ticks[0] = {
      position: 0,
      label: formatAlertAxisTickLabel({ timestamp: start, bucketHours, totalHours, locale }),
    };
  }

  const lastTick = ticks[ticks.length - 1];
  if (!lastTick || Math.abs(lastTick.position - 1) > 0.01) {
    ticks.push({
      position: 1,
      label: formatAlertAxisTickLabel({
        timestamp: end,
        bucketHours,
        totalHours,
        locale,
        isEnd: true,
      }),
    });
  } else {
    ticks[ticks.length - 1] = {
      position: 1,
      label: formatAlertAxisTickLabel({
        timestamp: end,
        bucketHours,
        totalHours,
        locale,
        isEnd: true,
      }),
    };
  }

  return ticks.map((tick, index, array) => ({
    position: tick.position,
    label: tick.label,
    align: index === 0 ? 'start' : index === array.length - 1 ? 'end' : 'center',
  }));
}

export function buildSelectedBucketDetails(
  selectedBarIndex: number | null,
  trends: AlertTrendSeries,
  locale: string,
) {
  if (selectedBarIndex === null) return null;

  const bucketStart = trends.bucketTimes[selectedBarIndex];
  const bucketEnd = bucketStart + trends.bucketSize * MS_PER_HOUR;
  return {
    rangeLabel: formatAlertBucketRange(bucketStart, bucketEnd, locale),
    start: bucketStart,
    end: bucketEnd,
  };
}
