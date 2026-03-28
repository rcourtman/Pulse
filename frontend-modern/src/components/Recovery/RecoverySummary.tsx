import { Component, For, Show, createMemo } from 'solid-js';
import { SummaryPanel } from '@/components/shared/SummaryPanel';
import { SummaryMetricCard } from '@/components/shared/SummaryMetricCard';
import type { ProtectionRollup, RecoveryOutcome, RecoveryPointsSeriesBucket } from '@/types/recovery';
import {
  buildRecoveryActivitySummary,
  buildRecoveryFreshnessBuckets,
  buildRecoveryItemCoverage,
  buildRecoveryPlatformCoverage,
  buildRecoveryPostureSegments,
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
  const postureSegments = createMemo(() => buildRecoveryPostureSegments(props.rollups()));
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
  const visiblePostureSegments = createMemo(() =>
    postureSegments().filter((segment) => segment.count > 0).slice(0, 4),
  );
  const handleTimeRangeChange = (range: string) =>
    props.onTimeRangeChange?.(range as RecoverySummaryTimeRange);

  return (
    <Show when={hasRollups()}>
      <SummaryPanel
        headerLeft={
          <>
            <span class="font-medium text-base-content">
              {summary().total} protected
            </span>
            <Show when={attentionCount() > 0}>
              <span class="text-amber-600 dark:text-amber-400">
                {attentionCount()} attention
              </span>
            </Show>
            <Show when={attentionCount() === 0}>
              <span class="text-emerald-600 dark:text-emerald-400">
                {healthyCount()} healthy
              </span>
            </Show>
            <Show when={postureSummary().running > 0}>
              <span class="text-blue-600 dark:text-blue-400">
                {postureSummary().running} running
              </span>
            </Show>
          </>
        }
        timeRange={props.timeRange()}
        onTimeRangeChange={props.onTimeRangeChange ? handleTimeRangeChange : undefined}
        timeRanges={RECOVERY_SUMMARY_TIME_RANGES}
        timeRangeLabels={RECOVERY_SUMMARY_TIME_RANGE_LABELS}
        testId="recovery-summary"
        class="overflow-hidden"
        density="compact"
      >
        <SummaryMetricCard label="Recovery Posture" loaded={true} hasData={hasRollups()} density="compact">
          <div class="flex h-full flex-col gap-2">
            <div>
              <div class={`text-xl font-semibold tabular-nums ${primaryPostureMetric().valueClass}`}>
                {primaryPostureMetric().value}
              </div>
              <div class="text-[11px] text-muted">{primaryPostureMetric().label}</div>
            </div>

            <div class="h-1.5 overflow-hidden rounded-full bg-surface-alt">
              <div class="flex h-full">
                <For each={visiblePostureSegments()}>
                  {(segment) => (
                    <div
                      class={segment.color}
                      style={{ width: `${Math.max(segment.percent, segment.count > 0 ? 4 : 0)}%` }}
                      title={`${segment.label}: ${segment.count}`}
                    />
                  )}
                </For>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-x-3 gap-y-1 text-[11px]">
              <For each={visiblePostureSegments()}>
                {(segment) => (
                  <div class="flex items-center justify-between gap-2">
                    <div class="flex min-w-0 items-center gap-1.5 text-base-content">
                      <span class={`h-1.5 w-1.5 rounded-full ${segment.color}`} />
                      <span class="truncate">{segment.label}</span>
                    </div>
                    <span class={`shrink-0 font-semibold tabular-nums ${segment.textColor}`}>
                      {segment.count}
                    </span>
                  </div>
                )}
              </For>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard label="Freshness" loaded={true} hasData={hasRollups()} density="compact">
          <div class="flex h-full flex-col gap-2">
            <div class="flex items-end justify-between gap-3">
              <div>
                <div class="text-xl font-semibold tabular-nums text-amber-600 dark:text-amber-400">
                  {summary().stale}
                </div>
                <div class="text-[11px] text-muted">stale items</div>
              </div>
              <div class="text-right text-[11px]">
                <div class="font-semibold tabular-nums text-rose-600 dark:text-rose-400">
                  {summary().neverSucceeded}
                </div>
                <div class="text-muted">Never Succeeded</div>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-x-3 gap-y-1 text-[11px]">
              <For each={freshnessBuckets()}>
                {(bucket) => (
                  <div class="flex items-center justify-between gap-2">
                    <div class="flex min-w-0 items-center gap-1.5 text-base-content">
                      <span class={`h-1.5 w-1.5 rounded-full ${bucket.color}`} />
                      <span class="truncate">{bucket.label}</span>
                    </div>
                    <span class="shrink-0 tabular-nums font-semibold text-base-content">
                      {bucket.count}
                    </span>
                  </div>
                )}
              </For>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard label="Protected Footprint" loaded={true} hasData={hasRollups()} density="compact">
          <div class="flex h-full flex-col gap-2">
            <div class="grid grid-cols-2 gap-3">
              <div>
                <div class="text-xl font-semibold tabular-nums text-base-content">
                  {itemCoverage().itemTypeCount}
                </div>
                <div class="text-[11px] text-muted">item types</div>
              </div>
              <div class="text-right">
                <div class="text-xl font-semibold tabular-nums text-base-content">
                  {platformCoverage().platformCount}
                </div>
                <div class="text-[11px] text-muted">platforms</div>
              </div>
            </div>

            <dl class="space-y-1 border-t border-border-subtle pt-1.5 text-[11px]">
              <div class="flex items-center justify-between gap-3">
                <dt class="text-muted">Primary Item</dt>
                <dd class="font-medium text-base-content">
                  {itemCoverage().primaryItemLabel ?? 'n/a'}
                </dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-muted">Primary Platform</dt>
                <dd class="font-medium text-base-content">
                  {platformCoverage().primaryPlatformLabel ?? 'n/a'}
                </dd>
              </div>
            </dl>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard
          label="Recent History"
          loaded={props.seriesLoaded()}
          hasData={activity().hasData}
          emptyMessage={props.seriesFailed?.() ? 'Trend data unavailable' : 'No recovery activity yet'}
          density="compact"
        >
          <div class="flex h-full flex-col gap-2">
            <div>
              <div class="text-xl font-semibold tabular-nums text-base-content">
                {activity().totalEvents}
              </div>
              <div class="text-[11px] text-muted">recovery points</div>
            </div>

            <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px]">
              <span class="text-muted">Days Active</span>
              <span class="font-semibold text-base-content">{activity().activeDays}</span>
              <span class="text-muted">Avg / Day</span>
              <span class="font-semibold text-base-content">
                {activity().averagePerDay.toFixed(1)}
              </span>
              <span class="text-muted">Peak</span>
              <span class="font-semibold text-base-content">{activity().busiestCount}</span>
            </div>

            <div class="border-t border-border-subtle pt-1.5 text-[11px]">
              <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
                <span class="text-muted">Peak Day</span>
                <span class="font-medium text-base-content">{activity().busiestLabel ?? 'n/a'}</span>
                <span class="text-muted">Latest</span>
                <span class="font-medium text-base-content">{activity().latestLabel ?? 'n/a'}</span>
              </div>
            </div>
          </div>
        </SummaryMetricCard>
      </SummaryPanel>
    </Show>
  );
};
