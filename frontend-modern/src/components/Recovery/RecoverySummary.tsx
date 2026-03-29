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
  const postureSecondaryLabel = createMemo(() => {
    if (healthyCount() <= 0) return undefined;
    return (
      <span class="ml-auto truncate text-xs text-emerald-600 dark:text-emerald-400">
        {healthyCount()} healthy
      </span>
    );
  });
  const freshWithin24hCount = createMemo(
    () =>
      (freshnessBuckets().find((bucket) => bucket.key === 'under1h')?.count ?? 0) +
      (freshnessBuckets().find((bucket) => bucket.key === 'under24h')?.count ?? 0),
  );
  const freshnessSecondaryLabel = createMemo(() => {
    if (summary().stale > 0) {
      return (
        <span class="ml-auto truncate text-xs text-amber-600 dark:text-amber-400">
          {summary().stale} stale
        </span>
      );
    }
    if (summary().neverSucceeded > 0) {
      return (
        <span class="ml-auto truncate text-xs text-red-600 dark:text-red-400">
          {summary().neverSucceeded} never succeeded
        </span>
      );
    }
    return undefined;
  });

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
  const headerStatusCue = createMemo(() => {
    if (attentionCount() > 0) {
      return (
        <span class="text-amber-600 dark:text-amber-400">
          {attentionCount()} attention
        </span>
      );
    }
    if (healthyCount() > 0) {
      return (
        <span class="text-emerald-600 dark:text-emerald-400">
          {healthyCount()} healthy
        </span>
      );
    }
    return undefined;
  });

  return (
    <Show when={hasRollups()}>
      <SummaryPanel
        headerLeft={
          <>
            <span class="font-medium text-base-content">{summary().total} protected items</span>
            <Show when={headerStatusCue()}>{headerStatusCue()}</Show>
          </>
        }
        timeRange={props.timeRange()}
        onTimeRangeChange={props.onTimeRangeChange ? handleTimeRangeChange : undefined}
        timeRanges={RECOVERY_SUMMARY_TIME_RANGES}
        timeRangeLabels={RECOVERY_SUMMARY_TIME_RANGE_LABELS}
        testId="recovery-summary"
        class="overflow-hidden"
      >
        <SummaryMetricCard
          label="Posture"
          secondaryLabel={postureSecondaryLabel()}
          loaded={true}
          hasData={hasRollups()}
        >
          <div class="flex h-full flex-col gap-1.5">
            <div>
              <div class={`text-xl font-semibold tabular-nums ${primaryPostureMetric().valueClass}`}>
                {primaryPostureMetric().value}
              </div>
              <div class="text-[11px] text-muted">{primaryPostureMetric().label}</div>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard
          label="Freshness"
          secondaryLabel={freshnessSecondaryLabel()}
          loaded={true}
          hasData={hasRollups()}
        >
          <div class="flex h-full flex-col gap-1.5">
            <div>
              <div class="text-xl font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">
                {freshWithin24hCount()}
              </div>
              <div class="text-[11px] text-muted">fresh in 24h</div>
            </div>
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
