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
  getRecoveryAttentionDotClass,
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
  const recentWindowLabel = createMemo(() => {
    const activitySummary = activity();
    if (!activitySummary.startLabel || !activitySummary.endLabel) return null;
    return `${activitySummary.startLabel} to ${activitySummary.endLabel}`;
  });
  const attentionItems = createMemo(() =>
    [
      {
        label: 'Stale',
        count: summary().stale,
        tone: 'amber',
      },
      {
        label: 'Never succeeded',
        count: summary().neverSucceeded,
        tone: 'rose',
      },
      {
        label: 'Attention',
        count: attentionCount(),
        tone: 'amber',
      },
      {
        label: 'Running',
        count: postureSummary().running,
        tone: 'blue',
      },
    ].filter((item) => item.count > 0),
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
            <span class="text-emerald-600 dark:text-emerald-400">
              {healthyCount()} healthy
            </span>
            <Show when={attentionCount() > 0}>
              <span class="text-amber-600 dark:text-amber-400">
                {attentionCount()} attention
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
      >
        <SummaryMetricCard label="Recovery Posture" loaded={true} hasData={hasRollups()}>
          <div class="flex h-full flex-col gap-3">
            <div class="grid grid-cols-3 gap-2 border-b border-border-subtle pb-2.5 text-sm">
              <div>
                <div class="text-[11px] uppercase tracking-wide text-muted">Healthy</div>
                <div class="mt-1 text-xl font-semibold text-emerald-600 dark:text-emerald-400">
                  {healthyCount()}
                </div>
              </div>
              <div>
                <div class="text-[11px] uppercase tracking-wide text-muted">Attention</div>
                <div class="mt-1 text-xl font-semibold text-amber-600 dark:text-amber-400">
                  {attentionCount()}
                </div>
              </div>
              <div>
                <div class="text-[11px] uppercase tracking-wide text-muted">Protected</div>
                <div class="mt-1 text-xl font-semibold text-base-content">{summary().total}</div>
              </div>
            </div>

            <div class="space-y-2">
              <div class="h-2 overflow-hidden rounded-full bg-surface-alt">
                <div class="flex h-full">
                  <For each={postureSegments()}>
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
              <div class="grid grid-cols-2 gap-x-3 gap-y-1.5">
                <For each={postureSegments()}>
                  {(segment) => (
                    <div class="flex items-center justify-between gap-3 text-sm">
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
            <dl class="space-y-2 text-sm">
              <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Stale</dt>
                <dd class="font-semibold text-amber-600 dark:text-amber-400">{summary().stale}</dd>
              </div>
              <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Never succeeded</dt>
                <dd class="font-semibold text-rose-600 dark:text-rose-400">
                  {summary().neverSucceeded}
                </dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Running</dt>
                <dd class="font-semibold text-blue-600 dark:text-blue-400">
                  {postureSummary().running}
                </dd>
              </div>
            </dl>

            <div>
              <div class="mb-2 text-[11px] font-medium uppercase tracking-wide text-muted">
                Attention Queue
              </div>
              <div class="space-y-1.5">
                <For each={attentionItems()}>
                  {(item) => (
                    <div class="flex items-center justify-between gap-3 text-sm">
                      <div class="flex items-center gap-2 text-base-content">
                        <span
                          class={`h-2.5 w-2.5 rounded-full ${getRecoveryAttentionDotClass(item.tone)}`}
                        />
                        <span>{item.label}</span>
                      </div>
                      <span
                        class={`font-semibold tabular-nums ${
                          item.tone === 'rose'
                            ? 'text-rose-600 dark:text-rose-400'
                            : item.tone === 'blue'
                              ? 'text-blue-600 dark:text-blue-400'
                              : 'text-amber-600 dark:text-amber-400'
                        }`}
                      >
                        {item.count}
                      </span>
                    </div>
                  )}
                </For>
              </div>
            </div>

            <div class="border-t border-border-subtle pt-2.5">
              <div class="mb-2 text-[11px] font-medium uppercase tracking-wide text-muted">
                Freshness Bands
              </div>
              <div class="flex flex-wrap gap-1.5">
                <For each={freshnessBuckets()}>
                  {(bucket) => (
                    <div class="inline-flex items-center gap-2 rounded-md border border-border-subtle bg-surface-alt/35 px-2 py-1 text-xs">
                      <span class={`h-2 w-2 rounded-full ${bucket.color}`} />
                      <span class="text-base-content">{bucket.label}</span>
                      <span class="tabular-nums text-base-content">{bucket.count}</span>
                    </div>
                  )}
                </For>
              </div>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard label="Protected Footprint" loaded={true} hasData={hasRollups()}>
          <div class="flex h-full flex-col gap-2.5">
            <dl class="space-y-2 text-sm">
              <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Item Types</dt>
                <dd class="font-semibold text-base-content">{itemCoverage().itemTypeCount}</dd>
              </div>
              <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Primary Item</dt>
                <dd class="font-semibold text-base-content">
                  {itemCoverage().primaryItemLabel ?? 'n/a'}
                </dd>
              </div>
              <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Platforms</dt>
                <dd class="font-semibold text-base-content">{platformCoverage().platformCount}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Primary Platform</dt>
                <dd class="font-semibold text-base-content">
                  {platformCoverage().primaryPlatformLabel ?? 'n/a'}
                </dd>
              </div>
            </dl>

            <Show when={itemCoverage().items.length > 0}>
              <div>
                <div class="mb-2 text-[11px] font-medium uppercase tracking-wide text-muted">
                  Item Types
                </div>
                <div class="flex flex-wrap gap-1.5">
                  <For each={itemCoverage().items.slice(0, 6)}>
                    {(item) => (
                      <div class="inline-flex items-center gap-2 text-[10px]">
                        <span class={item.toneClass}>{item.label}</span>
                        <span class="tabular-nums text-base-content">{item.count}</span>
                        <span class="text-muted">{item.percent}%</span>
                      </div>
                    )}
                  </For>
                </div>
              </div>
            </Show>

            <div>
              <div class="mb-2 text-[11px] font-medium uppercase tracking-wide text-muted">
                Platform Mix
              </div>
              <Show when={platformCoverage().multiPlatformCount > 0}>
                <div class="mb-2 text-xs text-muted">
                  {platformCoverage().multiPlatformCount} multi-platform item
                  {platformCoverage().multiPlatformCount === 1 ? '' : 's'}
                </div>
              </Show>
              <div class="flex flex-wrap gap-1.5">
                <For each={platformCoverage().items.slice(0, 6)}>
                  {(item) => {
                    const badge = getSourcePlatformBadge(item.key);
                    return (
                      <div class="inline-flex items-center gap-2 text-[10px]">
                        <span class={badge?.classes || 'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-base-content'}>
                          {badge?.label || item.label}
                        </span>
                        <span class="tabular-nums text-base-content">{item.count}</span>
                        <span class="text-muted">{item.percent}%</span>
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
          <div class="flex h-full flex-col gap-2.5">
            <dl class="space-y-2 text-sm">
              <Show when={recentWindowLabel()}>
                <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
                  <dt class="text-[11px] uppercase tracking-wide text-muted">Window</dt>
                  <dd class="text-right text-xs text-base-content">{recentWindowLabel()}</dd>
                </div>
              </Show>
              <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Recovery Points</dt>
                <dd class="font-semibold text-base-content">{activity().totalEvents}</dd>
              </div>
              <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Avg / Day</dt>
                <dd class="font-semibold text-base-content">{activity().averagePerDay.toFixed(1)}</dd>
              </div>
              <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Days Active</dt>
                <dd class="font-semibold text-base-content">{activity().activeDays}</dd>
              </div>
              <div class="flex items-center justify-between gap-3 border-b border-border-subtle pb-2">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Peak Day</dt>
                <dd class="font-semibold text-base-content">{activity().busiestLabel ?? 'n/a'}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-[11px] uppercase tracking-wide text-muted">Latest Activity</dt>
                <dd class="font-semibold text-base-content">{activity().latestLabel ?? 'n/a'}</dd>
              </div>
            </dl>
            <div class="flex flex-wrap gap-2 border-t border-border-subtle pt-3 text-xs">
              <div class="rounded-md border border-border-subtle bg-surface-alt/35 px-2.5 py-1">
                <span class="text-muted">Peak Throughput</span>
                <span class="ml-2 font-semibold text-base-content">{activity().busiestCount}</span>
              </div>
              <div class="rounded-md border border-border-subtle bg-surface-alt/35 px-2.5 py-1">
                <span class="text-muted">Latest Throughput</span>
                <span class="ml-2 font-semibold text-base-content">{activity().latestCount}</span>
              </div>
            </div>
          </div>
        </SummaryMetricCard>
      </SummaryPanel>
    </Show>
  );
};
