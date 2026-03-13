import type { ProtectionRollup, RecoveryOutcome, RecoveryPointsSeriesBucket } from '@/types/recovery';
import {
  getRecoveryOutcomeBarClass,
  getRecoveryOutcomeLabel,
  getRecoveryOutcomeTextClass,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';
import { isRecoveryRollupStale } from '@/utils/recoveryTablePresentation';

export const RECOVERY_SUMMARY_TIME_RANGES = ['7d', '30d', '90d'] as const;

export type RecoverySummaryTimeRange = (typeof RECOVERY_SUMMARY_TIME_RANGES)[number];

export const RECOVERY_SUMMARY_TIME_RANGE_LABELS: Record<RecoverySummaryTimeRange, string> = {
  '7d': '7d',
  '30d': '30d',
  '90d': '90d',
};

export type RecoveryFreshnessBucketKey = 'under1h' | 'under24h' | 'under7d' | 'over7d';

export interface RecoveryFreshnessBucketPresentation {
  key: RecoveryFreshnessBucketKey;
  label: string;
  color: string;
}

export interface RecoveryFreshnessBucketStat extends RecoveryFreshnessBucketPresentation {
  count: number;
  percent: number;
}

export interface RecoveryOutcomeSegment {
  outcome: RecoveryOutcome;
  label: string;
  count: number;
  percent: number;
  color: string;
  textColor: string;
}

export type RecoveryPostureKey =
  | 'healthy'
  | 'stale'
  | 'failed'
  | 'warning'
  | 'running'
  | 'never-succeeded'
  | 'unknown';

export interface RecoveryPostureSegment {
  key: RecoveryPostureKey;
  label: string;
  count: number;
  percent: number;
  color: string;
  textColor: string;
}

export interface RecoveryPostureSummary {
  healthy: number;
  stale: number;
  failed: number;
  warning: number;
  running: number;
  neverSucceeded: number;
  unknown: number;
  attention: number;
}

export interface RecoveryActivityBar {
  day: string;
  total: number;
  heightPct: number;
  isLatest: boolean;
  isPeak: boolean;
}

export interface RecoveryActivitySummary {
  hasData: boolean;
  totalEvents: number;
  averagePerDay: number;
  activeDays: number;
  latestCount: number;
  latestLabel: string | null;
  busiestCount: number;
  busiestLabel: string | null;
  startLabel: string | null;
  endLabel: string | null;
  bars: RecoveryActivityBar[];
}

export interface RecoveryAttentionItem {
  key: 'failed' | 'never-succeeded' | 'warning' | 'stale' | 'running';
  label: string;
  count: number;
  tone: 'rose' | 'amber' | 'blue';
  detail: string;
}

export function getRecoveryAttentionChipClass(
  tone: RecoveryAttentionItem['tone'],
): string {
  switch (tone) {
    case 'rose':
      return 'border-rose-500/30 bg-rose-500/10 text-rose-200';
    case 'blue':
      return 'border-blue-500/30 bg-blue-500/10 text-blue-200';
    default:
      return 'border-amber-500/30 bg-amber-500/10 text-amber-200';
  }
}

export function getRecoveryAttentionDotClass(
  tone: RecoveryAttentionItem['tone'],
): string {
  switch (tone) {
    case 'rose':
      return 'bg-rose-400';
    case 'blue':
      return 'bg-blue-400';
    default:
      return 'bg-amber-400';
  }
}

export const RECOVERY_FRESHNESS_BUCKETS: RecoveryFreshnessBucketPresentation[] = [
  { key: 'under1h', label: '<1h', color: 'bg-emerald-500' },
  { key: 'under24h', label: '<24h', color: 'bg-emerald-400' },
  { key: 'under7d', label: '<7d', color: 'bg-amber-400' },
  { key: 'over7d', label: '>7d', color: 'bg-red-500' },
];

const formatShortDate = (value: string): string => {
  const date = new Date(`${value}T00:00:00`);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
};

const toPercent = (count: number, total: number): number =>
  total > 0 ? Math.round((count / total) * 100) : 0;

const RECOVERY_POSTURE_PRESENTATION: Record<
  RecoveryPostureKey,
  Pick<RecoveryPostureSegment, 'label' | 'color' | 'textColor'>
> = {
  healthy: {
    label: 'Healthy',
    color: 'bg-emerald-500',
    textColor: 'text-emerald-600 dark:text-emerald-400',
  },
  stale: {
    label: 'Stale',
    color: 'bg-amber-400',
    textColor: 'text-amber-600 dark:text-amber-400',
  },
  failed: {
    label: 'Failed',
    color: 'bg-rose-500',
    textColor: 'text-rose-600 dark:text-rose-400',
  },
  warning: {
    label: 'Warning',
    color: 'bg-amber-500',
    textColor: 'text-amber-600 dark:text-amber-400',
  },
  running: {
    label: 'Running',
    color: 'bg-blue-500',
    textColor: 'text-blue-600 dark:text-blue-400',
  },
  'never-succeeded': {
    label: 'Never Succeeded',
    color: 'bg-rose-400',
    textColor: 'text-rose-600 dark:text-rose-400',
  },
  unknown: {
    label: 'Unknown',
    color: 'bg-slate-500',
    textColor: 'text-muted',
  },
};

export function buildRecoveryOutcomeSegments(summary: {
  total: number;
  counts: Record<RecoveryOutcome, number>;
}): RecoveryOutcomeSegment[] {
  if (summary.total <= 0) return [];
  return ['success', 'running', 'warning', 'failed', 'unknown']
    .map((outcome) => ({
      outcome: outcome as RecoveryOutcome,
      label: getRecoveryOutcomeLabel(outcome),
      count: summary.counts[outcome as RecoveryOutcome] || 0,
      percent: toPercent(summary.counts[outcome as RecoveryOutcome] || 0, summary.total),
      color: getRecoveryOutcomeBarClass(outcome),
      textColor: getRecoveryOutcomeTextClass(outcome),
    }))
    .filter((segment) => segment.count > 0);
}

export function buildRecoveryPostureSummary(
  rollups: ProtectionRollup[],
  nowMs = Date.now(),
): RecoveryPostureSummary {
  const counts: RecoveryPostureSummary = {
    healthy: 0,
    stale: 0,
    failed: 0,
    warning: 0,
    running: 0,
    neverSucceeded: 0,
    unknown: 0,
    attention: 0,
  };

  for (const rollup of rollups) {
    const successMs = rollup.lastSuccessAt ? Date.parse(rollup.lastSuccessAt) : 0;
    const attemptMs = rollup.lastAttemptAt ? Date.parse(rollup.lastAttemptAt) : 0;
    const neverSucceeded =
      (!Number.isFinite(successMs) || successMs <= 0) &&
      Number.isFinite(attemptMs) &&
      attemptMs > 0;
    const outcome = normalizeRecoveryOutcome(rollup.lastOutcome);
    const stale = !neverSucceeded && isRecoveryRollupStale(rollup, nowMs);

    if (neverSucceeded) {
      counts.neverSucceeded += 1;
      counts.attention += 1;
      continue;
    }
    if (outcome === 'failed') {
      counts.failed += 1;
      counts.attention += 1;
      continue;
    }
    if (outcome === 'warning') {
      counts.warning += 1;
      counts.attention += 1;
      continue;
    }
    if (stale) {
      counts.stale += 1;
      counts.attention += 1;
      continue;
    }
    if (outcome === 'running') {
      counts.running += 1;
      continue;
    }
    if (outcome === 'success') {
      counts.healthy += 1;
      continue;
    }
    counts.unknown += 1;
  }

  return counts;
}

export function buildRecoveryPostureSegments(
  rollups: ProtectionRollup[],
  nowMs = Date.now(),
): RecoveryPostureSegment[] {
  const summary = buildRecoveryPostureSummary(rollups, nowMs);
  const total = rollups.length;
  const order: RecoveryPostureKey[] = [
    'healthy',
    'stale',
    'failed',
    'warning',
    'running',
    'never-succeeded',
    'unknown',
  ];

  return order
    .map((key) => {
      const count =
        key === 'never-succeeded'
          ? summary.neverSucceeded
          : key === 'unknown'
            ? summary.unknown
            : summary[key];
      const presentation = RECOVERY_POSTURE_PRESENTATION[key];
      return {
        key,
        label: presentation.label,
        count,
        percent: toPercent(count, total),
        color: presentation.color,
        textColor: presentation.textColor,
      };
    })
    .filter((segment) => segment.count > 0);
}

export function buildRecoveryFreshnessBuckets(
  rollups: ProtectionRollup[],
  nowMs = Date.now(),
): RecoveryFreshnessBucketStat[] {
  const counts: Record<RecoveryFreshnessBucketKey, number> = {
    under1h: 0,
    under24h: 0,
    under7d: 0,
    over7d: 0,
  };

  for (const rollup of rollups) {
    const successMs = rollup.lastSuccessAt ? Date.parse(rollup.lastSuccessAt) : 0;
    if (!successMs || !Number.isFinite(successMs)) {
      counts.over7d += 1;
      continue;
    }

    const ageMs = nowMs - successMs;
    if (ageMs < 3_600_000) counts.under1h += 1;
    else if (ageMs < 86_400_000) counts.under24h += 1;
    else if (ageMs < 604_800_000) counts.under7d += 1;
    else counts.over7d += 1;
  }

  return RECOVERY_FRESHNESS_BUCKETS.map((bucket) => ({
    ...bucket,
    count: counts[bucket.key],
    percent: toPercent(counts[bucket.key], rollups.length),
  }));
}

export function buildRecoveryActivitySummary(
  series: RecoveryPointsSeriesBucket[],
): RecoveryActivitySummary {
  if (series.length === 0) {
    return {
      hasData: false,
      totalEvents: 0,
      averagePerDay: 0,
      activeDays: 0,
      latestCount: 0,
      latestLabel: null,
      busiestCount: 0,
      busiestLabel: null,
      startLabel: null,
      endLabel: null,
      bars: [],
    };
  }

  const totalEvents = series.reduce((sum, bucket) => sum + bucket.total, 0);
  const activeDays = series.filter((bucket) => bucket.total > 0).length;
  const latest = series[series.length - 1];
  const busiest = series.reduce((best, bucket) => (bucket.total > best.total ? bucket : best), series[0]);
  const maxTotal = Math.max(...series.map((bucket) => bucket.total), 0);

  return {
    hasData: totalEvents > 0,
    totalEvents,
    averagePerDay: totalEvents / series.length,
    activeDays,
    latestCount: latest.total,
    latestLabel: formatShortDate(latest.day),
    busiestCount: busiest.total,
    busiestLabel: formatShortDate(busiest.day),
    startLabel: formatShortDate(series[0].day),
    endLabel: formatShortDate(latest.day),
    bars: series.map((bucket, index) => ({
      day: bucket.day,
      total: bucket.total,
      heightPct:
        maxTotal <= 0 || bucket.total <= 0 ? 0 : Math.max((bucket.total / maxTotal) * 100, 10),
      isLatest: index === series.length - 1,
      isPeak: bucket.day === busiest.day && bucket.total > 0,
    })),
  };
}

export function buildRecoveryAttentionItems(summary: {
  counts: Record<RecoveryOutcome, number>;
  stale: number;
  neverSucceeded: number;
}): RecoveryAttentionItem[] {
  const items: RecoveryAttentionItem[] = [
    {
      key: 'failed',
      label: 'Failed',
      count: summary.counts.failed || 0,
      tone: 'rose',
      detail: 'Latest protection run failed and needs investigation.',
    },
    {
      key: 'never-succeeded',
      label: 'Never succeeded',
      count: summary.neverSucceeded || 0,
      tone: 'rose',
      detail: 'Protected items have not completed a successful run yet.',
    },
    {
      key: 'warning',
      label: 'Warnings',
      count: summary.counts.warning || 0,
      tone: 'amber',
      detail: 'Latest protection run completed with warnings.',
    },
    {
      key: 'stale',
      label: 'Stale',
      count: summary.stale || 0,
      tone: 'amber',
      detail: 'No successful protection has landed within the stale threshold.',
    },
    {
      key: 'running',
      label: 'Running now',
      count: summary.counts.running || 0,
      tone: 'blue',
      detail: 'Protection jobs are currently in flight.',
    },
  ];
  return items.filter((item) => item.count > 0);
}
