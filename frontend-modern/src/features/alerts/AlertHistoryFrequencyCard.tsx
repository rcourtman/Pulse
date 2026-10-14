import { createMemo, For, Show } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { hideTooltip, showTooltip } from '@/components/shared/Tooltip';
import { getAlertBucketCountLabel } from '@/utils/alertOverviewPresentation';
import {
  getAlertFrequencyClearFilterButtonClass,
  getAlertFrequencySelectionPresentation,
} from '@/utils/alertFrequencyPresentation';
import {
  getAlertSeverityDotClass,
  getAlertSeverityFillClass,
} from '@/utils/alertSeverityPresentation';

import { MS_PER_HOUR, alertSeverityCountKey, type AlertSeverityCounts } from './alertHistoryModel';
import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertHistoryFrequencyCardProps {
  state: AlertHistoryState;
}

// Most severe first: the legend reads in this order and each bar stacks it
// from the baseline up, so a period's critical alerts are never hidden.
const SEVERITY_ORDER = ['critical', 'warning', 'info'] as const;
const SEVERITY_NOUNS: Record<(typeof SEVERITY_ORDER)[number], [string, string]> = {
  critical: ['critical', 'critical'],
  warning: ['warning', 'warnings'],
  info: ['info', 'info'],
};

const formatSeverityCount = (severity: (typeof SEVERITY_ORDER)[number], count: number) =>
  `${count} ${SEVERITY_NOUNS[severity][count === 1 ? 0 : 1]}`;

const formatSeverityBreakdown = (counts: AlertSeverityCounts | undefined) =>
  counts
    ? SEVERITY_ORDER.filter((severity) => counts[severity] > 0)
        .map((severity) => formatSeverityCount(severity, counts[severity]))
        .join(', ')
    : '';

// The busiest period fills the chart and the rest scale by the square root of
// their share: a burst stands well clear of a quiet period (24 alerts against
// one is five times taller) while ordinary periods stay tall enough to read
// their severity mix. Each bar's tooltip carries the exact counts.
const BAR_MAX_PX = 40;
const BAR_MIN_PX = 4;

export function AlertHistoryFrequencyCard(props: AlertHistoryFrequencyCardProps) {
  const alertFrequencySelectionPresentation = createMemo(() =>
    getAlertFrequencySelectionPresentation(),
  );
  const severityTotals = createMemo(() => {
    const totals: AlertSeverityCounts = { critical: 0, warning: 0, info: 0 };
    for (const alert of props.state.alertData()) totals[alertSeverityCountKey(alert.severity)]++;
    return totals;
  });

  return (
    <Card padding="md">
      <div class="mb-3 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
        <SectionHeader
          title="Alert frequency"
          description={
            <span class="text-xs text-muted">{props.state.alertData().length} alerts</span>
          }
          size="sm"
          class="flex-1"
        />
        <div class="flex flex-col items-start gap-2 sm:items-end">
          <Show when={props.state.selectedBucketDetails()}>
            {(selection) => (
              <div class={alertFrequencySelectionPresentation().containerClass}>
                <span class={alertFrequencySelectionPresentation().labelClass}>Filtered Range</span>
                <span class="font-mono text-[11px]">{selection().rangeLabel}</span>
              </div>
            )}
          </Show>
          <div class="flex flex-col items-start gap-1 text-xs text-muted sm:items-end">
            <div>
              <span class="font-medium text-muted">Bar size:</span>{' '}
              {props.state.bucketDurationLabel()}
            </div>
            <Show when={props.state.rangeSummary()}>
              {(summary) => (
                <div class="flex items-center gap-1 whitespace-nowrap">
                  <span class="font-medium text-muted">Range:</span>
                  <span>{summary().startLabel}</span>
                  <span class="text-muted">→</span>
                  <span>{summary().endLabel}</span>
                </div>
              )}
            </Show>
            <div class="flex flex-wrap items-center justify-end gap-2">
              <Show when={props.state.selectedBarIndex() !== null}>
                <button
                  type="button"
                  onClick={() => props.state.setSelectedBarIndex(null)}
                  class={getAlertFrequencyClearFilterButtonClass()}
                >
                  Clear filter
                </button>
              </Show>
              <div class="flex items-center gap-2 text-xs text-muted" data-alert-frequency-legend>
                <For
                  each={SEVERITY_ORDER.filter(
                    (severity) => severity !== 'info' || severityTotals().info > 0,
                  )}
                >
                  {(severity) => (
                    <span class="flex items-center gap-1">
                      <span aria-hidden="true" class={getAlertSeverityDotClass(severity)}></span>
                      {formatSeverityCount(severity, severityTotals()[severity])}
                    </span>
                  )}
                </For>
              </div>
            </div>
          </div>
        </div>
      </div>

      {(() => {
        const trends = props.state.alertTrends();
        return (
          <div
            data-alert-frequency-scroll
            class="overflow-x-auto pb-1"
            role="group"
            aria-label="Filter alert history by time period"
          >
            {/*
              Keep each period at least 24 CSS pixels wide. On compact cards the
              chart and its axis scroll as one surface instead of compressing
              dozens of pointer targets below the WCAG 2.2 minimum target size.
            */}
            <div class="w-max min-w-full">
              <div class="rounded-sm bg-surface-alt p-1">
                <div class="flex h-12 items-end gap-1">
                  {trends.buckets.map((value, index) => {
                    const pixelHeight =
                      value > 0
                        ? Math.max(
                            BAR_MIN_PX,
                            Math.round(Math.sqrt(value / trends.max) * BAR_MAX_PX),
                          )
                        : 0;
                    const counts = trends.severityBuckets?.[index];
                    const breakdown = formatSeverityBreakdown(counts);
                    const isSelected = () => props.state.selectedBarIndex() === index;
                    const bucketStart = trends.bucketTimes[index];
                    const bucketEnd = bucketStart + trends.bucketSize * MS_PER_HOUR;
                    const bucketRangeLabel = props.state.formatBucketRange(bucketStart, bucketEnd);
                    const bucketDurationText =
                      trends.bucketSize % 24 === 0
                        ? `${trends.bucketSize / 24} day${trends.bucketSize / 24 === 1 ? '' : 's'}`
                        : `${trends.bucketSize} hour${trends.bucketSize === 1 ? '' : 's'}`;
                    const countLabel = getAlertBucketCountLabel(value);
                    const tooltipContent = [
                      breakdown ? `${countLabel}: ${breakdown}` : countLabel,
                      `${bucketDurationText} period`,
                      bucketRangeLabel,
                    ].join('\n');

                    return (
                      <button
                        type="button"
                        data-alert-frequency-bucket={index}
                        class="relative flex h-12 min-w-6 flex-1 cursor-pointer items-end rounded-xs focus:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-inset"
                        aria-pressed={isSelected()}
                        aria-label={`${breakdown ? `${countLabel} (${breakdown})` : countLabel} between ${bucketRangeLabel}`}
                        onClick={() =>
                          props.state.setSelectedBarIndex(
                            index === props.state.selectedBarIndex() ? null : index,
                          )
                        }
                      >
                        <span class="absolute bottom-0 h-1 w-full rounded-full bg-slate-300 opacity-30"></span>
                        <span
                          data-alert-frequency-bar
                          class="relative flex w-full flex-col-reverse overflow-hidden rounded-xs"
                          classList={{ 'ring-2 ring-blue-500/60': isSelected() }}
                          style={{ height: `${pixelHeight}px` }}
                          title={bucketRangeLabel}
                          onMouseEnter={(event) => {
                            if (value <= 0) {
                              hideTooltip();
                              return;
                            }
                            const rect = event.currentTarget.getBoundingClientRect();
                            showTooltip(tooltipContent, rect.left + rect.width / 2, rect.top, {
                              align: 'center',
                              direction: 'up',
                            });
                          }}
                          onMouseLeave={() => hideTooltip()}
                        >
                          <For
                            each={SEVERITY_ORDER.filter(
                              (severity) => (counts?.[severity] ?? 0) > 0,
                            )}
                          >
                            {(severity) => (
                              <span
                                data-alert-frequency-segment={severity}
                                class={`w-full shrink-0 ${getAlertSeverityFillClass(severity)}`}
                                style={{ height: `${((counts?.[severity] ?? 0) / value) * 100}%` }}
                              />
                            )}
                          </For>
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>

              <Show when={props.state.axisTicks().length > 0}>
                <div class="relative mt-3 h-10">
                  <div class="absolute inset-x-0 top-0 h-px bg-surface-hover"></div>
                  <For each={props.state.axisTicks()}>
                    {(tick, index) => {
                      const lastIndex = props.state.axisTicks().length - 1;
                      const mobileMidpoint = Math.floor(lastIndex / 2);
                      const visibleOnMobile =
                        index() === 0 || index() === mobileMidpoint || index() === lastIndex;

                      return (
                        <div
                          data-testid="alert-frequency-axis-tick"
                          class="pointer-events-none absolute top-0 h-full flex-col items-center sm:flex"
                          classList={{
                            hidden: !visibleOnMobile,
                            flex: visibleOnMobile,
                          }}
                          style={{ left: `${tick.position * 100}%` }}
                        >
                          <div class="h-3 w-px bg-slate-300"></div>
                          <div
                            data-testid="alert-frequency-axis-label"
                            class="mt-1 whitespace-nowrap text-[10px] text-muted transform"
                            classList={{
                              '-translate-x-1/2': tick.align === 'center',
                              '-translate-x-full': tick.align === 'end',
                            }}
                          >
                            {tick.label}
                          </div>
                        </div>
                      );
                    }}
                  </For>
                </div>
              </Show>
            </div>
          </div>
        );
      })()}
    </Card>
  );
}
