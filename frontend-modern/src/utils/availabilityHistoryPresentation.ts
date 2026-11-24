import type {
  AvailabilityHistoryBucket,
  AvailabilityHistoryRange,
  AvailabilityHistoryTarget,
} from '@/api/availabilityHistory';

export interface AvailabilityHistoryRangeMeta {
  value: AvailabilityHistoryRange;
  label: string;
  columnLabel: string;
  windowPhrase: string;
  evidenceLabel: string;
  headerTitle: string;
}

export const AVAILABILITY_HISTORY_RANGE_OPTIONS: readonly AvailabilityHistoryRangeMeta[] = [
  {
    value: '24h',
    label: '24h',
    columnLabel: 'Uptime 24h',
    windowPhrase: '24h',
    evidenceLabel: '24-hour',
    headerTitle: 'Share of the last 24 hours the check was reachable',
  },
  {
    value: '7d',
    label: '7d',
    columnLabel: 'Uptime 7d',
    windowPhrase: '7 days',
    evidenceLabel: '7-day',
    headerTitle: 'Share of the last 7 days the check was reachable',
  },
  {
    value: '30d',
    label: '30d',
    columnLabel: 'Uptime 30d',
    windowPhrase: '30 days',
    evidenceLabel: '30-day',
    headerTitle: 'Share of the last 30 days the check was reachable',
  },
  {
    value: '365d',
    label: '1y',
    columnLabel: 'Uptime 1y',
    windowPhrase: '365 days',
    evidenceLabel: '1-year',
    headerTitle: 'Share of the last 365 days the check was reachable',
  },
];

export const getAvailabilityHistoryRangeMeta = (
  range: AvailabilityHistoryRange = '24h',
): AvailabilityHistoryRangeMeta =>
  AVAILABILITY_HISTORY_RANGE_OPTIONS.find((option) => option.value === range) ??
  AVAILABILITY_HISTORY_RANGE_OPTIONS[0];

export type AvailabilityHistoryBucketState =
  'reachable' | 'unreachable' | 'indeterminate' | 'unknown';

export const AVAILABILITY_STATE_PRESENTATION: Record<
  AvailabilityHistoryBucketState,
  { label: string; className: string; textClassName: string }
> = {
  reachable: {
    label: 'Reachable',
    className: 'bg-emerald-500 dark:bg-emerald-400',
    textClassName: 'text-emerald-700 dark:text-emerald-300',
  },
  unreachable: {
    label: 'Unreachable',
    className: 'bg-rose-500 dark:bg-rose-400',
    textClassName: 'text-rose-700 dark:text-rose-300',
  },
  indeterminate: {
    label: 'Indeterminate',
    className: 'bg-amber-400 dark:bg-amber-300',
    textClassName: 'text-amber-700 dark:text-amber-300',
  },
  unknown: {
    label: 'Unknown',
    className: 'bg-slate-300 dark:bg-slate-600',
    textClassName: 'text-muted',
  },
};

export const classifyAvailabilityBucket = (
  bucket: AvailabilityHistoryBucket,
): AvailabilityHistoryBucketState => {
  const durations: Array<[AvailabilityHistoryBucketState, number]> = [
    ['unreachable', bucket.unreachableSeconds],
    ['indeterminate', bucket.indeterminateSeconds],
    ['reachable', bucket.reachableSeconds],
    ['unknown', bucket.unknownSeconds],
  ];
  durations.sort((left, right) => right[1] - left[1]);
  return durations[0]?.[0] ?? 'unknown';
};

export const formatAvailabilityPercent = (value: number): string =>
  `${value.toLocaleString(undefined, { maximumFractionDigits: 2 })}%`;

// An uptime percentage is only honest when most of the window had a
// determinate answer. availabilityPercent leaves out indeterminate time, so a
// window of mostly inconclusive UDP results could otherwise read 100 percent.
const MIN_UPTIME_COVERAGE_PERCENT = 90;
const MIN_DETERMINATE_SHARE = 0.9;

export type AvailabilityUptimeVerdict =
  | { kind: 'ok'; percent: number }
  | { kind: 'thin'; observedMinutes: number }
  | { kind: 'inconclusive' };

export const getAvailabilityUptimeVerdict = (
  summary: AvailabilityHistoryTarget['summary'],
): AvailabilityUptimeVerdict => {
  const observed =
    (summary?.reachableSeconds ?? 0) +
    (summary?.unreachableSeconds ?? 0) +
    (summary?.indeterminateSeconds ?? 0);
  if (
    !summary ||
    summary.coveragePercent < MIN_UPTIME_COVERAGE_PERCENT ||
    summary.availabilityPercent === undefined
  ) {
    return { kind: 'thin', observedMinutes: Math.round(observed / 60) };
  }
  const determinate = summary.reachableSeconds + summary.unreachableSeconds;
  if (observed <= 0 || determinate / observed < MIN_DETERMINATE_SHARE) {
    return { kind: 'inconclusive' };
  }
  return { kind: 'ok', percent: summary.availabilityPercent };
};

export const formatAvailabilityObservedDuration = (
  observedMinutes: number,
  range: AvailabilityHistoryRange,
): string => {
  if (range === '24h' || observedMinutes < 120) {
    return `${observedMinutes.toLocaleString()}m`;
  }
  const hours = Math.round(observedMinutes / 60);
  if (hours < 48) {
    return `${hours.toLocaleString()}h`;
  }
  return `${Math.round(hours / 24).toLocaleString()}d`;
};

export const formatAvailabilityDurationSeconds = (seconds: number): string => {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0s';
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remMinutes = minutes % 60;
  if (hours < 48) {
    return remMinutes > 0 ? `${hours}h ${remMinutes}m` : `${hours}h`;
  }
  const days = Math.floor(hours / 24);
  const remHours = hours % 24;
  return remHours > 0 ? `${days}d ${remHours}h` : `${days}d`;
};

// Compact window uptime for the checks table, on the same coverage rule as the
// fleet view: below 90 percent observed there is no honest percentage yet.
export const getAvailabilityUptimeCell = (
  history: AvailabilityHistoryTarget | undefined,
  range: AvailabilityHistoryRange = '24h',
): { label: string; title: string; tone: 'default' | 'warning' | 'danger' | 'muted' } => {
  const meta = getAvailabilityHistoryRangeMeta(range);
  if (!history?.summary) {
    return { label: '—', title: 'History unavailable', tone: 'muted' };
  }
  const verdict = getAvailabilityUptimeVerdict(history.summary);
  if (verdict.kind === 'thin') {
    return {
      label: '—',
      title: `Not enough history yet: ${formatAvailabilityObservedDuration(verdict.observedMinutes, range)} observed in the last ${meta.windowPhrase}`,
      tone: 'muted',
    };
  }
  if (verdict.kind === 'inconclusive') {
    return {
      label: '—',
      title: `Most results in the last ${meta.windowPhrase} were inconclusive, so there is no uptime figure`,
      tone: 'muted',
    };
  }
  const percent = verdict.percent;
  return {
    label: formatAvailabilityPercent(percent),
    title: `Available ${formatAvailabilityPercent(percent)} of the last ${meta.windowPhrase} (${formatAvailabilityPercent(history.summary.coveragePercent)} observed)`,
    tone: percent < 95 ? 'danger' : percent < 99 ? 'warning' : 'default',
  };
};
