import { Component, For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import type { ProtectionRollup, RecoveryOutcome, RecoveryPointsSeriesBucket } from '@/types/recovery';
import {
  buildRecoveryActivitySummary,
  buildRecoveryAttentionItems,
  buildRecoveryFreshnessBuckets,
  buildRecoveryPostureSegments,
  buildRecoveryPostureSummary,
  RECOVERY_SUMMARY_TIME_RANGES,
  RECOVERY_SUMMARY_TIME_RANGE_LABELS,
  type RecoverySummaryTimeRange,
} from '@/utils/recoverySummaryPresentation';
import { segmentedButtonClass } from '@/utils/segmentedButton';

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

function getAttentionChipClass(tone: 'rose' | 'amber' | 'blue'): string {
  switch (tone) {
    case 'rose':
      return 'border-rose-500/30 bg-rose-500/10 text-rose-200';
    case 'blue':
      return 'border-blue-500/30 bg-blue-500/10 text-blue-200';
    default:
      return 'border-amber-500/30 bg-amber-500/10 text-amber-200';
  }
}

function getAttentionDotClass(tone: 'rose' | 'amber' | 'blue'): string {
  switch (tone) {
    case 'rose':
      return 'bg-rose-400';
    case 'blue':
      return 'bg-blue-400';
    default:
      return 'bg-amber-400';
  }
}

export const RecoverySummary: Component<RecoverySummaryProps> = (props) => {
  const summary = () => props.summary();
  const hasRollups = () => summary().total > 0;

  const postureSummary = createMemo(() => buildRecoveryPostureSummary(props.rollups()));
  const postureSegments = createMemo(() => buildRecoveryPostureSegments(props.rollups()));
  const freshnessBuckets = createMemo(() => buildRecoveryFreshnessBuckets(props.rollups()));
  const activity = createMemo(() => buildRecoveryActivitySummary(props.series()));
  const attentionItems = createMemo(() => buildRecoveryAttentionItems(summary()));

  const healthyCount = createMemo(() => postureSummary().healthy);
  const attentionCount = createMemo(() => postureSummary().attention);

  return (
    <Show when={hasRollups()}>
      <Card padding="sm" class="overflow-hidden" data-testid="recovery-summary">
        <div class="flex flex-col gap-3">
          <div class="flex flex-wrap items-center justify-between gap-2 border-b border-border-subtle px-1 pb-2 text-[11px]">
            <div class="flex flex-wrap items-center gap-3">
              <span class="font-medium text-base-content">{summary().total} protected</span>
              <span class="text-emerald-400">{healthyCount()} healthy</span>
              <Show when={attentionCount() > 0}>
                <span class="text-amber-300">{attentionCount()} attention</span>
              </Show>
              <Show when={postureSummary().running > 0}>
                <span class="text-blue-300">{postureSummary().running} running</span>
              </Show>
            </div>
            <Show when={props.onTimeRangeChange}>
              <div class="inline-flex shrink-0 rounded border border-border bg-surface p-0.5 text-xs">
                <For each={RECOVERY_SUMMARY_TIME_RANGES}>
                  {(range) => (
                    <button
                      type="button"
                      onClick={() => props.onTimeRangeChange?.(range)}
                      class={segmentedButtonClass(props.timeRange() === range)}
                    >
                      {RECOVERY_SUMMARY_TIME_RANGE_LABELS[range]}
                    </button>
                  )}
                </For>
              </div>
            </Show>
          </div>

          <div class="grid gap-3 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,0.9fr)_minmax(0,1fr)]">
            <section class="rounded-md border border-border bg-surface-hover/40 p-3">
              <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                Recovery Posture
              </div>
              <div class="mt-3 grid gap-3 sm:grid-cols-3">
                <div>
                  <div class="text-[10px] uppercase tracking-wide text-muted">Protected</div>
                  <div class="mt-1 text-2xl font-semibold text-base-content">{summary().total}</div>
                </div>
                <div>
                  <div class="text-[10px] uppercase tracking-wide text-muted">Healthy</div>
                  <div class="mt-1 text-2xl font-semibold text-emerald-400">{healthyCount()}</div>
                </div>
                <div>
                  <div class="text-[10px] uppercase tracking-wide text-muted">Attention</div>
                  <div class="mt-1 text-2xl font-semibold text-amber-300">{attentionCount()}</div>
                </div>
              </div>

              <div class="mt-3 h-3 overflow-hidden rounded-full bg-surface-alt">
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

              <div class="mt-3 grid gap-2 sm:grid-cols-2">
                <For each={postureSegments()}>
                  {(segment) => (
                    <div class="flex items-center justify-between gap-2 rounded border border-border-subtle bg-surface/60 px-2.5 py-1.5 text-xs">
                      <div class="flex items-center gap-2">
                        <span class={`h-2 w-2 rounded-full ${segment.color}`} />
                        <span class="text-base-content">{segment.label}</span>
                      </div>
                      <span class={segment.textColor}>{segment.count}</span>
                    </div>
                  )}
                </For>
              </div>
            </section>

            <section class="rounded-md border border-border bg-surface-hover/40 p-3">
              <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                Freshness
              </div>
              <div class="mt-3 space-y-2">
                <For each={freshnessBuckets()}>
                  {(bucket) => (
                    <div class="grid grid-cols-[44px_minmax(0,1fr)_28px] items-center gap-2 text-xs">
                      <span class="text-base-content">{bucket.label}</span>
                      <div class="h-2 overflow-hidden rounded-full bg-surface-alt">
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
            </section>

            <section class="rounded-md border border-border bg-surface-hover/40 p-3">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                    Recent History
                  </div>
                  <div class="mt-3 grid grid-cols-3 gap-3 text-sm">
                    <div>
                      <div class="text-[10px] uppercase tracking-wide text-muted">
                        Recovery Points
                      </div>
                      <div class="mt-1 text-2xl font-semibold text-base-content">
                        {activity().totalEvents}
                      </div>
                    </div>
                    <div>
                      <div class="text-[10px] uppercase tracking-wide text-muted">Avg / Day</div>
                      <div class="mt-1 text-2xl font-semibold text-base-content">
                        {activity().averagePerDay.toFixed(1)}
                      </div>
                    </div>
                    <div>
                      <div class="text-[10px] uppercase tracking-wide text-muted">Days Active</div>
                      <div class="mt-1 text-2xl font-semibold text-base-content">
                        {activity().activeDays}
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <Show
                when={activity().hasData}
                fallback={
                  <div class="mt-6 text-sm text-muted">
                    {props.seriesFailed?.() ? 'Trend data unavailable' : 'No recovery activity yet'}
                  </div>
                }
              >
                <div class="mt-4 flex h-16 items-end gap-1 overflow-hidden rounded bg-surface-alt px-1.5 py-1">
                  <For each={activity().bars}>
                    {(bar) => (
                      <div class="flex h-full flex-1 items-end">
                        <div
                          class={`w-full rounded-t-sm ${
                            bar.isPeak
                              ? 'bg-blue-500'
                              : bar.isLatest
                                ? 'bg-blue-400'
                                : 'bg-blue-900/60'
                          }`}
                          style={{ height: `${bar.heightPct}%` }}
                          title={`${bar.day}: ${bar.total} recovery points`}
                        />
                      </div>
                    )}
                  </For>
                </div>

                <div class="mt-3 flex items-center justify-between gap-2 text-xs text-muted">
                  <span>
                    Peak: {activity().busiestLabel ?? 'n/a'} ({activity().busiestCount})
                  </span>
                  <span>
                    Latest: {activity().latestLabel ?? 'n/a'} ({activity().latestCount})
                  </span>
                </div>
              </Show>
            </section>
          </div>

          <Show when={attentionItems().length > 0}>
            <div class="flex flex-wrap items-center gap-2 border-t border-border-subtle px-1 pt-3">
              <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                Attention Queue
              </div>
              <For each={attentionItems().slice(0, 5)}>
                {(item) => (
                  <div
                    class={`inline-flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs ${getAttentionChipClass(item.tone)}`}
                  >
                    <span class={`h-2 w-2 rounded-full ${getAttentionDotClass(item.tone)}`} />
                    <span class="font-medium">{item.label}</span>
                    <span class="tabular-nums">{item.count}</span>
                  </div>
                )}
              </For>
            </div>
          </Show>
        </div>
      </Card>
    </Show>
  );
};

