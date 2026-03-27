import { Component, For, Show, createMemo } from 'solid-js';
import { Card } from '@/components/shared/Card';
import type { ProtectionRollup, RecoveryOutcome, RecoveryPointsSeriesBucket } from '@/types/recovery';
import {
  buildRecoveryActivitySummary,
  buildRecoveryAttentionItems,
  buildRecoveryFreshnessBuckets,
  buildRecoveryItemCoverage,
  buildRecoveryPlatformCoverage,
  buildRecoveryPostureSegments,
  buildRecoveryPostureSummary,
  getRecoveryAttentionChipClass,
  getRecoveryAttentionDotClass,
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

export const RecoverySummary: Component<RecoverySummaryProps> = (props) => {
  const summary = () => props.summary();
  const hasRollups = () => summary().total > 0;

  const postureSummary = createMemo(() => buildRecoveryPostureSummary(props.rollups()));
  const postureSegments = createMemo(() => buildRecoveryPostureSegments(props.rollups()));
  const freshnessBuckets = createMemo(() => buildRecoveryFreshnessBuckets(props.rollups()));
  const itemCoverage = createMemo(() => buildRecoveryItemCoverage(props.rollups()));
  const platformCoverage = createMemo(() => buildRecoveryPlatformCoverage(props.rollups()));
  const activity = createMemo(() => buildRecoveryActivitySummary(props.series()));
  const attentionItems = createMemo(() => buildRecoveryAttentionItems(summary()));

  const healthyCount = createMemo(() => postureSummary().healthy);
  const attentionCount = createMemo(() => postureSummary().attention);

  return (
    <Show when={hasRollups()}>
      <Card
        padding="none"
        class="overflow-hidden border-border bg-surface-hover shadow-[0_14px_34px_rgba(2,6,23,0.14)]"
        data-testid="recovery-summary"
      >
        <div class="flex flex-col gap-4 p-3 sm:p-4">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-border-subtle/80 bg-surface pb-3 text-[11px]">
            <div class="flex flex-wrap items-center gap-2.5">
              <span class="inline-flex items-center gap-2 rounded-full border border-border-subtle bg-surface/70 px-2.5 py-1 font-medium text-base-content">
                <span>{summary().total} protected</span>
              </span>
              <span class="inline-flex items-center gap-2 rounded-full border border-violet-500/25 bg-violet-500/8 px-2.5 py-1 text-violet-200">
                <span>
                  {itemCoverage().itemTypeCount} item type
                  {itemCoverage().itemTypeCount === 1 ? '' : 's'}
                </span>
              </span>
              <span class="inline-flex items-center gap-2 rounded-full border border-sky-500/25 bg-sky-500/8 px-2.5 py-1 text-sky-200">
                <span>
                  {platformCoverage().platformCount} platform
                  {platformCoverage().platformCount === 1 ? '' : 's'}
                </span>
              </span>
              <span class="inline-flex items-center gap-2 rounded-full border border-emerald-500/25 bg-emerald-500/8 px-2.5 py-1 text-emerald-300">
                <span>{healthyCount()} healthy</span>
              </span>
              <Show when={attentionCount() > 0}>
                <span class="inline-flex items-center gap-2 rounded-full border border-amber-500/30 bg-amber-500/10 px-2.5 py-1 text-amber-200">
                  <span>{attentionCount()} attention</span>
                </span>
              </Show>
              <Show when={postureSummary().running > 0}>
                <span class="inline-flex items-center gap-2 rounded-full border border-blue-500/30 bg-blue-500/10 px-2.5 py-1 text-blue-200">
                  <span>{postureSummary().running} running</span>
                </span>
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

          <div class="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(22rem,0.85fr)]">
            <section class="rounded-xl border border-border bg-surface p-4 shadow-[inset_0_1px_0_rgba(148,163,184,0.04)]">
              <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-400">
                    Recovery Posture
                  </div>
                  <div class="mt-2 max-w-xl text-sm text-slate-300">
                    Recovery readiness across the current protected estate, with unhealthy
                    coverage and stale protection surfaced first.
                  </div>
                </div>
                <Show when={attentionItems().length > 0}>
                  <div class="inline-flex w-fit items-center gap-2 rounded-full border border-amber-500/25 bg-amber-500/8 px-3 py-1.5 text-xs text-amber-200">
                    <span class="font-semibold uppercase tracking-[0.16em]">Attention Queue</span>
                    <span class="rounded-full bg-amber-500/15 px-2 py-0.5 tabular-nums">
                      {attentionCount()}
                    </span>
                  </div>
                </Show>
              </div>

              <div class="mt-5 grid gap-3 sm:grid-cols-3">
                <div>
                  <div class="text-[10px] uppercase tracking-wide text-muted">Protected</div>
                  <div class="mt-1 text-3xl font-semibold text-base-content">{summary().total}</div>
                </div>
                <div>
                  <div class="text-[10px] uppercase tracking-wide text-muted">Healthy</div>
                  <div class="mt-1 text-3xl font-semibold text-emerald-400">{healthyCount()}</div>
                </div>
                <div>
                  <div class="text-[10px] uppercase tracking-wide text-muted">Attention</div>
                  <div class="mt-1 text-3xl font-semibold text-amber-300">{attentionCount()}</div>
                </div>
              </div>

              <div class="mt-5 h-3 overflow-hidden rounded-full bg-surface-alt/90">
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

              <div class="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
                <For each={postureSegments()}>
                  {(segment) => (
                    <div class="flex items-center justify-between gap-2 rounded-lg border border-border-subtle bg-surface px-3 py-2 text-xs">
                      <div class="flex items-center gap-2">
                        <span class={`h-2 w-2 rounded-full ${segment.color}`} />
                        <span class="text-base-content">{segment.label}</span>
                      </div>
                      <span class={segment.textColor}>{segment.count}</span>
                    </div>
                  )}
                </For>
              </div>
              <div class="mt-5 grid gap-4 lg:grid-cols-[minmax(0,0.78fr)_minmax(0,1fr)]">
                <div class="rounded-lg border border-border-subtle bg-surface p-3">
                  <div class="text-[10px] font-semibold uppercase tracking-[0.16em] text-muted">
                    Freshness
                  </div>
                  <div class="mt-3 space-y-2.5">
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
                </div>

                <div class="rounded-lg border border-border-subtle bg-surface p-3">
                  <div class="text-[10px] font-semibold uppercase tracking-[0.16em] text-muted">
                    Attention Queue
                  </div>
                  <Show
                    when={attentionItems().length > 0}
                    fallback={<div class="mt-3 text-sm text-emerald-300">No active recovery risks.</div>}
                  >
                    <div class="mt-3 grid gap-2 sm:grid-cols-2">
                      <For each={attentionItems().slice(0, 4)}>
                        {(item) => (
                          <div
                            class={`rounded-lg border px-3 py-2.5 ${getRecoveryAttentionChipClass(item.tone)}`}
                          >
                            <div class="flex items-center justify-between gap-2">
                              <div class="flex items-center gap-2">
                                <span
                                  class={`h-2.5 w-2.5 rounded-full ${getRecoveryAttentionDotClass(item.tone)}`}
                                />
                                <span class="text-xs font-semibold uppercase tracking-[0.14em]">
                                  {item.label}
                                </span>
                              </div>
                              <span class="text-sm font-semibold tabular-nums">{item.count}</span>
                            </div>
                            <div class="mt-1.5 text-xs leading-5 text-current/80">{item.detail}</div>
                          </div>
                        )}
                      </For>
                    </div>
                  </Show>
                </div>
              </div>
            </section>

            <div class="grid gap-4">
              <section class="rounded-xl border border-border bg-surface p-4">
                <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                  Protected Footprint
                </div>
                <div class="mt-4 grid gap-3 sm:grid-cols-2">
                  <div class="rounded-lg border border-border-subtle bg-surface p-3">
                    <div class="text-[10px] uppercase tracking-wide text-muted">Item Types</div>
                    <div class="mt-1 text-3xl font-semibold text-base-content">
                      {itemCoverage().itemTypeCount}
                    </div>
                  </div>
                  <div class="rounded-lg border border-border-subtle bg-surface p-3">
                    <div class="text-[10px] uppercase tracking-wide text-muted">Primary Item</div>
                    <div class="mt-1 text-xl font-semibold text-base-content">
                      {itemCoverage().primaryItemLabel ?? 'n/a'}
                    </div>
                  </div>
                  <div class="rounded-lg border border-border-subtle bg-surface p-3">
                    <div class="text-[10px] uppercase tracking-wide text-muted">Platforms</div>
                    <div class="mt-1 text-3xl font-semibold text-base-content">
                      {platformCoverage().platformCount}
                    </div>
                  </div>
                  <div class="rounded-lg border border-border-subtle bg-surface p-3">
                    <div class="text-[10px] uppercase tracking-wide text-muted">Primary Platform</div>
                    <div class="mt-1 text-xl font-semibold text-base-content">
                      {platformCoverage().primaryPlatformLabel ?? 'n/a'}
                    </div>
                  </div>
                </div>

                <div class="mt-4 space-y-4">
                  <Show when={itemCoverage().items.length > 0}>
                    <div>
                      <div class="mb-2 text-[10px] font-semibold uppercase tracking-wide text-muted">
                        Item Types
                      </div>
                      <div class="flex flex-wrap gap-2">
                        <For each={itemCoverage().items.slice(0, 6)}>
                          {(item) => (
                            <div class="inline-flex items-center gap-2 rounded-md border border-border-subtle bg-surface px-2.5 py-1.5 text-xs">
                              <span class={`rounded px-1.5 py-0.5 text-[10px] font-medium ${item.toneClass}`}>
                                {item.label}
                              </span>
                              <span class="tabular-nums text-base-content">{item.count}</span>
                              <span class="text-muted">{item.percent}%</span>
                            </div>
                          )}
                        </For>
                      </div>
                    </div>
                  </Show>
                  <div>
                    <div class="mb-2 text-[10px] font-semibold uppercase tracking-wide text-muted">
                      Platform Mix
                    </div>
                    <Show when={platformCoverage().multiPlatformCount > 0}>
                      <div class="mb-2 text-xs text-muted">
                        {platformCoverage().multiPlatformCount} protected item
                        {platformCoverage().multiPlatformCount === 1 ? '' : 's'} span multiple
                        platforms.
                      </div>
                    </Show>
                    <div class="flex flex-wrap gap-2">
                      <For each={platformCoverage().items.slice(0, 6)}>
                        {(item) => (
                          <div class="inline-flex items-center gap-2 rounded-md border border-border-subtle bg-surface px-2.5 py-1.5 text-xs">
                            <span class={`rounded px-1.5 py-0.5 text-[10px] font-medium ${item.toneClass}`}>
                              {item.label}
                            </span>
                            <span class="tabular-nums text-base-content">{item.count}</span>
                            <span class="text-muted">{item.percent}%</span>
                          </div>
                        )}
                      </For>
                    </div>
                  </div>
                </div>
              </section>

              <section class="rounded-xl border border-border bg-surface p-4">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                      Recent History
                    </div>
                    <div class="mt-4 grid grid-cols-3 gap-3 text-sm">
                      <div class="rounded-lg border border-border-subtle bg-surface p-3">
                        <div class="text-[10px] uppercase tracking-wide text-muted">
                          Recovery Points
                        </div>
                        <div class="mt-1 text-2xl font-semibold text-base-content">
                          {activity().totalEvents}
                        </div>
                      </div>
                      <div class="rounded-lg border border-border-subtle bg-surface p-3">
                        <div class="text-[10px] uppercase tracking-wide text-muted">Avg / Day</div>
                        <div class="mt-1 text-2xl font-semibold text-base-content">
                          {activity().averagePerDay.toFixed(1)}
                        </div>
                      </div>
                      <div class="rounded-lg border border-border-subtle bg-surface p-3">
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
                  <div class="mt-4 flex h-24 items-end gap-1 overflow-hidden rounded-lg border border-border-subtle bg-surface px-2 py-2">
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
          </div>
        </div>
      </Card>
    </Show>
  );
};
