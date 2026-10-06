import type { WorkloadGuest } from '@/types/workloads';
import type { Alert, VM } from '@/types/api';
import type {
  AggregatedMetricPoint,
  HistoryTimeRange,
  ResourceType as HistoryResourceType,
} from '@/api/charts';

import { formatHistoryChartTooltipValue } from '@/components/shared/historyChartModel';
import {
  getAlertAttentionCopy,
  type AlertAttentionCopy,
} from '@/features/alerts/metricAlertPresentation';
import { formatBytes, formatPercent, getBackupInfo, type BackupThresholds } from '@/utils/format';
import {
  getWorkloadGuestDiskStatusMessage,
  getWorkloadsGuestBackupStatusPresentation,
} from '@/utils/workloadGuestPresentation';
import { getWorkloadCPUPercent, resolveWorkloadType } from '@/utils/workloads';
import { getWorkloadMetricHistoryTarget } from '@/utils/workloadMetricHistoryTarget';
import type { NestedWorkloadContext } from './nestedWorkloadContext';
import type { WorkloadsMemoryDisplayBasis } from './workloadsFilterModel';

import {
  getWorkloadMemoryObservationPresentation,
  type MemoryObservationPresentation,
} from '@/utils/memoryObservation';

type Guest = WorkloadGuest;

export interface GuestDrawerProps {
  guest: Guest;
  onClose: () => void;
  metadataId?: string;
  customUrl?: string;
  onCustomUrlChange?: (guestId: string, url: string) => void;
  parentNodeOnline?: boolean;
  parentMemoryTotal?: number;
  memoryDisplayBasis?: WorkloadsMemoryDisplayBasis;
  nestedWorkloadContext?: NestedWorkloadContext;
  alerts?: Alert[];
}

export type GuestDrawerTab = 'overview' | 'history' | 'discovery' | 'manage';

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

export interface GuestDrawerHistoryTimeBounds {
  startTime: number;
  endTime: number;
}

export interface GuestDrawerHistoryDeferredMetric {
  lastKnownValue?: number;
  valueLabel?: 'freshness unknown';
  message: string;
}

export interface GuestDrawerBackupPresentation {
  ageClass: string;
  ageLabel: string;
  dateLabel: string;
}

export const isGuestDrawerVM = (guest: Guest): guest is VM => resolveWorkloadType(guest) === 'vm';

export const getGuestDrawerAlertAttention = (
  alert: Alert,
  context: Pick<GuestDrawerProps, 'guest' | 'memoryDisplayBasis' | 'parentMemoryTotal'>,
): AlertAttentionCopy => {
  const copy = getAlertAttentionCopy(alert);
  if (context.memoryDisplayBasis !== 'host' || alert.type.trim().toLowerCase() !== 'memory') {
    return copy;
  }

  const used = context.guest.memory?.used;
  const hostTotal = context.parentMemoryTotal;
  const hostShare =
    typeof used === 'number' &&
    Number.isFinite(used) &&
    used >= 0 &&
    typeof hostTotal === 'number' &&
    Number.isFinite(hostTotal) &&
    hostTotal > 0
      ? formatPercent((used / hostTotal) * 100)
      : null;
  const comparison = hostShare
    ? `guest allocation · ${hostShare} of host capacity`
    : 'guest allocation · row uses host capacity';
  return { ...copy, message: `${copy.message} (${comparison})` };
};

const getGuestDrawerDiskUsage = (guest: Guest): number | undefined => {
  const usage = guest.telemetryAvailability?.disk === false ? undefined : guest.disk?.usage;
  return typeof usage === 'number' && Number.isFinite(usage) && usage >= 0 ? usage : undefined;
};

export type GuestDrawerMemoryReading = MemoryObservationPresentation;

const getGuestDrawerMemoryUsage = (guest: Guest): number | undefined => {
  const usage = guest.memory?.usage;
  return guest.telemetryAvailability?.memory !== false &&
    !guest.memory?.usageUnavailable &&
    typeof usage === 'number' &&
    Number.isFinite(usage) &&
    usage >= 0 &&
    usage <= 100
    ? usage
    : undefined;
};

export const getGuestDrawerMemoryReading = getWorkloadMemoryObservationPresentation;

export const getGuestDrawerDeferredMetrics = (
  guest: Guest,
): Record<string, GuestDrawerHistoryDeferredMetric> => {
  const metrics: Record<string, GuestDrawerHistoryDeferredMetric> = {};
  if (isGuestDrawerVM(guest) && guest.diskStatusReason) {
    metrics.disk = {
      lastKnownValue: guest.diskStatusReason.startsWith('prev-')
        ? getGuestDrawerDiskUsage(guest)
        : undefined,
      message: getWorkloadGuestDiskStatusMessage(guest.diskStatusReason),
    };
  }
  const memory = getGuestDrawerMemoryReading(guest);
  if (memory && memory.state !== 'current') {
    metrics.memory = {
      lastKnownValue: memory.state === 'unavailable' ? undefined : getGuestDrawerMemoryUsage(guest),
      ...(memory.state === 'unknown' ? { valueLabel: 'freshness unknown' as const } : {}),
      message: memory.message,
    };
  }
  return metrics;
};

// Current-value metrics displayed beside history legends while the metrics
// store is still accumulating samples. These values never become chart points:
// a current reading is not evidence of a historical trend.
export const getGuestDrawerCurrentMetrics = (guest: Guest): Record<string, number | undefined> => {
  const availability = guest.telemetryAvailability;
  const available = (metric: keyof NonNullable<Guest['telemetryAvailability']>): boolean =>
    availability?.[metric] ?? true;
  const cpuPercent = available('cpu') ? getWorkloadCPUPercent(guest.cpu) : undefined;
  const memoryReading = getGuestDrawerMemoryReading(guest);
  const memUsage =
    available('memory') && (!memoryReading || memoryReading.state === 'current')
      ? getGuestDrawerMemoryUsage(guest)
      : undefined;
  // A paused filesystem read is not current, even if the snapshot still
  // carries a numeric summary. Retained evidence has its own labelled path.
  const diskUsage =
    isGuestDrawerVM(guest) && guest.diskStatusReason ? undefined : getGuestDrawerDiskUsage(guest);
  const finite = (value: number | undefined): number | undefined =>
    typeof value === 'number' && Number.isFinite(value) ? value : undefined;
  return {
    cpu: finite(cpuPercent),
    memory: finite(memUsage),
    disk: diskUsage,
    netin: available('networkIO') ? finite(guest.networkIn) : undefined,
    netout: available('networkIO') ? finite(guest.networkOut) : undefined,
    diskread: available('diskIO') ? finite(guest.diskRead) : undefined,
    diskwrite: available('diskIO') ? finite(guest.diskWrite) : undefined,
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

const isHistoryTimestamp = (timestamp: number): boolean =>
  Number.isFinite(timestamp) && Number.isFinite(new Date(timestamp).getTime());

export const normalizeGuestDrawerHistoryPoints = (
  points: AggregatedMetricPoint[] | undefined,
  unit: string,
): AggregatedMetricPoint[] =>
  (points ?? [])
    .filter((point) => isHistoryTimestamp(point.timestamp) && Number.isFinite(point.value))
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

// Both inputs are normalized stored observations, ordered by timestamp. A
// series cannot span a time where another series in its panel has a reading
// and this one does not. Do not infer failed polls or cadence from elapsed
// time: the API supplies neither. Keep singleton segments as real evidence.
export const getGuestDrawerHistorySegments = (
  points: readonly AggregatedMetricPoint[],
  observationTimes: readonly number[],
): AggregatedMetricPoint[][] => {
  const timeIndices = new Map(observationTimes.map((timestamp, index) => [timestamp, index]));
  const segments: AggregatedMetricPoint[][] = [];
  let previousIndex: number | undefined;
  for (const point of points) {
    const index = timeIndices.get(point.timestamp);
    if (index === undefined) continue;
    if (previousIndex === undefined || (index !== previousIndex && index !== previousIndex + 1)) {
      segments.push([]);
    }
    segments[segments.length - 1].push(point);
    previousIndex = index;
  }
  return segments;
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
  window?: { start: number; end: number },
): GuestDrawerHistoryTimeBounds | null => {
  const timestamps = groupedSeries
    .flatMap((item) => item.points.map((point) => point.timestamp))
    .filter(isHistoryTimestamp);
  const windowBounds =
    window &&
    isHistoryTimestamp(window.start) &&
    isHistoryTimestamp(window.end) &&
    window.end > window.start
      ? { startTime: window.start, endTime: window.end }
      : null;
  if (timestamps.length === 0) return windowBounds;

  // The API's requested window is shared across every panel. Do not stretch
  // a few recent readings across the whole selected range, or scale each
  // metric group to different times. Preserve returned edge observations
  // (including aggregate bucket timestamps) by widening the common envelope.
  const first = Math.min(...timestamps);
  const last = Math.max(...timestamps);
  return {
    startTime: Math.min(windowBounds?.startTime ?? first, first),
    endTime: Math.max(windowBounds?.endTime ?? last, last),
  };
};

export const getGuestDrawerHistoryTarget = (guest: Guest): GuestDrawerHistoryTarget | null => {
  return getWorkloadMetricHistoryTarget(guest);
};

export const hasGuestDrawerOsInfo = (guest: Guest): boolean =>
  (guest.osName?.length ?? 0) > 0 || (guest.osVersion?.length ?? 0) > 0;

export const getGuestDrawerAgentLabel = (guest: Guest): string => {
  const version = (guest.agentVersion || '').trim();
  if (!version) return '';
  if (guest.agentKind === 'pulse') return `Pulse ${version}`;
  return guest.agentKind === 'qemu-guest' ||
    (guest.agentKind === undefined && isGuestDrawerVM(guest))
    ? `QEMU ${version}`
    : version;
};

export const getGuestDrawerAgentTitle = (guest: Guest): string => {
  const version = (guest.agentVersion || '').trim();
  if (!version) return '';
  if (guest.agentKind === 'pulse') return `Pulse Agent ${version}`;
  return guest.agentKind === 'qemu-guest' ||
    (guest.agentKind === undefined && isGuestDrawerVM(guest))
    ? `QEMU guest agent ${version}`
    : version;
};

export const getGuestDrawerAgentHeading = (guest: Guest): string =>
  guest.agentKind === 'pulse' ? 'Pulse Agent' : 'Guest agent';

export interface GuestDrawerGuestReadPresentation {
  label: string;
  detail: string;
  tone: 'muted' | 'warning';
  precaution: boolean;
}

// Both drawer shapes use the same provider-owned signals. A canonical resource
// need not fabricate a complete workload (or infer liveness from metrics).
export interface GuestDrawerGuestReadEvidence extends Pick<
  Guest,
  'type' | 'workloadType' | 'agentKind' | 'platformType' | 'platformScopes'
> {
  diskStatusReason?: VM['diskStatusReason'];
  guestAgentStatus?: VM['guestAgentStatus'];
  lock?: VM['lock'];
  backupInProgress?: VM['backupInProgress'];
}

export const GUEST_DRAWER_BACKUP_PRECAUTION =
  'Do not run live diagnostics or restart the guest agent during a backup, freeze/thaw or an unresponsive-guest incident. An OK backup or a running VM does not prove thaw. Confirm thaw and writes to the filesystems covered by the backup independently.';

const guestReadDeferrals = new Set([
  'vm-locked',
  'lock-unverified',
  'agent-busy',
  'agent-cooldown',
  'agent-response-incomplete',
  'agent-capacity',
  'invalid-guest-key',
  'agent-timeout',
]);

// Version/OS metadata can survive a lock or failed read. Neither it nor a
// parent-agent action target establishes QGA liveness. Use only the current
// provider state and fixed read reasons; never print an unknown raw value.
export const getGuestDrawerGuestReadPresentation = (
  guest: GuestDrawerGuestReadEvidence,
): GuestDrawerGuestReadPresentation | null => {
  if (resolveWorkloadType(guest) !== 'vm') return null;
  const hasProxmoxEvidence =
    guest.type === 'qemu' ||
    guest.agentKind === 'qemu-guest' ||
    guest.platformType === 'proxmox-pve' ||
    guest.platformScopes?.includes('proxmox-pve') ||
    Boolean(guest.guestAgentStatus || guest.diskStatusReason);
  if (!hasProxmoxEvidence) return null;

  const reason = (guest.diskStatusReason || '').replace(/^prev-/, '');
  const readDeferred = guestReadDeferrals.has(reason);
  const state = (guest.guestAgentStatus || '').trim().toLowerCase();
  if ((guest.lock || '').trim()) {
    return {
      label: 'Deferred',
      tone: 'warning',
      precaution: true,
      detail: getWorkloadGuestDiskStatusMessage('vm-locked'),
    };
  }
  if (guest.backupInProgress) {
    return {
      label: 'Backup in progress',
      tone: 'warning',
      precaution: true,
      detail:
        'A backup is reported in progress. Guest-agent availability does not establish safe live checks during the backup.',
    };
  }
  if (state === 'deferred' || (!state && readDeferred)) {
    return {
      label: 'Deferred',
      tone: 'warning',
      precaution: true,
      detail: readDeferred
        ? getWorkloadGuestDiskStatusMessage(reason)
        : 'Guest reads are deferred. Previously observed details do not prove a current connection.',
    };
  }
  const precaution = readDeferred;
  switch (state) {
    case 'available':
      return {
        label: 'Reported available',
        tone: 'muted',
        precaution,
        detail:
          'Proxmox reports guest-agent availability. This does not independently confirm thaw or filesystem writes.',
      };
    case 'expected-unreachable':
      return {
        label: 'Unreachable',
        tone: 'warning',
        precaution,
        detail:
          'Proxmox expects the guest agent but cannot reach it. Previously observed details do not prove a current connection.',
      };
    case 'not-running':
      return {
        label: 'Not running',
        tone: 'warning',
        precaution,
        detail:
          'Proxmox reports that the guest agent is not running. Previously observed details do not prove a current connection.',
      };
    case 'disabled':
      return {
        label: 'Disabled',
        tone: 'muted',
        precaution,
        detail:
          'Proxmox reports that the guest agent is disabled. Previously observed details do not prove a current connection.',
      };
    default:
      return {
        label: 'Unknown',
        tone: 'muted',
        precaution,
        detail:
          'Pulse has no recognised current guest-agent state. Previously observed details do not prove a current connection.',
      };
  }
};

// A read-specific precaution is independent of the reported availability flag.
// For example Proxmox can report QGA enabled while its last request timed out.
export const getGuestDrawerGuestReadPrecaution = (
  guest: GuestDrawerGuestReadEvidence,
): string | null => {
  const presentation = getGuestDrawerGuestReadPresentation(guest);
  if (!presentation?.precaution) return null;
  const reason = (guest.diskStatusReason || '').replace(/^prev-/, '');
  return guestReadDeferrals.has(reason)
    ? getWorkloadGuestDiskStatusMessage(reason)
    : presentation.detail;
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
  const reading = getGuestDrawerMemoryReading(guest);
  const total = memory.total ?? 0;
  const used = memory.used ?? 0;
  const cache = memory.cache ?? 0;

  if (memory.usageUnavailable || reading?.state === 'unavailable') {
    rows.push({ label: 'Usage', value: 'Unavailable' });
    if (total > 0) {
      rows.push({ label: 'Total', value: formatBytes(total) });
    }
  } else if (total > 0) {
    rows.push({
      label: 'Usage',
      value: `${formatPercent((used / total) * 100)} · ${formatBytes(used)}`,
    });
    rows.push({ label: 'Total', value: formatBytes(total) });
    if (cache > 0) {
      rows.push({ label: 'Reclaimable cache', value: formatBytes(cache) });
    }
    if (typeof memory.free === 'number') {
      rows.push({ label: 'Free', value: formatBytes(memory.free) });
    }
  }

  if (memory.balloon && memory.balloon > 0 && memory.balloon !== total) {
    rows.push({ label: 'Balloon', value: formatBytes(memory.balloon) });
  }

  if (
    !memory.usageUnavailable &&
    reading?.state !== 'unavailable' &&
    memory.swapTotal &&
    memory.swapTotal > 0
  ) {
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
  lastBackup: string | number | Date | null | undefined,
  thresholds?: BackupThresholds,
  now: Date = new Date(),
): GuestDrawerBackupPresentation => {
  const timestamp = lastBackup instanceof Date ? lastBackup.getTime() : lastBackup;
  const info = getBackupInfo(timestamp, thresholds, now);
  const statusPresentation = getWorkloadsGuestBackupStatusPresentation(info.status);
  if (info.ageMs === null) {
    return {
      ageClass: statusPresentation.color,
      ageLabel: info.status === 'never' ? 'No completed backup found' : info.ageFormatted,
      dateLabel: 'Unknown',
    };
  }
  const backupDate = new Date(timestamp!);
  const daysSince = Math.floor(info.ageMs / (1000 * 60 * 60 * 24));

  return {
    ageClass: statusPresentation.color,
    ageLabel: daysSince === 0 ? 'Today' : daysSince === 1 ? 'Yesterday' : `${daysSince}d ago`,
    dateLabel: backupDate.toLocaleDateString(),
  };
};
