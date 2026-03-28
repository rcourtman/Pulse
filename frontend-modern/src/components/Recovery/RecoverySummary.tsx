import { Component, For, Show, createMemo } from 'solid-js';
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
  const postureRows = createMemo(() => [
    {
      label: 'Healthy',
      value: healthyCount(),
      valueClass: 'text-emerald-600 dark:text-emerald-400',
    },
    {
      label: 'Stale',
      value: postureSummary().stale,
      valueClass: 'text-amber-600 dark:text-amber-400',
    },
    {
      label: 'Failed',
      value: postureSummary().failed + postureSummary().neverSucceeded,
      valueClass: 'text-rose-600 dark:text-rose-400',
    },
  ]);
  const freshnessRows = createMemo(() => [
    {
      label: '<24h',
      value: freshnessBuckets().find((bucket) => bucket.key === 'under24h')?.count ?? 0,
    },
    {
      label: '>7d',
      value: freshnessBuckets().find((bucket) => bucket.key === 'over7d')?.count ?? 0,
    },
    {
      label: 'Never Succeeded',
      value: summary().neverSucceeded,
    },
  ]);
  const freshWithin24hCount = createMemo(
    () =>
      (freshnessBuckets().find((bucket) => bucket.key === 'under1h')?.count ?? 0) +
      (freshnessBuckets().find((bucket) => bucket.key === 'under24h')?.count ?? 0),
  );
  const freshWithin1hCount = createMemo(
    () => freshnessBuckets().find((bucket) => bucket.key === 'under1h')?.count ?? 0,
  );

  const MetricRows = (rowProps: {
    rows: Array<{ label: string; value: string | number; valueClass?: string }>;
  }) => (
    <dl class="space-y-1 text-[11px]">
      <For each={rowProps.rows}>
        {(row) => (
          <div class="flex items-center justify-between gap-3">
            <dt class="text-muted">{row.label}</dt>
            <dd class={`font-semibold tabular-nums text-base-content ${row.valueClass ?? ''}`.trim()}>
              {row.value}
            </dd>
          </div>
        )}
      </For>
    </dl>
  );

  return (
    <Show when={hasRollups()}>
      <SummaryPanel
        headerLeft={
          <span class="font-medium text-base-content">{summary().total} protected</span>
        }
        timeRange={props.timeRange()}
        onTimeRangeChange={props.onTimeRangeChange ? handleTimeRangeChange : undefined}
        timeRanges={RECOVERY_SUMMARY_TIME_RANGES}
        timeRangeLabels={RECOVERY_SUMMARY_TIME_RANGE_LABELS}
        testId="recovery-summary"
        class="overflow-hidden"
      >
        <SummaryMetricCard label="Recovery Posture" loaded={true} hasData={hasRollups()}>
          <div class="flex h-full flex-col gap-2">
            <div>
              <div class={`text-xl font-semibold tabular-nums ${primaryPostureMetric().valueClass}`}>
                {primaryPostureMetric().value}
              </div>
              <div class="text-[11px] text-muted">{primaryPostureMetric().label}</div>
            </div>
            <div class="border-t border-border-subtle pt-1.5">
              <MetricRows rows={postureRows()} />
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard label="Freshness" loaded={true} hasData={hasRollups()}>
          <div class="flex h-full flex-col gap-2">
            <div>
              <div class="text-xl font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">
                {freshWithin24hCount()}
              </div>
              <div class="text-[11px] text-muted">fresh in 24h</div>
            </div>
            <div class="border-t border-border-subtle pt-1.5">
              <MetricRows
                rows={[
                  { label: '<1h', value: freshWithin1hCount() },
                  ...freshnessRows(),
                ]}
              />
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard label="Protected Footprint" loaded={true} hasData={hasRollups()}>
          <div class="flex h-full flex-col gap-2">
            <div>
              <div class="text-xl font-semibold tabular-nums text-base-content">
                {itemCoverage().itemTypeCount}
              </div>
              <div class="text-[11px] text-muted">item types</div>
            </div>
            <div class="border-t border-border-subtle pt-1.5">
              <MetricRows
                rows={[
                  { label: 'Platforms', value: platformCoverage().platformCount },
                  { label: 'Primary Item', value: itemCoverage().primaryItemLabel ?? 'n/a' },
                  { label: 'Primary Platform', value: platformCoverage().primaryPlatformLabel ?? 'n/a' },
                ]}
              />
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard
          label="Recent History"
          loaded={props.seriesLoaded()}
          hasData={activity().hasData}
          emptyMessage={props.seriesFailed?.() ? 'Trend data unavailable' : 'No recovery activity yet'}
        >
          <div class="flex h-full flex-col gap-2">
            <div>
              <div class="text-xl font-semibold tabular-nums text-base-content">
                {activity().totalEvents}
              </div>
              <div class="text-[11px] text-muted">recovery points</div>
            </div>
            <div class="border-t border-border-subtle pt-1.5">
              <MetricRows
                rows={[
                  { label: 'Days Active', value: activity().activeDays },
                  { label: 'Avg / Day', value: activity().averagePerDay.toFixed(1) },
                  { label: 'Peak Day', value: activity().busiestLabel ?? 'n/a' },
                  { label: 'Latest Activity', value: activity().latestLabel ?? 'n/a' },
                ]}
              />
            </div>
          </div>
        </SummaryMetricCard>
      </SummaryPanel>
    </Show>
  );
};
