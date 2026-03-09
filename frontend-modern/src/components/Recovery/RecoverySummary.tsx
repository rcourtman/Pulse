import { Component, For, Show, createMemo } from 'solid-js';
import { SummaryPanel } from '@/components/shared/SummaryPanel';
import { SummaryMetricCard } from '@/components/shared/SummaryMetricCard';
import type { ProtectionRollup, RecoveryOutcome, RecoveryPointsSeriesBucket } from '@/types/recovery';
import {
  buildRecoveryActivitySummary,
  buildRecoveryAttentionItems,
  buildRecoveryFreshnessBuckets,
  buildRecoveryOutcomeSegments,
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

function getAttentionToneClass(tone: 'rose' | 'amber' | 'blue'): string {
  switch (tone) {
    case 'rose':
      return 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-200';
    case 'blue':
      return 'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-200';
    default:
      return 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200';
  }
}

export const RecoverySummary: Component<RecoverySummaryProps> = (props) => {
  const summary = () => props.summary();
  const hasRollups = () => summary().total > 0;

  const outcomeSegments = createMemo(() => buildRecoveryOutcomeSegments(summary()));
  const freshnessBuckets = createMemo(() => buildRecoveryFreshnessBuckets(props.rollups()));
  const activity = createMemo(() => buildRecoveryActivitySummary(props.series()));
  const attentionItems = createMemo(() => buildRecoveryAttentionItems(summary()));
  const healthyCount = createMemo(() => summary().counts.success || 0);
  const issueCount = createMemo(
    () => (summary().counts.warning || 0) + (summary().counts.failed || 0) + summary().stale,
  );

  return (
    <Show when={hasRollups()}>
      <SummaryPanel
        testId="recovery-summary"
        headerLeft={
          <>
            <span class="font-medium text-base-content">{summary().total} protected</span>
            <span class="text-emerald-600 dark:text-emerald-400">{healthyCount()} healthy</span>
            <Show when={issueCount() > 0}>
              <span class="text-amber-600 dark:text-amber-400">
                {issueCount()} need attention
              </span>
            </Show>
            <Show when={summary().neverSucceeded > 0}>
              <span class="text-rose-600 dark:text-rose-400">
                {summary().neverSucceeded} never succeeded
              </span>
            </Show>
          </>
        }
        timeRange={props.timeRange()}
        timeRanges={RECOVERY_SUMMARY_TIME_RANGES}
        timeRangeLabels={RECOVERY_SUMMARY_TIME_RANGE_LABELS}
        onTimeRangeChange={
          props.onTimeRangeChange
            ? (range) => props.onTimeRangeChange!(range as RecoverySummaryTimeRange)
            : undefined
        }
      >
        <SummaryMetricCard label="Coverage" loaded={true} hasData={outcomeSegments().length > 0}>
          <div class="flex h-full flex-col justify-between gap-3">
            <div class="grid grid-cols-2 gap-2 text-sm">
              <div>
                <div class="text-[11px] uppercase tracking-wide text-muted">Protected</div>
                <div class="text-lg font-semibold text-base-content">{summary().total}</div>
              </div>
              <div>
                <div class="text-[11px] uppercase tracking-wide text-muted">Healthy</div>
                <div class="text-lg font-semibold text-emerald-600 dark:text-emerald-400">
                  {healthyCount()}
                </div>
              </div>
            </div>

            <div class="h-3 overflow-hidden rounded-full bg-surface-hover">
              <div class="flex h-full">
                <For each={outcomeSegments()}>
                  {(segment) => (
                    <div
                      class={segment.color}
                      style={{ width: `${Math.max(segment.percent, 0)}%` }}
                      title={`${segment.label}: ${segment.count}`}
                    />
                  )}
                </For>
              </div>
            </div>

            <div class="space-y-1">
              <For each={outcomeSegments()}>
                {(segment) => (
                  <div class="flex items-center justify-between gap-2 text-xs">
                    <div class="flex items-center gap-2">
                      <span class={`h-2 w-2 rounded-full ${segment.color}`} />
                      <span class="text-base-content">{segment.label}</span>
                    </div>
                    <span class={segment.textColor}>
                      {segment.count} ({segment.percent}%)
                    </span>
                  </div>
                )}
              </For>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard
          label="Freshness"
          loaded={true}
          hasData={freshnessBuckets().some((bucket) => bucket.count > 0)}
        >
          <div class="flex h-full flex-col justify-between gap-2">
            <div class="text-sm text-muted">Last successful recovery point by age.</div>
            <div class="space-y-2">
              <For each={freshnessBuckets()}>
                {(bucket) => (
                  <div class="grid grid-cols-[44px_minmax(0,1fr)_42px] items-center gap-2 text-xs">
                    <span class="text-base-content">{bucket.label}</span>
                    <div class="h-2 overflow-hidden rounded-full bg-surface-hover">
                      <div
                        class={`h-full rounded-full ${bucket.color}`}
                        style={{ width: `${Math.max(bucket.percent, bucket.count > 0 ? 8 : 0)}%` }}
                      />
                    </div>
                    <span class="text-right tabular-nums text-base-content">{bucket.count}</span>
                  </div>
                )}
              </For>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard
          label="Recent History"
          loaded={props.seriesLoaded()}
          hasData={activity().hasData}
          emptyMessage={props.seriesFailed?.() ? 'Trend data unavailable' : 'No recovery activity yet'}
        >
          <div class="flex h-full flex-col justify-between gap-3">
            <div class="grid grid-cols-3 gap-2 text-sm">
              <div>
                <div class="text-[11px] uppercase tracking-wide text-muted">Recovery Points</div>
                <div class="text-lg font-semibold text-base-content">{activity().totalEvents}</div>
              </div>
              <div>
                <div class="text-[11px] uppercase tracking-wide text-muted">Avg / Day</div>
                <div class="text-lg font-semibold text-base-content">
                  {activity().averagePerDay.toFixed(1)}
                </div>
              </div>
              <div>
                <div class="text-[11px] uppercase tracking-wide text-muted">Days Active</div>
                <div class="text-lg font-semibold text-base-content">{activity().activeDays}</div>
              </div>
            </div>

            <div class="flex h-14 items-end gap-1 overflow-hidden rounded bg-surface-hover px-1.5 py-1">
              <For each={activity().bars}>
                {(bar) => (
                  <div class="flex h-full flex-1 items-end">
                    <div
                      class={`w-full rounded-t-sm ${
                        bar.isPeak
                          ? 'bg-blue-600'
                          : bar.isLatest
                            ? 'bg-blue-400'
                            : 'bg-blue-200 dark:bg-blue-900'
                      }`}
                      style={{ height: `${bar.heightPct}%` }}
                      title={`${bar.day}: ${bar.total} recovery points`}
                    />
                  </div>
                )}
              </For>
            </div>

            <div class="flex items-center justify-between gap-2 text-xs text-muted">
              <span>
                Peak: {activity().busiestLabel ?? 'n/a'} ({activity().busiestCount})
              </span>
              <span>
                Latest: {activity().latestLabel ?? 'n/a'} ({activity().latestCount})
              </span>
            </div>
          </div>
        </SummaryMetricCard>

        <SummaryMetricCard label="Attention" loaded={true} hasData={attentionItems().length > 0}>
          <div class="flex h-full flex-col gap-2">
            <For each={attentionItems().slice(0, 3)}>
              {(item) => (
                <div class={`rounded-md border px-2.5 py-2 text-xs ${getAttentionToneClass(item.tone)}`}>
                  <div class="flex items-center justify-between gap-2">
                    <span class="font-medium">{item.label}</span>
                    <span class="tabular-nums font-semibold">{item.count}</span>
                  </div>
                  <div class="mt-1 opacity-80">{item.detail}</div>
                </div>
              )}
            </For>
            <Show when={attentionItems().length > 3}>
              <div class="text-xs text-muted">+{attentionItems().length - 3} more attention items</div>
            </Show>
          </div>
        </SummaryMetricCard>
      </SummaryPanel>
    </Show>
  );
};
