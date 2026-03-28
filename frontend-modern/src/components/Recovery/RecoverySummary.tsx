import { Component, For, Show, createMemo } from 'solid-js';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
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
  const recentWindowLabel = createMemo(() => {
    const activitySummary = activity();
    if (!activitySummary.startLabel || !activitySummary.endLabel) return null;
    return `${activitySummary.startLabel} to ${activitySummary.endLabel}`;
  });
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
      >
        <SummaryMetricCard label="Recovery Posture" loaded={true} hasData={hasRollups()}>
          <div class="flex h-full flex-col gap-3">
            <div class="flex items-start justify-between gap-3">
              <div>
                <div class={`text-2xl font-semibold tabular-nums ${primaryPostureMetric().valueClass}`}>
                  {primaryPostureMetric().value}
                </div>
                <div class="text-xs text-muted">{primaryPostureMetric().label}</div>
              </div>
              <div class="text-right">
                <div class="text-sm font-semibold tabular-nums text-base-content">
                  {summary().total}
                </div>
                <div class="text-[11px] text-muted">protected items</div>
              </div>
            </div>

            <div class="space-y-1.5">
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
              <div class="grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
                <For each={visiblePostureSegments()}>
                  {(segment) => (
                    <div class="flex items-center justify-between gap-3">
                      <div class="flex items-center gap-2 text-base-content">
                        <span class={`h-2 w-2 rounded-full ${segment.color}`} />
                        <span>{segment.label}</span>
                      </div>
                      <span class={`font-semibold tabular-nums ${segment.textColor}`}>
                        {segment.count}
                      </span>
                    </div>
                  )}
                </For>
              </div>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard label="Freshness" loaded={true} hasData={hasRollups()}>
          <div class="flex h-full flex-col gap-3">
            <div class="flex items-start justify-between gap-3">
              <div>
                <div class="text-2xl font-semibold tabular-nums text-amber-600 dark:text-amber-400">
                  {summary().stale}
                </div>
                <div class="text-xs text-muted">stale items</div>
              </div>
              <div class="space-y-1 text-right text-xs">
                <div>
                  <span class="font-semibold text-rose-600 dark:text-rose-400">
                    {summary().neverSucceeded}
                  </span>{' '}
                  <span class="text-muted">never succeeded</span>
                </div>
                <div>
                  <span class="font-semibold text-blue-600 dark:text-blue-400">
                    {postureSummary().running}
                  </span>{' '}
                  <span class="text-muted">running</span>
                </div>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-x-3 gap-y-1.5 text-xs">
              <For each={freshnessBuckets()}>
                {(bucket) => (
                  <div class="flex items-center justify-between gap-3">
                    <div class="flex items-center gap-2 text-base-content">
                      <span class={`h-2 w-2 rounded-full ${bucket.color}`} />
                      <span>{bucket.label}</span>
                    </div>
                    <span class="tabular-nums font-semibold text-base-content">{bucket.count}</span>
                  </div>
                )}
              </For>
            </div>

            <div class="grid grid-cols-2 gap-2 border-t border-border-subtle pt-2 text-[11px]">
              <div class="rounded-md border border-border-subtle bg-surface-alt/35 px-2.5 py-1.5">
                <div class="text-muted">Attention</div>
                <div class="mt-1 font-semibold text-base-content">{attentionCount()}</div>
              </div>
              <div class="rounded-md border border-border-subtle bg-surface-alt/35 px-2.5 py-1.5">
                <div class="text-muted">Fresh &lt;24h</div>
                <div class="mt-1 font-semibold text-base-content">
                  {freshnessBuckets()
                    .filter((bucket) => bucket.key === 'under1h' || bucket.key === 'under24h')
                    .reduce((total, bucket) => total + bucket.count, 0)}
                </div>
              </div>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard label="Protected Footprint" loaded={true} hasData={hasRollups()}>
          <div class="flex h-full flex-col gap-3">
            <div class="grid grid-cols-2 gap-2 text-[11px]">
              <div class="rounded-md border border-border-subtle bg-surface-alt/35 px-2.5 py-2">
                <div class="text-muted">Item Types</div>
                <div class="mt-1 text-xl font-semibold tabular-nums text-base-content">
                  {itemCoverage().itemTypeCount}
                </div>
              </div>
              <div class="rounded-md border border-border-subtle bg-surface-alt/35 px-2.5 py-2">
                <div class="text-muted">Platforms</div>
                <div class="mt-1 text-xl font-semibold tabular-nums text-base-content">
                  {platformCoverage().platformCount}
                </div>
              </div>
            </div>

            <dl class="space-y-1.5 text-xs">
              <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-1.5">
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

            <div class="grid gap-2 border-t border-border-subtle pt-2">
              <Show when={itemCoverage().items.length > 0}>
                <div class="flex flex-wrap gap-1.5">
                  <For each={itemCoverage().items.slice(0, 3)}>
                    {(item) => (
                      <div class="inline-flex items-center gap-1.5 text-[11px]">
                        <span class={item.toneClass}>{item.label}</span>
                        <span class="tabular-nums text-base-content">{item.count}</span>
                      </div>
                    )}
                  </For>
                </div>
              </Show>

              <div class="flex flex-wrap gap-1.5">
                <For each={platformCoverage().items.slice(0, 3)}>
                  {(item) => {
                    const badge = getSourcePlatformBadge(item.key);
                    return (
                      <div class="inline-flex items-center gap-1.5 text-[11px]">
                        <span class={badge?.classes || 'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-base-content'}>
                          {badge?.label || item.label}
                        </span>
                        <span class="tabular-nums text-base-content">{item.count}</span>
                      </div>
                    );
                  }}
                </For>
              </div>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard
          label="Recent History"
          loaded={props.seriesLoaded()}
          hasData={activity().hasData}
          emptyMessage={props.seriesFailed?.() ? 'Trend data unavailable' : 'No recovery activity yet'}
        >
          <div class="flex h-full flex-col gap-3">
            <div class="flex items-start justify-between gap-3">
              <div>
                <div class="text-2xl font-semibold tabular-nums text-base-content">
                  {activity().totalEvents}
                </div>
                <div class="text-xs text-muted">recovery points</div>
              </div>
              <Show when={recentWindowLabel()}>
                <div class="max-w-[9rem] text-right text-[11px] text-muted">
                  {recentWindowLabel()}
                </div>
              </Show>
            </div>

            <div class="grid grid-cols-3 gap-2 text-[11px]">
              <div class="rounded-md border border-border-subtle bg-surface-alt/35 px-2.5 py-1.5">
                <div class="text-muted">Days Active</div>
                <div class="mt-1 font-semibold text-base-content">{activity().activeDays}</div>
              </div>
              <div class="rounded-md border border-border-subtle bg-surface-alt/35 px-2.5 py-1.5">
                <div class="text-muted">Avg / Day</div>
                <div class="mt-1 font-semibold text-base-content">
                  {activity().averagePerDay.toFixed(1)}
                </div>
              </div>
              <div class="rounded-md border border-border-subtle bg-surface-alt/35 px-2.5 py-1.5">
                <div class="text-muted">Peak</div>
                <div class="mt-1 font-semibold text-base-content">{activity().busiestCount}</div>
              </div>
            </div>

            <dl class="space-y-1.5 border-t border-border-subtle pt-2 text-xs">
              <div class="flex items-center justify-between gap-3">
                <dt class="text-muted">Peak Day</dt>
                <dd class="font-medium text-base-content">{activity().busiestLabel ?? 'n/a'}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-muted">Latest Activity</dt>
                <dd class="font-medium text-base-content">{activity().latestLabel ?? 'n/a'}</dd>
              </div>
            </dl>
          </div>
        </SummaryMetricCard>
      </SummaryPanel>
    </Show>
  );
};
