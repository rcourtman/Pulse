import { Component, For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { segmentedButtonClass } from '@/utils/segmentedButton';
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
      <Card padding="sm" class="border-border-subtle" data-testid="recovery-summary">
        <div class="flex flex-col gap-3">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div class="space-y-1">
              <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                Recovery Overview
              </div>
              <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
                <span class="font-medium text-base-content">
                  {summary().total} protected items
                </span>
                <span class="text-emerald-600 dark:text-emerald-400">
                  {healthyCount()} healthy
                </span>
                <Show when={summary().stale > 0}>
                  <span class="text-amber-600 dark:text-amber-400">
                    {summary().stale} stale
                  </span>
                </Show>
                <Show when={summary().neverSucceeded > 0}>
                  <span class="text-rose-600 dark:text-rose-400">
                    {summary().neverSucceeded} never succeeded
                  </span>
                </Show>
              </div>
            </div>

            <Show when={props.onTimeRangeChange}>
              <div class="inline-flex shrink-0 rounded border border-border bg-surface p-0.5 text-xs">
                <For each={RECOVERY_SUMMARY_TIME_RANGES}>
                  {(range) => (
                    <button
                      type="button"
                      onClick={() => props.onTimeRangeChange?.(range)}
                      class={`px-2 py-1 ${segmentedButtonClass(props.timeRange() === range, false, 'accent')}`}
                    >
                      {RECOVERY_SUMMARY_TIME_RANGE_LABELS[range]}
                    </button>
                  )}
                </For>
              </div>
            </Show>
          </div>

          <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
            <div class="rounded-md border border-border-subtle bg-surface-alt px-3 py-2">
              <div class="text-[11px] uppercase tracking-wide text-muted">Protected</div>
              <div class="mt-1 text-2xl font-semibold text-base-content">{summary().total}</div>
              <div class="text-xs text-muted">Items with recovery evidence in Pulse</div>
            </div>
            <div class="rounded-md border border-border-subtle bg-surface-alt px-3 py-2">
              <div class="text-[11px] uppercase tracking-wide text-muted">Healthy</div>
              <div class="mt-1 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
                {healthyCount()}
              </div>
              <div class="text-xs text-muted">Latest result succeeded</div>
            </div>
            <div class="rounded-md border border-border-subtle bg-surface-alt px-3 py-2">
              <div class="text-[11px] uppercase tracking-wide text-muted">Needs Attention</div>
              <div class="mt-1 text-2xl font-semibold text-amber-600 dark:text-amber-400">
                {issueCount()}
              </div>
              <div class="text-xs text-muted">Warnings, failures, or stale protection</div>
            </div>
            <div class="rounded-md border border-border-subtle bg-surface-alt px-3 py-2">
              <div class="text-[11px] uppercase tracking-wide text-muted">Never Succeeded</div>
              <div class="mt-1 text-2xl font-semibold text-rose-600 dark:text-rose-400">
                {summary().neverSucceeded}
              </div>
              <div class="text-xs text-muted">Items without a successful run yet</div>
            </div>
          </div>

          <div class="grid gap-3 xl:grid-cols-[minmax(0,1.45fr)_minmax(320px,1fr)]">
            <div class="space-y-3">
              <div class="rounded-md border border-border-subtle bg-surface-alt p-3">
                <div class="flex items-center justify-between gap-2">
                  <div>
                    <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                      Protection Posture
                    </div>
                    <div class="text-sm text-muted">
                      The latest known state for every protected item.
                    </div>
                  </div>
                </div>

                <div class="mt-3 h-4 overflow-hidden rounded-full bg-surface">
                  <div class="flex h-full">
                    <For each={outcomeSegments()}>
                      {(segment) => (
                        <div
                          class={segment.color}
                          style={{ width: `${segment.percent}%` }}
                          title={`${segment.label}: ${segment.count}`}
                        />
                      )}
                    </For>
                  </div>
                </div>

                <div class="mt-3 grid gap-2 sm:grid-cols-2">
                  <For each={outcomeSegments()}>
                    {(segment) => (
                      <div class="rounded-md border border-border bg-surface px-3 py-2">
                        <div class="flex items-center justify-between gap-3">
                          <div class="flex items-center gap-2">
                            <span class={`h-2.5 w-2.5 rounded-full ${segment.color}`} />
                            <span class="text-sm font-medium text-base-content">{segment.label}</span>
                          </div>
                          <span class={`text-sm font-medium ${segment.textColor}`}>
                            {segment.count}
                          </span>
                        </div>
                        <div class="mt-1 text-xs text-muted">{segment.percent}% of protected items</div>
                      </div>
                    )}
                  </For>
                </div>
              </div>

              <div class="rounded-md border border-border-subtle bg-surface-alt p-3">
                <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                  Freshness Distribution
                </div>
                <div class="mt-1 text-sm text-muted">
                  When the last successful recovery point landed.
                </div>

                <div class="mt-3 space-y-2">
                  <For each={freshnessBuckets()}>
                    {(bucket) => (
                      <div class="grid grid-cols-[56px_minmax(0,1fr)_46px] items-center gap-3">
                        <div class="text-sm font-medium text-base-content">{bucket.label}</div>
                        <div class="h-2 overflow-hidden rounded-full bg-surface">
                          <div
                            class={`h-full rounded-full ${bucket.color}`}
                            style={{ width: `${Math.max(bucket.percent, bucket.count > 0 ? 6 : 0)}%` }}
                          />
                        </div>
                        <div class="text-right text-sm tabular-nums text-base-content">
                          {bucket.count}
                        </div>
                      </div>
                    )}
                  </For>
                </div>
              </div>
            </div>

            <div class="space-y-3">
              <div class="rounded-md border border-border-subtle bg-surface-alt p-3">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                      Activity
                    </div>
                    <div class="text-sm text-muted">
                      Daily recovery evidence across the selected window.
                    </div>
                  </div>
                  <Show when={activity().hasData}>
                    <div class="text-right text-xs text-muted">
                      <div>{activity().startLabel}</div>
                      <div>{activity().endLabel}</div>
                    </div>
                  </Show>
                </div>

                <Show
                  when={activity().hasData}
                  fallback={
                    <div class="mt-4 rounded-md border border-dashed border-border px-3 py-4 text-sm text-muted">
                      <Show when={props.seriesLoaded()} fallback={'Loading activity...'}>
                        <Show when={props.seriesFailed?.()} fallback={'No recovery activity yet.'}>
                          Trend data unavailable
                        </Show>
                      </Show>
                    </div>
                  }
                >
                  <div class="mt-3 grid grid-cols-3 gap-2">
                    <div class="rounded-md border border-border bg-surface px-3 py-2">
                      <div class="text-[11px] uppercase tracking-wide text-muted">Events</div>
                      <div class="mt-1 text-lg font-semibold text-base-content">
                        {activity().totalEvents}
                      </div>
                    </div>
                    <div class="rounded-md border border-border bg-surface px-3 py-2">
                      <div class="text-[11px] uppercase tracking-wide text-muted">Avg / Day</div>
                      <div class="mt-1 text-lg font-semibold text-base-content">
                        {activity().averagePerDay.toFixed(1)}
                      </div>
                    </div>
                    <div class="rounded-md border border-border bg-surface px-3 py-2">
                      <div class="text-[11px] uppercase tracking-wide text-muted">Active Days</div>
                      <div class="mt-1 text-lg font-semibold text-base-content">
                        {activity().activeDays}
                      </div>
                    </div>
                  </div>

                  <div class="mt-3">
                    <div class="flex h-28 items-end gap-1 overflow-hidden rounded-md border border-border bg-surface px-2 py-2">
                      <For each={activity().bars}>
                        {(bar) => (
                          <div class="flex h-full flex-1 items-end">
                            <div
                              class={`w-full rounded-t-sm transition-all ${
                                bar.isPeak
                                  ? 'bg-blue-600'
                                  : bar.isLatest
                                    ? 'bg-blue-400'
                                    : 'bg-blue-200 dark:bg-blue-900'
                              }`}
                              style={{ height: `${bar.heightPct}%` }}
                              title={`${bar.day}: ${bar.total} events`}
                            />
                          </div>
                        )}
                      </For>
                    </div>
                    <div class="mt-2 flex items-center justify-between text-xs text-muted">
                      <span>
                        Busiest day: {activity().busiestLabel} ({activity().busiestCount})
                      </span>
                      <span>
                        Latest: {activity().latestLabel} ({activity().latestCount})
                      </span>
                    </div>
                  </div>
                </Show>
              </div>

              <div class="rounded-md border border-border-subtle bg-surface-alt p-3">
                <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                  Attention Queue
                </div>
                <div class="mt-1 text-sm text-muted">
                  The things I would look at first before touching history.
                </div>

                <Show
                  when={attentionItems().length > 0}
                  fallback={
                    <div class="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-3 text-sm text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200">
                      Nothing is asking for immediate attention right now.
                    </div>
                  }
                >
                  <div class="mt-3 space-y-2">
                    <For each={attentionItems()}>
                      {(item) => (
                        <div class={`rounded-md border px-3 py-2 ${getAttentionToneClass(item.tone)}`}>
                          <div class="flex items-center justify-between gap-3">
                            <span class="text-sm font-medium">{item.label}</span>
                            <span class="text-sm font-semibold tabular-nums">{item.count}</span>
                          </div>
                          <div class="mt-1 text-xs opacity-80">{item.detail}</div>
                        </div>
                      )}
                    </For>
                  </div>
                </Show>
              </div>
            </div>
          </div>
        </div>
      </Card>
    </Show>
  );
};
