import { createMemo, createSignal, For, Show } from 'solid-js';
import type { Accessor, Component, JSX } from 'solid-js';

import { Card } from '@/components/shared/Card';
import {
  SUMMARY_CHART_PLOT_AREA_CLASS,
  SUMMARY_CHART_SLOT_CLASS,
} from '@/components/shared/summaryChartLayout';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import {
  getRecoveryActivityEmptyState,
  getRecoveryActivityLoadingState,
} from '@/utils/recoveryEmptyStatePresentation';
import { getRecoveryArtifactModePresentation } from '@/utils/recoveryArtifactModePresentation';
import {
  getRecoveryPrettyDateLabel,
  getRecoveryCompactAxisLabel,
} from '@/utils/recoveryDatePresentation';
import {
  getRecoveryTimelineAxisLabelClass,
  getRecoveryTimelineAxisTicks,
  getRecoveryTimelineChartGapPx,
  getRecoveryTimelineChartMinWidthPx,
  RECOVERY_TIMELINE_LEGEND_ITEM_CLASS,
} from '@/utils/recoveryTimelineChartPresentation';
import {
  getRecoveryTimelineBarMarkerClass,
  getRecoveryTimelineDayFilterStateLabel,
  getRecoveryTimelineColumnAriaLabel,
  getRecoveryTimelineColumnButtonClass,
  getRecoveryTimelineEmptyMarkerClass,
  getRecoveryTimelinePointTotalLabel,
  getRecoveryTimelineTooltipRows,
} from '@/utils/recoveryTimelinePresentation';

interface RecoveryRollupSummary {
  total: number;
  stale: number;
  neverSucceeded: number;
}

interface ActivitySummary {
  totalPoints: number;
  activeDays: number;
  averagePerDay: number;
}

interface TimelinePoint {
  key: string;
  label: string;
  total: number;
  snapshot: number;
  local: number;
  remote: number;
}

interface TimelineModel {
  points: TimelinePoint[];
  axisMax: number;
  labelEvery: number;
}

interface ActivityTooltipState {
  dateLabel: string;
  point: TimelinePoint;
  x: number;
  y: number;
}

const RECOVERY_ACTIVITY_RANGE_DAYS = [7, 30, 90, 365] as const;

interface RecoveryActivitySectionProps {
  activitySummary: Accessor<ActivitySummary>;
  chartRangeDays: Accessor<7 | 30 | 90 | 365>;
  isMobile: boolean;
  loading: Accessor<boolean>;
  overallRollupsSummary: Accessor<RecoveryRollupSummary>;
  onRangeChange: (days: (typeof RECOVERY_ACTIVITY_RANGE_DAYS)[number]) => void;
  dayFilterKey: Accessor<string | null>;
  toggleDayFilter: (key: string) => void;
  timeline: Accessor<TimelineModel>;
}

function RecoveryActivitySectionContent(props: RecoveryActivitySectionProps): JSX.Element {
  const [activityTooltip, setActivityTooltip] = createSignal<ActivityTooltipState | null>(null);
  const timelineAxisTicks = () =>
    getRecoveryTimelineAxisTicks(
      props.timeline().points.length,
      props.isMobile,
      props.timeline().labelEvery,
    );
  const chartMinWidthStyle = () => {
    const minWidth = getRecoveryTimelineChartMinWidthPx(
      props.isMobile,
      props.chartRangeDays(),
      props.timeline().points.length,
    );
    return minWidth > 0 ? `${minWidth}px` : '100%';
  };
  const chartGapStyle = () => `${getRecoveryTimelineChartGapPx(props.chartRangeDays())}px`;
  const filteredTimelinePoint = createMemo(() => {
    const selected = props.dayFilterKey();
    if (!selected) return null;
    return props.timeline().points.find((point) => point.key === selected) ?? null;
  });
  const timelineHasDayFilter = () => filteredTimelinePoint() !== null;
  const showActivityTooltip = (target: HTMLElement, point: TimelinePoint, dateLabel: string) => {
    const rect = target.getBoundingClientRect();
    setActivityTooltip({
      dateLabel,
      point,
      x: rect.left + rect.width / 2,
      y: rect.top,
    });
  };
  const hideActivityTooltip = () => setActivityTooltip(null);
  const formatActivityAverage = (value: number): string => {
    if (!Number.isFinite(value) || value <= 0) return '0/day';
    if (value >= 10) return `${Math.round(value)}/day`;
    return `${Number(value.toFixed(1))}/day`;
  };
  return (
    <>
      <div class="mb-2 flex flex-col gap-1.5">
        <div class="flex flex-col gap-1.5 lg:flex-row lg:items-center lg:justify-between">
          <div class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted">
            <div class="font-semibold uppercase tracking-[0.18em] text-muted">
              Recovery Activity
            </div>
            <span class="font-medium text-base-content">
              {props.activitySummary().totalPoints} recovery points
            </span>
            <span>{props.activitySummary().activeDays} active days</span>
            <Show when={props.overallRollupsSummary().stale > 0}>
              <span class="font-medium text-amber-600 dark:text-amber-400">
                {props.overallRollupsSummary().stale} stale
              </span>
            </Show>
          </div>
          <div class="flex flex-wrap items-center gap-2 text-[11px]">
            <div
              role="group"
              aria-label="Recovery activity range"
              class="inline-flex shrink-0 rounded border border-border bg-surface p-0.5"
            >
              <For each={RECOVERY_ACTIVITY_RANGE_DAYS}>
                {(days) => {
                  const selected = () => props.chartRangeDays() === days;
                  return (
                    <button
                      type="button"
                      class={`rounded px-2 py-1 font-medium transition-colors ${
                        selected()
                          ? 'bg-surface-hover text-base-content'
                          : 'text-muted hover:bg-surface-hover hover:text-base-content'
                      }`}
                      aria-pressed={selected() ? 'true' : 'false'}
                      onClick={() => props.onRangeChange(days)}
                    >
                      {days}d
                    </button>
                  );
                }}
              </For>
            </div>
            <span class="rounded-md border border-border bg-surface-alt px-2 py-1 text-muted">
              Avg {formatActivityAverage(props.activitySummary().averagePerDay)}
            </span>
          </div>
        </div>
      </div>

      <div class="rounded-lg border border-border-subtle bg-surface-alt/25 px-2 py-1.5">
        <div class="mb-1 flex flex-wrap items-center justify-end gap-1.5 text-[9px] text-muted">
          <div class="flex flex-wrap items-center gap-1.5 text-[9px] text-muted">
            <div class={RECOVERY_TIMELINE_LEGEND_ITEM_CLASS}>
              <span
                class={`h-2.5 w-2.5 rounded ${getRecoveryArtifactModePresentation('snapshot').segmentClassName}`}
              />
              {getRecoveryArtifactModePresentation('snapshot').aggregateLabel}
            </div>
            <div class={RECOVERY_TIMELINE_LEGEND_ITEM_CLASS}>
              <span
                class={`h-2.5 w-2.5 rounded ${getRecoveryArtifactModePresentation('local').segmentClassName}`}
              />
              {getRecoveryArtifactModePresentation('local').aggregateLabel}
            </div>
            <div class={RECOVERY_TIMELINE_LEGEND_ITEM_CLASS}>
              <span
                class={`h-2.5 w-2.5 rounded ${getRecoveryArtifactModePresentation('remote').segmentClassName}`}
              />
              {getRecoveryArtifactModePresentation('remote').aggregateLabel}
            </div>
          </div>
        </div>

        <Show
          when={!props.loading() && props.timeline().points.length > 0}
          fallback={
            <div class="flex h-32 items-center justify-center text-sm text-muted">
              {props.loading()
                ? getRecoveryActivityLoadingState().text
                : getRecoveryActivityEmptyState().text}
            </div>
          }
        >
          <div class="relative">
            <div class="grid grid-cols-[auto_minmax(0,1fr)] gap-2.5">
              <div
                class={`flex ${SUMMARY_CHART_PLOT_AREA_CLASS} flex-col justify-between text-[10px] text-muted`}
              >
                <For each={[0, 1, 2]}>
                  {(step) => {
                    const value = Math.round((props.timeline().axisMax * (2 - step)) / 2);
                    return <span>{value}</span>;
                  }}
                </For>
              </div>

              <div
                data-testid="recovery-activity-chart-scroll"
                class="overflow-x-auto overscroll-x-contain pb-1"
              >
                <div
                  class={`relative ${SUMMARY_CHART_SLOT_CLASS}`}
                  style={{ 'min-width': chartMinWidthStyle() }}
                >
                  <div
                    data-testid="recovery-activity-bars"
                    class="absolute inset-x-0 bottom-4 top-0 flex items-stretch"
                    style={{ gap: chartGapStyle() }}
                  >
                    <For each={props.timeline().points}>
                      {(point) => {
                        const total = point.total;
                        const heightPct =
                          props.timeline().axisMax > 0
                            ? (total / props.timeline().axisMax) * 100
                            : 0;
                        const columnHeight = Math.max(0, Math.min(100, heightPct));
                        const snapshotHeight = total > 0 ? (point.snapshot / total) * 100 : 0;
                        const localHeight = total > 0 ? (point.local / total) * 100 : 0;
                        const remoteHeight = total > 0 ? (point.remote / total) * 100 : 0;
                        const isSelected = () => props.dayFilterKey() === point.key;
                        const dateLabel = getRecoveryPrettyDateLabel(point.key);

                        return (
                          <div class="min-w-[3px] flex-1 self-stretch">
                            <button
                              type="button"
                              data-recovery-activity-bar={point.key}
                              class={`h-full w-full ${getRecoveryTimelineColumnButtonClass(
                                isSelected(),
                                timelineHasDayFilter(),
                              )}`}
                              aria-label={getRecoveryTimelineColumnAriaLabel(
                                dateLabel,
                                total,
                                isSelected(),
                              )}
                              aria-pressed={isSelected() ? 'true' : 'false'}
                              onClick={() => props.toggleDayFilter(point.key)}
                              onMouseEnter={(event) => {
                                showActivityTooltip(event.currentTarget, point, dateLabel);
                              }}
                              onMouseLeave={hideActivityTooltip}
                              onFocus={(event) => {
                                showActivityTooltip(event.currentTarget, point, dateLabel);
                              }}
                              onBlur={hideActivityTooltip}
                            >
                              <div class="relative h-full w-full overflow-hidden rounded-sm">
                                <Show
                                  when={total > 0}
                                  fallback={
                                    <div
                                      data-testid="recovery-activity-empty-marker"
                                      class={getRecoveryTimelineEmptyMarkerClass(
                                        isSelected(),
                                        timelineHasDayFilter(),
                                      )}
                                    />
                                  }
                                >
                                  <div
                                    data-testid="recovery-activity-bar-stack"
                                    class={getRecoveryTimelineBarMarkerClass(
                                      isSelected(),
                                      timelineHasDayFilter(),
                                    )}
                                    style={{ height: `${columnHeight}%` }}
                                  >
                                    <Show when={remoteHeight > 0}>
                                      <div
                                        class={`w-full ${getRecoveryArtifactModePresentation('remote').segmentClassName}`}
                                        style={{ height: `${remoteHeight}%` }}
                                      />
                                    </Show>
                                    <Show when={localHeight > 0}>
                                      <div
                                        class={`w-full ${getRecoveryArtifactModePresentation('local').segmentClassName}`}
                                        style={{ height: `${localHeight}%` }}
                                      />
                                    </Show>
                                    <Show when={snapshotHeight > 0}>
                                      <div
                                        class={`w-full ${getRecoveryArtifactModePresentation('snapshot').segmentClassName}`}
                                        style={{ height: `${snapshotHeight}%` }}
                                      />
                                    </Show>
                                  </div>
                                </Show>
                              </div>
                            </button>
                          </div>
                        );
                      }}
                    </For>
                  </div>

                  <div class="pointer-events-none absolute inset-x-0 bottom-0 h-4">
                    <For each={timelineAxisTicks()}>
                      {(tick) => {
                        const point = props.timeline().points[tick.index];
                        const isSelected = () => props.dayFilterKey() === point.key;
                        const alignmentClass =
                          tick.align === 'start'
                            ? 'left-0 text-left'
                            : tick.align === 'end'
                              ? 'left-full -translate-x-full text-right'
                              : '-translate-x-1/2 text-center';

                        return (
                          <span
                            class={`absolute bottom-0 whitespace-nowrap text-[9px] ${
                              isSelected()
                                ? getRecoveryTimelineAxisLabelClass(true)
                                : getRecoveryTimelineAxisLabelClass(false)
                            } ${alignmentClass}`}
                            style={{ left: `${tick.positionPct}%` }}
                          >
                            {getRecoveryCompactAxisLabel(point.key, props.chartRangeDays())}
                          </span>
                        );
                      }}
                    </For>
                  </div>
                </div>
              </div>
            </div>

            <TooltipPortal
              when={activityTooltip() !== null}
              x={activityTooltip()?.x ?? 0}
              y={activityTooltip()?.y ?? 0}
              align="center"
              direction="up"
              maxWidth={260}
            >
              <Show when={activityTooltip()}>
                {(tooltip) => (
                  <div data-testid="recovery-activity-tooltip" class="min-w-[210px]">
                    <div class="flex items-start justify-between gap-3 border-b border-border pb-1">
                      <div class="min-w-0">
                        <div class="truncate text-[11px] font-semibold text-base-content">
                          {tooltip().dateLabel}
                        </div>
                        <div class="mt-0.5 text-[10px] text-muted">
                          {getRecoveryTimelinePointTotalLabel(tooltip().point.total)}
                        </div>
                      </div>
                      <div class="shrink-0 rounded border border-border bg-surface-alt px-1.5 py-0.5 text-[9px] font-medium text-muted">
                        {getRecoveryTimelineDayFilterStateLabel(
                          props.dayFilterKey() === tooltip().point.key,
                          timelineHasDayFilter(),
                        )}
                      </div>
                    </div>

                    <ul class="mt-1.5 space-y-1">
                      <For each={getRecoveryTimelineTooltipRows(tooltip().point)}>
                        {(row) => (
                          <li
                            class={`flex items-center justify-between gap-4 ${
                              row.muted ? 'text-muted/70' : 'text-base-content'
                            }`}
                          >
                            <span class="flex min-w-0 items-center gap-1.5">
                              <span class={`h-2 w-2 shrink-0 rounded-sm ${row.segmentClassName}`} />
                              <span class="truncate">{row.label}</span>
                            </span>
                            <span class="shrink-0 font-mono tabular-nums">{row.value}</span>
                          </li>
                        )}
                      </For>
                    </ul>
                  </div>
                )}
              </Show>
            </TooltipPortal>
          </div>
        </Show>
      </div>
    </>
  );
}

export const RecoveryActivitySection: Component<RecoveryActivitySectionProps> = (props) => (
  <Card padding="sm" class="h-full border-border-subtle bg-surface">
    <RecoveryActivitySectionContent {...props} />
  </Card>
);
