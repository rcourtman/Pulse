import { createMemo, For, Show } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { hideTooltip, showTooltip } from '@/components/shared/Tooltip';
import { getAlertBucketCountLabel } from '@/utils/alertOverviewPresentation';
import {
  getAlertFrequencyClearFilterButtonClass,
  getAlertFrequencySelectionPresentation,
} from '@/utils/alertFrequencyPresentation';
import { getAlertSeverityDotClass } from '@/utils/alertSeverityPresentation';

import { MS_PER_HOUR } from './alertHistoryModel';
import type { AlertHistoryState } from './useAlertHistoryState';

interface AlertHistoryFrequencyCardProps {
  state: AlertHistoryState;
}

export function AlertHistoryFrequencyCard(props: AlertHistoryFrequencyCardProps) {
  const alertFrequencySelectionPresentation = createMemo(() =>
    getAlertFrequencySelectionPresentation(),
  );

  return (
    <Card padding="md">
      <div class="mb-3 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
        <SectionHeader
          title="Alert frequency"
          description={<span class="text-xs text-muted">{props.state.alertData().length} alerts</span>}
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
              <div class="flex items-center gap-2 text-xs text-muted">
                <span class="flex items-center gap-1">
                  <div class={getAlertSeverityDotClass('warning')}></div>
                  {
                    props.state
                      .alertData()
                      .filter((alert) => alert.severity === 'warning').length
                  }{' '}
                  warnings
                </span>
                <span class="flex items-center gap-1">
                  <div class={getAlertSeverityDotClass('critical')}></div>
                  {
                    props.state
                      .alertData()
                      .filter((alert) => alert.severity === 'critical').length
                  }{' '}
                  critical
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="mb-1 text-[10px] text-muted">
        Showing {props.state.alertTrends().buckets.length} time periods (
        {props.state.bucketDurationLabel()} each) · Total: {props.state.alertData().length} alerts
      </div>

      {(() => {
        const trends = props.state.alertTrends();
        return (
          <div class="rounded bg-surface-alt p-1">
            <div class="flex h-12 items-end gap-1">
              {trends.buckets.map((value, index) => {
                const scaledHeight =
                  value > 0 ? Math.min(100, Math.max(20, Math.log(value + 1) * 20)) : 0;
                const pixelHeight = value > 0 ? Math.max(8, (scaledHeight / 100) * 40) : 0;
                const isSelected = props.state.selectedBarIndex() === index;
                const bucketStart = trends.bucketTimes[index];
                const bucketEnd = bucketStart + trends.bucketSize * MS_PER_HOUR;
                const bucketRangeLabel = props.state.formatBucketRange(bucketStart, bucketEnd);
                const bucketDurationText =
                  trends.bucketSize % 24 === 0
                    ? `${trends.bucketSize / 24} day${trends.bucketSize / 24 === 1 ? '' : 's'}`
                    : `${trends.bucketSize} hour${trends.bucketSize === 1 ? '' : 's'}`;
                const countLabel = getAlertBucketCountLabel(value);
                const tooltipContent = [
                  countLabel,
                  `${bucketDurationText} period`,
                  bucketRangeLabel,
                ].join('\n');

                return (
                  <div
                    class="relative flex flex-1 cursor-pointer items-end"
                    role="button"
                    tabIndex={0}
                    aria-pressed={isSelected}
                    aria-label={`${countLabel} between ${bucketRangeLabel}`}
                    onClick={() =>
                      props.state.setSelectedBarIndex(
                        index === props.state.selectedBarIndex() ? null : index,
                      )
                    }
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        props.state.setSelectedBarIndex(
                          index === props.state.selectedBarIndex() ? null : index,
                        );
                      }
                    }}
                  >
                    <div class="absolute bottom-0 h-1 w-full rounded-full bg-slate-300 opacity-30"></div>
                    <div
                      class="relative w-full rounded-sm transition-all"
                      style={{
                        height: `${pixelHeight}px`,
                        'background-color':
                          value > 0 ? (isSelected ? '#2563eb' : '#3b82f6') : 'transparent',
                        opacity: isSelected ? '1' : '0.8',
                        'box-shadow': isSelected ? '0 0 0 2px rgba(37, 99, 235, 0.4)' : 'none',
                      }}
                      title={bucketRangeLabel}
                      onMouseEnter={(event) => {
                        if (value <= 0) {
                          hideTooltip();
                          return;
                        }
                        const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
                        showTooltip(tooltipContent, rect.left + rect.width / 2, rect.top, {
                          align: 'center',
                          direction: 'up',
                        });
                      }}
                      onMouseLeave={() => hideTooltip()}
                    />
                  </div>
                );
              })}
            </div>
          </div>
        );
      })()}

      <Show when={props.state.axisTicks().length > 0}>
        <div class="relative mt-3 h-10">
          <div class="absolute inset-x-0 top-0 h-px bg-surface-hover"></div>
          <For each={props.state.axisTicks()}>
            {(tick) => (
              <div
                class="pointer-events-none absolute top-0 flex h-full flex-col items-center"
                style={{ left: `${tick.position * 100}%` }}
              >
                <div class="h-3 w-px bg-slate-300"></div>
                <div
                  class="mt-1 whitespace-nowrap text-[10px] text-muted transform"
                  classList={{
                    '-translate-x-1/2': tick.align === 'center',
                    '-translate-x-full': tick.align === 'end',
                  }}
                >
                  {tick.label}
                </div>
              </div>
            )}
          </For>
        </div>
      </Show>
    </Card>
  );
}
