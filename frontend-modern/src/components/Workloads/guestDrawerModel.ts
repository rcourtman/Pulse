import type { WorkloadGuest } from '@/types/workloads';
import type {
  AggregatedMetricPoint,
  HistoryTimeRange,
  ResourceType as HistoryResourceType,
} from '@/api/charts';

import { formatHistoryChartTooltipValue } from '@/components/shared/historyChartModel';
import { formatBytes, formatPercent } from '@/utils/format';
import { getCanonicalWorkloadId, resolveWorkloadType } from '@/utils/workloads';

type Guest = WorkloadGuest;

export interface GuestDrawerProps {
  guest: Guest;
  onClose: () => void;
  customUrl?: string;
  onCustomUrlChange?: (guestId: string, url: string) => void;
}

export type GuestDrawerTab = 'overview' | 'history' | 'discovery';

export interface GuestDrawerHistoryTarget {
  resourceType: HistoryResourceType;
  resourceId: string;
}

export interface GuestDrawerHistoryChartConfig {
  metric: string;
  label: string;
  unit: string;
  color: string;
}

export interface GuestDrawerHistoryGroupConfig {
  id: string;
  label: string;
  unit: string;
  series: GuestDrawerHistoryChartConfig[];
}

export interface GuestDrawerHistoryScale {
  minValue: number;
  maxValue: number;
}

export interface GuestDrawerBackupPresentation {
  ageClass: string;
  ageLabel: string;
  dateLabel: string;
}

export const isGuestDrawerVM = (guest: Guest): boolean => resolveWorkloadType(guest) === 'vm';

// Fallback current-value metrics for the guest drawer's history charts.
// Mirrors `getNodeDrawerHistoryFallbackMetrics` — supplies a single
// finite value per metric so `mergeFallbackHistoryMetrics` can synthesize
// a flat 2-point line, replacing the "Collecting history" placeholder
// when the metrics-store hasn't yet accumulated 2+ samples for that
// resource within the selected range. Keys must match the `metric`
// strings declared in `GUEST_DRAWER_HISTORY_GROUPS`.
export const getGuestDrawerHistoryFallbackMetrics = (
  guest: Guest,
): Record<string, number | undefined> => {
  const cpuRaw = typeof guest.cpu === 'number' ? guest.cpu : undefined;
  const cpuPercent =
    cpuRaw === undefined || !Number.isFinite(cpuRaw)
      ? undefined
      : cpuRaw <= 1.5
        ? cpuRaw * 100
        : cpuRaw;
  const memUsage = guest.memory?.usage;
  const diskUsage = guest.disk?.usage;
  const finite = (value: number | undefined): number | undefined =>
    typeof value === 'number' && Number.isFinite(value) ? value : undefined;
  return {
    cpu: finite(cpuPercent),
    memory: finite(memUsage),
    disk: finite(diskUsage),
    netin: finite(guest.networkIn),
    netout: finite(guest.networkOut),
    diskread: finite(guest.diskRead),
    diskwrite: finite(guest.diskWrite),
  };
};

export const GUEST_DRAWER_HISTORY_DEFAULT_RANGE: HistoryTimeRange = '24h';

export const GUEST_DRAWER_HISTORY_GROUPS: GuestDrawerHistoryGroupConfig[] = [
  {
    id: 'utilization',
    label: 'Utilization',
    unit: '%',
    series: [
      { metric: 'cpu', label: 'CPU', unit: '%', color: '#8b5cf6' },
      { metric: 'memory', label: 'Memory', unit: '%', color: '#f59e0b' },
      { metric: 'disk', label: 'Disk', unit: '%', color: '#10b981' },
    ],
  },
  {
    id: 'network',
    label: 'Network I/O',
    unit: 'B/s',
    series: [
      { metric: 'netin', label: 'In', unit: 'B/s', color: '#10b981' },
      { metric: 'netout', label: 'Out', unit: 'B/s', color: '#fb923c' },
    ],
  },
  {
    id: 'disk-io',
    label: 'Disk I/O',
    unit: 'B/s',
    series: [
      { metric: 'diskread', label: 'Read', unit: 'B/s', color: '#3b82f6' },
      { metric: 'diskwrite', label: 'Write', unit: 'B/s', color: '#f59e0b' },
    ],
  },
];

const clampHistoryPointValue = (value: number, unit: string): number => {
  if (!Number.isFinite(value)) return 0;
  const nonNegative = Math.max(0, value);
  return unit === '%' ? Math.min(100, nonNegative) : nonNegative;
};

export const normalizeGuestDrawerHistoryPoints = (
  points: AggregatedMetricPoint[] | undefined,
  unit: string,
): AggregatedMetricPoint[] =>
  (points ?? [])
    .filter((point) => Number.isFinite(point.timestamp) && Number.isFinite(point.value))
    .map((point) => {
      const value = clampHistoryPointValue(point.value, unit);
      return {
        ...point,
        value,
        min:
          typeof point.min === 'number' && Number.isFinite(point.min)
            ? clampHistoryPointValue(point.min, unit)
            : value,
        max:
          typeof point.max === 'number' && Number.isFinite(point.max)
            ? clampHistoryPointValue(point.max, unit)
            : value,
      };
    })
    .sort((a, b) => a.timestamp - b.timestamp);

export const getGuestDrawerHistoryScale = (
  series: readonly { points: readonly AggregatedMetricPoint[] }[],
  unit: string,
): GuestDrawerHistoryScale => {
  if (unit === '%') return { minValue: 0, maxValue: 100 };

  if (unit === 'C') {
    let minValue = Infinity;
    let maxValue = -Infinity;
    for (const item of series) {
      for (const point of item.points) {
        const low = typeof point.min === 'number' ? point.min : point.value;
        const high = typeof point.max === 'number' ? point.max : point.value;
        if (Number.isFinite(low) && low < minValue) {
          minValue = low;
        }
        if (Number.isFinite(high) && high > maxValue) {
          maxValue = high;
        }
      }
    }

    if (!Number.isFinite(minValue) || !Number.isFinite(maxValue)) {
      return { minValue: 0, maxValue: 100 };
    }
    if (minValue === maxValue) {
      return {
        minValue: Math.max(0, minValue - 5),
        maxValue: maxValue + 5,
      };
    }
    const padding = Math.max(2, (maxValue - minValue) * 0.15);
    return {
      minValue: Math.max(0, minValue - padding),
      maxValue: maxValue + padding,
    };
  }

  let maxValue = 0;
  for (const item of series) {
    for (const point of item.points) {
      const value = typeof point.max === 'number' ? point.max : point.value;
      if (Number.isFinite(value) && value > maxValue) {
        maxValue = value;
      }
    }
  }

  return { minValue: 0, maxValue: Math.max(1, maxValue * 1.15) };
};

export const buildGuestDrawerHistoryPath = (
  points: readonly AggregatedMetricPoint[],
  scale: GuestDrawerHistoryScale,
  startTime: number,
  endTime: number,
  width = 360,
  height = 92,
): string => {
  if (points.length < 2) return '';

  const left = 34;
  const right = 8;
  const top = 8;
  const bottom = 18;
  const plotWidth = width - left - right;
  const plotHeight = height - top - bottom;
  const timeSpan = Math.max(1, endTime - startTime);
  const valueSpan = Math.max(1, scale.maxValue - scale.minValue);

  return points
    .map((point, index) => {
      const x = left + ((point.timestamp - startTime) / timeSpan) * plotWidth;
      const bounded = Math.min(Math.max(point.value, scale.minValue), scale.maxValue);
      const y = top + (1 - (bounded - scale.minValue) / valueSpan) * plotHeight;
      return `${index === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(' ');
};

export const getGuestDrawerHistoryValueLabel = (
  points: readonly AggregatedMetricPoint[],
  unit: string,
): string => {
  const latest = points[points.length - 1];
  if (!latest) return '-';
  return formatHistoryChartTooltipValue(latest.value, unit);
};

export const getGuestDrawerHistoryRangeBounds = (
  groupedSeries: readonly { points: readonly AggregatedMetricPoint[] }[],
): { startTime: number; endTime: number } | null => {
  const timestamps = groupedSeries.flatMap((item) => item.points.map((point) => point.timestamp));
  if (timestamps.length === 0) return null;
  return {
    startTime: Math.min(...timestamps),
    endTime: Math.max(...timestamps),
  };
};

export const getGuestDrawerHistoryTarget = (guest: Guest): GuestDrawerHistoryTarget | null => {
  const resourceId = getCanonicalWorkloadId(guest).trim();
  if (!resourceId) return null;

  const workloadType = resolveWorkloadType(guest);
  switch (workloadType) {
    case 'vm':
      return { resourceType: 'vm', resourceId };
    case 'system-container':
      return { resourceType: 'system-container', resourceId };
    case 'app-container':
      return { resourceType: 'app-container', resourceId };
    case 'pod':
      return { resourceType: 'pod', resourceId };
    default:
      return null;
  }
};

export const hasGuestDrawerOsInfo = (guest: Guest): boolean =>
  (guest.osName?.length ?? 0) > 0 || (guest.osVersion?.length ?? 0) > 0;

export const getGuestDrawerAgentLabel = (guest: Guest): string => {
  const version = (guest.agentVersion || '').trim();
  if (!version) return '';
  return isGuestDrawerVM(guest) ? `QEMU ${version}` : version;
};

export const getGuestDrawerAgentTitle = (guest: Guest): string => {
  const version = (guest.agentVersion || '').trim();
  if (!version) return '';
  return isGuestDrawerVM(guest) ? `QEMU guest agent ${version}` : version;
};

export interface GuestDrawerMemoryRow {
  label: string;
  value: string;
}

// Memory rows for the guest drawer Overview card. Leads with the primary
// RAM usage (Usage / Total / Free) so the "Memory" card lives up to its title
// and matches the node drawer's memory card, then appends balloon/swap when
// present. The collapsed row only shows the RAM gauge; the drawer is where the
// breakdown belongs.
export const getGuestDrawerMemoryRows = (guest: Guest): GuestDrawerMemoryRow[] => {
  const memory = guest.memory;
  if (!memory) return [];

  const rows: GuestDrawerMemoryRow[] = [];
  const total = memory.total ?? 0;
  const used = memory.used ?? 0;

  if (total > 0) {
    rows.push({ label: 'Usage', value: `${formatPercent((used / total) * 100)} · ${formatBytes(used)}` });
    rows.push({ label: 'Total', value: formatBytes(total) });
    if (typeof memory.free === 'number') {
      rows.push({ label: 'Free', value: formatBytes(memory.free) });
    }
  }

  if (memory.balloon && memory.balloon > 0 && memory.balloon !== total) {
    rows.push({ label: 'Balloon', value: formatBytes(memory.balloon) });
  }

  if (memory.swapTotal && memory.swapTotal > 0) {
    rows.push({
      label: 'Swap',
      value: `${formatBytes(memory.swapUsed ?? 0)} / ${formatBytes(memory.swapTotal)}`,
    });
  }

  return rows;
};

export const hasGuestDrawerFilesystemDetails = (guest: Guest): boolean =>
  Array.isArray(guest.disks) && guest.disks.length > 0;

export const getGuestDrawerNetworkInterfaces = (guest: Guest) => guest.networkInterfaces || [];

export const normalizeGuestDrawerTags = (tags: Guest['tags']): string[] => {
  if (Array.isArray(tags)) {
    return tags.map((tag) => tag.trim()).filter((tag) => tag.length > 0);
  }
  if (typeof tags === 'string') {
    return tags
      .split(',')
      .map((tag) => tag.trim())
      .filter((tag) => tag.length > 0);
  }
  return [];
};

export const getGuestDrawerBackupPresentation = (
  lastBackup: string | number | Date,
  now: Date = new Date(),
): GuestDrawerBackupPresentation => {
  const backupDate = new Date(lastBackup);
  const daysSince = Math.floor((now.getTime() - backupDate.getTime()) / (1000 * 60 * 60 * 24));
  const isOld = daysSince > 7;
  const isCritical = daysSince > 30;

  return {
    ageClass: isCritical
      ? 'text-red-600 dark:text-red-400'
      : isOld
        ? 'text-amber-600 dark:text-amber-400'
        : 'text-green-600 dark:text-green-400',
    ageLabel: daysSince === 0 ? 'Today' : daysSince === 1 ? 'Yesterday' : `${daysSince}d ago`,
    dateLabel: backupDate.toLocaleDateString(),
  };
};
