import { Component, Show, createMemo } from 'solid-js';
import { SummaryPanel } from '@/components/shared/SummaryPanel';
import { SummaryMetricCard } from '@/components/shared/SummaryMetricCard';
import type { ProtectionRollup, RecoveryOutcome, RecoveryPointsSeriesBucket } from '@/types/recovery';
import {
  buildRecoveryActivitySummary,
  buildRecoveryFreshnessBuckets,
  buildRecoveryItemCoverage,
  buildRecoveryPlatformCoverage,
  buildRecoveryPostureSummary,
  RECOVERY_SUMMARY_TIME_RANGES,
  RECOVERY_SUMMARY_TIME_RANGE_LABELS,
  type RecoverySummaryTimeRange,
} from '@/utils/recoverySummaryPresentation';

export interface RecoverySummaryProps {
  rollups: () => ProtectionRollup[];
  series: () => RecoveryPointsSeriesBucket[];
  seriesLoaded: () => boolean;
  seriesFailed?: () => boolean;
  summary: () => {
    total: number;
    counts: Record<RecoveryOutcome, number>;
    stale: number;
    neverSucceeded: number;
  };
  timeRange: () => RecoverySummaryTimeRange;
  onTimeRangeChange?: (range: RecoverySummaryTimeRange) => void;
}

export const RecoverySummary: Component<RecoverySummaryProps> = (props) => {
  const summary = () => props.summary();
  const hasRollups = () => summary().total > 0;

  const postureSummary = createMemo(() => buildRecoveryPostureSummary(props.rollups()));
  const freshnessBuckets = createMemo(() => buildRecoveryFreshnessBuckets(props.rollups()));
  const itemCoverage = createMemo(() => buildRecoveryItemCoverage(props.rollups()));
  const platformCoverage = createMemo(() => buildRecoveryPlatformCoverage(props.rollups()));
  const activity = createMemo(() => buildRecoveryActivitySummary(props.series()));
  const healthyCount = createMemo(() => postureSummary().healthy);
  const attentionCount = createMemo(() => postureSummary().attention);
  const primaryPostureMetric = createMemo(() => {
    if (attentionCount() > 0) {
      return {
        value: attentionCount(),
        label: 'need attention',
        valueClass: 'text-amber-600 dark:text-amber-400',
      };
    }
    if (postureSummary().running > 0) {
      return {
        value: postureSummary().running,
        label: 'currently running',
        valueClass: 'text-blue-600 dark:text-blue-400',
      };
    }
    return {
      value: healthyCount(),
      label: 'healthy items',
      valueClass: 'text-emerald-600 dark:text-emerald-400',
    };
  });
  const handleTimeRangeChange = (range: string) =>
    props.onTimeRangeChange?.(range as RecoverySummaryTimeRange);
  const postureRows = createMemo(
    () => {
      const rows: Array<{ label: string; value: string | number; valueClass?: string }> = [];
      const failedCount = postureSummary().failed + postureSummary().neverSucceeded;
      if (healthyCount() > 0) {
        rows.push({
          label: 'Healthy',
          value: healthyCount(),
          valueClass: 'text-emerald-600 dark:text-emerald-400',
        });
      }
      if (failedCount > 0) {
        rows.push({
          label: 'Failed',
          value: failedCount,
          valueClass: 'text-rose-600 dark:text-rose-400',
        });
      } else if (postureSummary().stale > 0) {
        rows.push({
          label: 'Stale',
          value: postureSummary().stale,
          valueClass: 'text-amber-600 dark:text-amber-400',
        });
      } else if (postureSummary().warning > 0) {
        rows.push({
          label: 'Warning',
          value: postureSummary().warning,
          valueClass: 'text-amber-600 dark:text-amber-400',
        });
      } else if (postureSummary().running > 0) {
        rows.push({
          label: 'Running',
          value: postureSummary().running,
          valueClass: 'text-blue-600 dark:text-blue-400',
        });
      }
      return rows.slice(0, 2);
    },
  );
  const postureSupportLine = createMemo(() =>
    postureRows()
      .map((row) => `${row.label} ${row.value}`)
      .join(' · '),
  );
  const freshWithin24hCount = createMemo(
    () =>
      (freshnessBuckets().find((bucket) => bucket.key === 'under1h')?.count ?? 0) +
      (freshnessBuckets().find((bucket) => bucket.key === 'under24h')?.count ?? 0),
  );
  const freshWithin1hCount = createMemo(
    () => freshnessBuckets().find((bucket) => bucket.key === 'under1h')?.count ?? 0,
  );
  const freshnessRows = createMemo(
    () => {
      const rows: Array<{ label: string; value: string | number; valueClass?: string }> = [];
      const over7dCount = freshnessBuckets().find((bucket) => bucket.key === 'over7d')?.count ?? 0;
      if (over7dCount > 0) {
        rows.push({ label: '>7d', value: over7dCount });
      }
      if (summary().neverSucceeded > 0) {
        rows.push({
          label: 'Never Succeeded',
          value: summary().neverSucceeded,
        });
      }
      if (freshWithin1hCount() > 0) {
        rows.push({
          label: '<1h',
          value: freshWithin1hCount(),
        });
      }
      return rows.slice(0, 2);
    },
  );
  const freshnessSupportLine = createMemo(() =>
    freshnessRows()
      .map((row) => `${row.label} ${row.value}`)
      .join(' · '),
  );

  const coverageSecondaryLabel = createMemo(() => {
    const platformCount = platformCoverage().platformCount;
    if (platformCount <= 0) return undefined;
    return (
      <span class="ml-auto truncate text-xs text-muted">
        {platformCount} {platformCount === 1 ? 'platform' : 'platforms'}
      </span>
    );
  });

  const activitySecondaryLabel = createMemo(() => {
    const latestLabel = activity().latestLabel;
    if (!latestLabel) return undefined;
    return <span class="ml-auto truncate text-xs text-muted">{latestLabel}</span>;
  });

  return (
    <Show when={hasRollups()}>
      <SummaryPanel
        headerLeft={
          <span class="font-medium text-base-content">{summary().total} protected items</span>
        }
        timeRange={props.timeRange()}
        onTimeRangeChange={props.onTimeRangeChange ? handleTimeRangeChange : undefined}
        timeRanges={RECOVERY_SUMMARY_TIME_RANGES}
        timeRangeLabels={RECOVERY_SUMMARY_TIME_RANGE_LABELS}
        testId="recovery-summary"
        class="overflow-hidden"
      >
        <SummaryMetricCard label="Posture" loaded={true} hasData={hasRollups()}>
          <div class="flex h-full flex-col gap-1.5">
            <div>
              <div class={`text-xl font-semibold tabular-nums ${primaryPostureMetric().valueClass}`}>
                {primaryPostureMetric().value}
              </div>
              <div class="text-[11px] text-muted">{primaryPostureMetric().label}</div>
            </div>
            <Show when={postureSupportLine()}>
              <div class="text-[11px] text-muted">{postureSupportLine()}</div>
            </Show>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard label="Freshness" loaded={true} hasData={hasRollups()}>
          <div class="flex h-full flex-col gap-1.5">
            <div>
              <div class="text-xl font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">
                {freshWithin24hCount()}
              </div>
              <div class="text-[11px] text-muted">fresh in 24h</div>
            </div>
            <Show when={freshnessSupportLine()}>
              <div class="text-[11px] text-muted">{freshnessSupportLine()}</div>
            </Show>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard
          label="Coverage"
          secondaryLabel={coverageSecondaryLabel()}
          loaded={true}
          hasData={hasRollups()}
        >
          <div class="flex h-full flex-col gap-1.5">
            <div>
              <div class="text-xl font-semibold tabular-nums text-base-content">
                {itemCoverage().itemTypeCount}
              </div>
              <div class="text-[11px] text-muted">item types</div>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard
          label="Activity"
          secondaryLabel={activitySecondaryLabel()}
          loaded={props.seriesLoaded()}
          hasData={activity().hasData}
          emptyMessage={props.seriesFailed?.() ? 'Trend data unavailable' : 'No recovery activity yet'}
        >
          <div class="flex h-full flex-col gap-1.5">
            <div>
              <div class="text-xl font-semibold tabular-nums text-base-content">
                {activity().totalEvents}
              </div>
              <div class="text-[11px] text-muted">recovery points</div>
            </div>
          </div>
        </SummaryMetricCard>
      </SummaryPanel>
    </Show>
  );
};
