import { createSignal, For, Show } from 'solid-js';
import type { Accessor, Component } from 'solid-js';

import { TooltipPortal } from '@/components/shared/TooltipPortal';
import {
  SUMMARY_CHART_PLOT_AREA_CLASS,
  SUMMARY_CHART_SLOT_COMPACT_CLASS,
} from '@/components/shared/summaryChartLayout';
import {
  getRecoveryCompactAxisLabel,
  getRecoveryPrettyDateLabel,
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
  getRecoveryTimelineColumnButtonClass,
  getRecoveryTimelineEmptyMarkerClass,
} from '@/utils/recoveryTimelinePresentation';

import {
  BACKUP_ACTIVITY_RANGE_DAYS,
  getBackupActivityAxisLabel,
  getBackupActivityColumnAriaLabel,
  getBackupActivityDayFilterStateLabel,
  getBackupActivityPointTotalLabel,
  getBackupActivitySegmentPresentation,
  getBackupActivityTooltipRows,
  type BackupActivityMetricMode,
  type BackupActivityNoun,
  type BackupActivityPoint,
  type BackupActivityRangeDays,
  type BackupActivitySegmentKind,
  type BackupActivityTimeline,
} from './proxmoxBackupActivityPresentation';

interface TooltipState {
  dateLabel: string;
  point: BackupActivityPoint;
  x: number;
  y: number;
}

export interface BackupActivityChartMetricToggle {
  mode: Accessor<BackupActivityMetricMode>;
  onChange: (mode: BackupActivityMetricMode) => void;
}

interface BackupActivityChartProps {
  title: string;
  noun: BackupActivityNoun;
  segmentKinds: readonly BackupActivitySegmentKind[];
  range: Accessor<BackupActivityRangeDays>;
  onRangeChange: (days: BackupActivityRangeDays) => void;
  timeline: Accessor<BackupActivityTimeline>;
  selectedDateKey: Accessor<string | null>;
  onToggleDay: (key: string) => void;
  metricMode?: Accessor<BackupActivityMetricMode>;
  metricToggle?: BackupActivityChartMetricToggle;
}

export const BackupActivityChart: Component<BackupActivityChartProps> = (props) => {
  const [tooltip, setTooltip] = createSignal<TooltipState | null>(null);

  const points = () => props.timeline().points;
  const axisMax = () => props.timeline().axisMax;
  const hasSelection = () => props.selectedDateKey() !== null;
  const metricMode = (): BackupActivityMetricMode =>
    props.metricMode?.() ?? props.metricToggle?.mode() ?? 'count';

  const axisTicks = () =>
    getRecoveryTimelineAxisTicks(points().length, false, props.timeline().labelEvery);

  const chartMinWidthStyle = () => {
    const minWidth = getRecoveryTimelineChartMinWidthPx(false, props.range(), points().length);
    return minWidth > 0 ? `${minWidth}px` : '100%';
  };

  const chartGapStyle = () => `${getRecoveryTimelineChartGapPx(props.range())}px`;

  const showTooltip = (target: HTMLElement, point: BackupActivityPoint, dateLabel: string) => {
    const rect = target.getBoundingClientRect();
    setTooltip({
      dateLabel,
      point,
      x: rect.left + rect.width / 2,
      y: rect.top,
    });
  };
  const hideTooltip = () => setTooltip(null);

  return (
    <div class="rounded-lg border border-border-subtle bg-surface-alt/25 px-2 py-2">
      <div class="mb-2 flex flex-col gap-1.5 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted">
          <div class="font-semibold uppercase tracking-[0.18em] text-muted">{props.title}</div>
          <For each={props.segmentKinds}>
            {(kind) => {
              const presentation = getBackupActivitySegmentPresentation(kind);
              return (
                <span class={RECOVERY_TIMELINE_LEGEND_ITEM_CLASS}>
                  <span class={`h-2.5 w-2.5 rounded ${presentation.swatchClassName}`} />
                  {presentation.label}
                </span>
              );
            }}
          </For>
        </div>
        <div class="flex flex-wrap items-center gap-2 text-[11px]">
          <Show when={props.metricToggle}>
            <div
              role="group"
              aria-label="Activity metric"
              class="inline-flex shrink-0 rounded border border-border bg-surface p-0.5"
            >
              <For each={['count', 'volume'] as const}>
                {(mode) => {
                  const selected = () => props.metricToggle?.mode() === mode;
                  return (
                    <button
                      type="button"
                      class={`rounded px-2 py-1 font-medium transition-colors ${
                        selected()
                          ? 'bg-surface-hover text-base-content'
                          : 'text-muted hover:bg-surface-hover hover:text-base-content'
                      }`}
                      aria-pressed={selected() ? 'true' : 'false'}
                      onClick={() => props.metricToggle?.onChange(mode)}
                    >
                      {mode === 'count' ? 'Count' : 'Volume'}
                    </button>
                  );
                }}
              </For>
            </div>
          </Show>
          <div
            role="group"
            aria-label="Activity range"
            class="inline-flex shrink-0 rounded border border-border bg-surface p-0.5"
          >
            <For each={BACKUP_ACTIVITY_RANGE_DAYS}>
              {(days) => {
                const selected = () => props.range() === days;
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
                    {days === 365 ? '1y' : `${days}d`}
                  </button>
                );
              }}
            </For>
          </div>
        </div>
      </div>

      <Show
        when={points().length > 0}
        fallback={
          <div class="flex h-24 items-center justify-center text-xs text-muted">
            No activity in the selected window.
          </div>
        }
      >
        <div class="relative">
          <div class="grid grid-cols-[auto_minmax(0,1fr)] gap-2">
            <div
              class={`flex ${SUMMARY_CHART_PLOT_AREA_CLASS} flex-col justify-between text-[10px] text-muted tabular-nums`}
            >
              <For each={[0, 1, 2]}>
                {(step) => {
                  const value = () => (axisMax() * (2 - step)) / 2;
                  return <span>{getBackupActivityAxisLabel(value(), metricMode())}</span>;
                }}
              </For>
            </div>

            <div class="overflow-x-auto overscroll-x-contain pb-1">
              <div
                class={`relative ${SUMMARY_CHART_SLOT_COMPACT_CLASS}`}
                style={{ 'min-width': chartMinWidthStyle() }}
              >
                <div
                  class="absolute inset-x-0 bottom-4 top-0 flex items-stretch"
                  style={{ gap: chartGapStyle() }}
                >
                  <For each={points()}>
                    {(point) => {
                      const total = () => point.total;
                      const heightPct = () =>
                        axisMax() > 0 ? Math.min(100, (total() / axisMax()) * 100) : 0;
                      const isSelected = () => props.selectedDateKey() === point.key;
                      const dateLabel = getRecoveryPrettyDateLabel(point.key);

                      return (
                        <div class="min-w-[3px] flex-1 self-stretch">
                          <button
                            type="button"
                            class={`h-full w-full ${getRecoveryTimelineColumnButtonClass(
                              isSelected(),
                              hasSelection(),
                            )}`}
                            aria-label={getBackupActivityColumnAriaLabel(
                              dateLabel,
                              total(),
                              isSelected(),
                              props.noun,
                              metricMode(),
                            )}
                            aria-pressed={isSelected() ? 'true' : 'false'}
                            onClick={() => props.onToggleDay(point.key)}
                            onMouseEnter={(event) =>
                              showTooltip(event.currentTarget, point, dateLabel)
                            }
                            onMouseLeave={hideTooltip}
                            onFocus={(event) => showTooltip(event.currentTarget, point, dateLabel)}
                            onBlur={hideTooltip}
                          >
                            <div class="relative h-full w-full overflow-hidden rounded-sm">
                              <Show
                                when={total() > 0}
                                fallback={
                                  <div
                                    class={getRecoveryTimelineEmptyMarkerClass(
                                      isSelected(),
                                      hasSelection(),
                                    )}
                                  />
                                }
                              >
                                <div
                                  class={getRecoveryTimelineBarMarkerClass(
                                    isSelected(),
                                    hasSelection(),
                                  )}
                                  style={{ height: `${heightPct()}%` }}
                                >
                                  <For each={props.segmentKinds}>
                                    {(kind) => {
                                      const count = () => point.counts[kind] ?? 0;
                                      const segmentPct = () =>
                                        total() > 0 ? (count() / total()) * 100 : 0;
                                      return (
                                        <Show when={segmentPct() > 0}>
                                          <div
                                            class={`w-full ${getBackupActivitySegmentPresentation(kind).segmentClassName}`}
                                            style={{ height: `${segmentPct()}%` }}
                                          />
                                        </Show>
                                      );
                                    }}
                                  </For>
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
                  <For each={axisTicks()}>
                    {(tick) => {
                      const point = points()[tick.index];
                      const isSelected = () =>
                        point ? props.selectedDateKey() === point.key : false;
                      const alignmentClass =
                        tick.align === 'start'
                          ? 'left-0 text-left'
                          : tick.align === 'end'
                            ? 'left-full -translate-x-full text-right'
                            : '-translate-x-1/2 text-center';
                      return (
                        <Show when={point}>
                          <span
                            class={`absolute bottom-0 whitespace-nowrap text-[9px] ${getRecoveryTimelineAxisLabelClass(isSelected())} ${alignmentClass}`}
                            style={{ left: `${tick.positionPct}%` }}
                          >
                            {getRecoveryCompactAxisLabel(point!.key, props.range())}
                          </span>
                        </Show>
                      );
                    }}
                  </For>
                </div>
              </div>
            </div>
          </div>

          <TooltipPortal
            when={tooltip() !== null}
            x={tooltip()?.x ?? 0}
            y={tooltip()?.y ?? 0}
            align="center"
            direction="up"
            maxWidth={240}
          >
            <Show when={tooltip()}>
              {(t) => (
                <div class="min-w-[180px]">
                  <div class="flex items-start justify-between gap-3 border-b border-border pb-1">
                    <div class="min-w-0">
                      <div class="truncate text-[11px] font-semibold text-base-content">
                        {t().dateLabel}
                      </div>
                      <div class="mt-0.5 text-[10px] text-muted">
                        {getBackupActivityPointTotalLabel(
                          t().point.total,
                          props.noun,
                          metricMode(),
                        )}
                      </div>
                    </div>
                    <div class="shrink-0 rounded border border-border bg-surface-alt px-1.5 py-0.5 text-[9px] font-medium text-muted">
                      {getBackupActivityDayFilterStateLabel(
                        props.selectedDateKey() === t().point.key,
                        hasSelection(),
                      )}
                    </div>
                  </div>
                  <ul class="mt-1.5 space-y-1">
                    <For
                      each={getBackupActivityTooltipRows(
                        t().point,
                        props.segmentKinds,
                        metricMode(),
                      )}
                    >
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
  );
};

export default BackupActivityChart;
