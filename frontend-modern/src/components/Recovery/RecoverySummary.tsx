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
        class="overflow-hidden border-border bg-surface shadow-[0_10px_24px_rgba(2,6,23,0.1)]"
        data-testid="recovery-summary"
      >
        <div class="flex flex-col gap-4 p-4 sm:p-5">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-border-subtle/80 bg-surface pb-4 text-xs">
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

          <div class="grid items-start gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(22rem,0.85fr)]">
            <section class="rounded-xl border border-border-subtle bg-surface-alt/35 p-4 shadow-[inset_0_1px_0_rgba(148,163,184,0.04)]">
              <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <div class="text-xs font-semibold uppercase tracking-[0.18em] text-slate-400">
                    Recovery Posture
                  </div>
                  <div class="mt-1.5 max-w-xl text-sm leading-6 text-muted">
                    Recovery readiness across the current protected estate, with unhealthy
                    coverage surfaced before activity detail.
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

              <div class="mt-4 grid gap-3 sm:grid-cols-3">
                <div class="rounded-lg border border-border-subtle bg-surface px-3 py-3">
                  <div class="text-[11px] uppercase tracking-wide text-muted">Protected</div>
                  <div class="mt-1 text-3xl font-semibold tracking-tight text-base-content">{summary().total}</div>
                </div>
                <div class="rounded-lg border border-border-subtle bg-surface px-3 py-3">
                  <div class="text-[11px] uppercase tracking-wide text-muted">Healthy</div>
                  <div class="mt-1 text-3xl font-semibold tracking-tight text-emerald-400">{healthyCount()}</div>
                </div>
                <div class="rounded-lg border border-border-subtle bg-surface px-3 py-3">
                  <div class="text-[11px] uppercase tracking-wide text-muted">Attention</div>
                  <div class="mt-1 text-3xl font-semibold tracking-tight text-amber-300">{attentionCount()}</div>
                </div>
              </div>

              <div class="mt-4 h-3 overflow-hidden rounded-full bg-surface-alt/90">
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

              <div class="mt-4 flex flex-wrap gap-2">
                <For each={postureSegments()}>
                  {(segment) => (
                    <div class="inline-flex items-center gap-2 rounded-full border border-border-subtle bg-surface px-3 py-1.5 text-sm">
                      <div class="flex items-center gap-2">
                        <span class={`h-2 w-2 rounded-full ${segment.color}`} />
                        <span class="text-base-content">{segment.label}</span>
                      </div>
                      <span class={segment.textColor}>{segment.count}</span>
                    </div>
                  )}
                </For>
              </div>
              <div class="mt-4 grid gap-3 lg:grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)]">
                <div class="rounded-lg border border-border-subtle bg-surface p-3">
                  <div class="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted">
                    Freshness
                  </div>
                  <div class="mt-3 space-y-2.5">
                    <For each={freshnessBuckets()}>
                      {(bucket) => (
                        <div class="grid grid-cols-[52px_minmax(0,1fr)_28px] items-center gap-2 text-sm">
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
                  <div class="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted">
                    Attention Queue
                  </div>
                  <Show
                    when={attentionItems().length > 0}
                    fallback={<div class="mt-3 text-sm text-emerald-300">No active recovery risks.</div>}
                  >
                    <div class="mt-3 grid gap-2">
                      <For each={attentionItems().slice(0, 2)}>
                        {(item) => (
                          <div
                            class={`rounded-lg border px-3 py-3 ${getRecoveryAttentionChipClass(item.tone)}`}
                          >
                            <div class="flex items-center justify-between gap-2">
                              <div class="flex items-center gap-2">
                                <span
                                  class={`h-2.5 w-2.5 rounded-full ${getRecoveryAttentionDotClass(item.tone)}`}
                                />
                                <span class="text-[11px] font-semibold uppercase tracking-[0.14em]">
                                  {item.label}
                                </span>
                              </div>
                              <span class="text-base font-semibold tabular-nums">{item.count}</span>
                            </div>
                            <div class="mt-1.5 text-sm leading-6 text-current/80">{item.detail}</div>
                          </div>
                        )}
                      </For>
                    </div>
                  </Show>
                </div>
              </div>
            </section>

            <div class="grid gap-3">
              <section class="rounded-xl border border-border-subtle bg-surface-alt/35 p-4">
                <div class="text-xs font-semibold uppercase tracking-[0.18em] text-muted">
                  Protected Footprint
                </div>
                <div class="mt-3 grid gap-3 sm:grid-cols-2">
                  <div class="rounded-lg border border-border-subtle bg-surface p-3">
                    <div class="text-[11px] uppercase tracking-wide text-muted">Item Types</div>
                    <div class="mt-1 text-3xl font-semibold tracking-tight text-base-content">
                      {itemCoverage().itemTypeCount}
                    </div>
                  </div>
                  <div class="rounded-lg border border-border-subtle bg-surface p-3">
                    <div class="text-[11px] uppercase tracking-wide text-muted">Primary Item</div>
                    <div class="mt-1 text-xl font-semibold leading-7 text-base-content">
                      {itemCoverage().primaryItemLabel ?? 'n/a'}
                    </div>
                  </div>
                  <div class="rounded-lg border border-border-subtle bg-surface p-3">
                    <div class="text-[11px] uppercase tracking-wide text-muted">Platforms</div>
                    <div class="mt-1 text-3xl font-semibold tracking-tight text-base-content">
                      {platformCoverage().platformCount}
                    </div>
                  </div>
                  <div class="rounded-lg border border-border-subtle bg-surface p-3">
                    <div class="text-[11px] uppercase tracking-wide text-muted">Primary Platform</div>
                    <div class="mt-1 text-xl font-semibold leading-7 text-base-content">
                      {platformCoverage().primaryPlatformLabel ?? 'n/a'}
                    </div>
                  </div>
                </div>

                <div class="mt-3 space-y-3">
                  <Show when={itemCoverage().items.length > 0}>
                    <div>
                      <div class="mb-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
                        Item Types
                      </div>
                      <div class="flex flex-wrap gap-2">
                        <For each={itemCoverage().items.slice(0, 6)}>
                          {(item) => (
                            <div class="inline-flex items-center gap-2 rounded-md border border-border-subtle bg-surface px-2.5 py-1.5 text-sm">
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
                    <div class="mb-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
                      Platform Mix
                    </div>
                    <Show when={platformCoverage().multiPlatformCount > 0}>
                      <div class="mb-2 text-sm text-muted">
                        {platformCoverage().multiPlatformCount} protected item
                        {platformCoverage().multiPlatformCount === 1 ? '' : 's'} span multiple
                        platforms.
                      </div>
                    </Show>
                    <div class="flex flex-wrap gap-2">
                      <For each={platformCoverage().items.slice(0, 6)}>
                        {(item) => (
                          <div class="inline-flex items-center gap-2 rounded-md border border-border-subtle bg-surface px-2.5 py-1.5 text-sm">
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

              <section class="rounded-xl border border-border-subtle bg-surface-alt/35 p-4">
                <div class="text-xs font-semibold uppercase tracking-[0.18em] text-muted">
                  Recent History
                </div>
                <div class="mt-3 grid gap-3">
                  <div class="rounded-lg border border-border-subtle bg-surface p-3">
                    <div class="grid gap-3 sm:grid-cols-3">
                      <div>
                        <div class="text-[11px] uppercase tracking-wide text-muted">
                          Recovery Points
                        </div>
                        <div class="mt-1 text-2xl font-semibold text-base-content">
                          {activity().totalEvents}
                        </div>
                      </div>
                      <div>
                        <div class="text-[11px] uppercase tracking-wide text-muted">Avg / Day</div>
                        <div class="mt-1 text-2xl font-semibold text-base-content">
                          {activity().averagePerDay.toFixed(1)}
                        </div>
                      </div>
                      <div>
                        <div class="text-[11px] uppercase tracking-wide text-muted">Days Active</div>
                        <div class="mt-1 text-2xl font-semibold text-base-content">
                          {activity().activeDays}
                        </div>
                      </div>
                    </div>
                  </div>
                  <Show
                    when={activity().hasData}
                    fallback={
                      <div class="rounded-lg border border-dashed border-border-subtle bg-surface p-3 text-sm text-muted">
                        {props.seriesFailed?.() ? 'Trend data unavailable' : 'No recovery activity yet'}
                      </div>
                    }
                  >
                    <div class="rounded-lg border border-border-subtle bg-surface p-3">
                      <div class="grid gap-3 sm:grid-cols-2">
                        <div>
                          <div class="text-[11px] uppercase tracking-wide text-muted">Peak Day</div>
                          <div class="mt-1 text-base font-semibold text-base-content">
                            {activity().busiestLabel ?? 'n/a'}
                          </div>
                          <div class="mt-1 text-sm text-muted">
                            {activity().busiestCount} recovery point
                            {activity().busiestCount === 1 ? '' : 's'}
                          </div>
                        </div>
                        <div>
                          <div class="text-[11px] uppercase tracking-wide text-muted">Latest Activity</div>
                          <div class="mt-1 text-base font-semibold text-base-content">
                            {activity().latestLabel ?? 'n/a'}
                          </div>
                          <div class="mt-1 text-sm text-muted">
                            {activity().latestCount} recovery point
                            {activity().latestCount === 1 ? '' : 's'}
                          </div>
                        </div>
                      </div>
                    </div>
                  </Show>
                </div>
              </section>
            </div>
          </div>
        </div>
      </Card>
    </Show>
  );
};
