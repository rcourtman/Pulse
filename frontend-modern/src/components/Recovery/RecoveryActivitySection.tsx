import { For, Show } from 'solid-js';
import type { Accessor, Component } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { hideTooltip, showTooltip } from '@/components/shared/Tooltip';
import { segmentedButtonClass } from '@/utils/segmentedButton';
import {
  getRecoveryBreadcrumbLinkClass,
} from '@/utils/recoveryActionPresentation';
import { getRecoveryFilterChipPresentation } from '@/utils/recoveryFilterChipPresentation';
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
  getRecoveryTimelineBarMinWidthClass,
  getRecoveryTimelineLabelEvery,
  RECOVERY_TIMELINE_LEGEND_ITEM_CLASS,
  RECOVERY_TIMELINE_RANGE_GROUP_CLASS,
} from '@/utils/recoveryTimelineChartPresentation';
import { getRecoveryTimelineColumnButtonClass } from '@/utils/recoveryTimelinePresentation';

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

interface RecoveryActivitySectionProps {
  activitySummary: Accessor<ActivitySummary>;
  activeClusterLabel: Accessor<string>;
  activeItemTypeLabel: Accessor<string>;
  activeNamespaceLabel: Accessor<string>;
  activeNodeLabel: Accessor<string>;
  chartRangeDays: Accessor<7 | 30 | 90 | 365>;
  clearClusterFilter: () => void;
  clearFocusedRollup: () => void;
  clearItemTypeFilter: () => void;
  clearNamespaceFilter: () => void;
  clearNodeFilter: () => void;
  clearSelectedDate: () => void;
  hasFocusedRollup: Accessor<boolean>;
  isMobile: boolean;
  loading: Accessor<boolean>;
  overallRollupsSummary: Accessor<RecoveryRollupSummary>;
  selectedDateKey: Accessor<string | null>;
  selectedDateLabel: Accessor<string>;
  selectedHistoryItemLabel: Accessor<string | null>;
  setChartRangeDays: (value: 7 | 30 | 90 | 365) => void;
  toggleSelectedDate: (key: string) => void;
  timeline: Accessor<TimelineModel>;
}

const rangeOptions: Array<7 | 30 | 90 | 365> = [7, 30, 90, 365];

export const RecoveryActivitySection: Component<RecoveryActivitySectionProps> = (props) => (
  <Card
    padding="sm"
    class="h-full border-border-subtle bg-surface"
  >
    <div class="mb-2.5 flex flex-col gap-2.5">
      <div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex min-w-0 flex-col gap-1.5">
          <div class="flex flex-wrap items-baseline gap-x-2 gap-y-1">
            <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
              Recovery Activity
            </div>
            <div class="text-xs text-muted">
              Daily recovery points across the selected history window.
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted">
            <span class="font-medium text-base-content">
              {props.activitySummary().totalPoints} recovery points
            </span>
            <span>{props.activitySummary().averagePerDay.toFixed(1)} per day</span>
            <span>{props.activitySummary().activeDays} active days</span>
            <Show when={props.overallRollupsSummary().stale > 0}>
              <span class="font-medium text-amber-600 dark:text-amber-400">
                {props.overallRollupsSummary().stale} stale
              </span>
            </Show>
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <Show when={props.selectedHistoryItemLabel()}>
            <div class="inline-flex items-center gap-2 rounded-md border border-border-subtle bg-surface/70 px-2.5 py-1.5 text-xs">
              <span class="font-semibold uppercase tracking-[0.14em] text-muted">Focused Item</span>
              <span class="max-w-[18rem] truncate font-medium text-base-content">
                {props.selectedHistoryItemLabel()}
              </span>
            </div>
          </Show>
          <Show
            when={props.hasFocusedRollup()}
            fallback={<span class="text-xs text-muted">All protected items</span>}
          >
            <button type="button" onClick={props.clearFocusedRollup} class={getRecoveryBreadcrumbLinkClass()}>
              All history
            </button>
          </Show>
        </div>
      </div>
    </div>

    <Show
      when={
        props.selectedDateKey() ||
        props.activeItemTypeLabel() ||
        props.activeClusterLabel() ||
        props.activeNodeLabel() ||
        props.activeNamespaceLabel()
      }
    >
      <div class="mb-2 flex flex-wrap items-center gap-1.5">
        <Show when={props.selectedDateKey()}>
          {(() => {
            const chip = getRecoveryFilterChipPresentation('day');
            return (
              <div class={chip.className}>
                <span class="font-medium uppercase tracking-wide">{chip.label}</span>
                <span class="truncate font-mono text-[10px]" title={props.selectedDateLabel()}>
                  {props.selectedDateLabel()}
                </span>
                <button type="button" onClick={props.clearSelectedDate} class={chip.clearButtonClass}>
                  Clear
                </button>
              </div>
            );
          })()}
        </Show>
        <Show when={props.activeClusterLabel()}>
          {(() => {
            const chip = getRecoveryFilterChipPresentation('cluster');
            return (
              <div class={chip.className}>
                <span class="font-medium uppercase tracking-wide">{chip.label}</span>
                <span class="truncate font-mono text-[10px]" title={props.activeClusterLabel()}>
                  {props.activeClusterLabel()}
                </span>
                <button type="button" onClick={props.clearClusterFilter} class={chip.clearButtonClass}>
                  Clear
                </button>
              </div>
            );
          })()}
        </Show>
        <Show when={props.activeItemTypeLabel()}>
          {(() => {
            const chip = getRecoveryFilterChipPresentation('item-type');
            return (
              <div class={chip.className}>
                <span class="font-medium uppercase tracking-wide">{chip.label}</span>
                <span class="truncate font-mono text-[10px]" title={props.activeItemTypeLabel()}>
                  {props.activeItemTypeLabel()}
                </span>
                <button
                  type="button"
                  onClick={props.clearItemTypeFilter}
                  class={chip.clearButtonClass}
                >
                  Clear
                </button>
              </div>
            );
          })()}
        </Show>
        <Show when={props.activeNodeLabel()}>
          {(() => {
            const chip = getRecoveryFilterChipPresentation('node');
            return (
              <div class={chip.className}>
                <span class="font-medium uppercase tracking-wide">{chip.label}</span>
                <span class="truncate font-mono text-[10px]" title={props.activeNodeLabel()}>
                  {props.activeNodeLabel()}
                </span>
                <button type="button" onClick={props.clearNodeFilter} class={chip.clearButtonClass}>
                  Clear
                </button>
              </div>
            );
          })()}
        </Show>
        <Show when={props.activeNamespaceLabel()}>
          {(() => {
            const chip = getRecoveryFilterChipPresentation('namespace');
            return (
              <div data-testid="active-namespace-chip" class={chip.className}>
                <span class="font-medium uppercase tracking-wide">{chip.label}</span>
                <span class="truncate font-mono text-[10px]" title={props.activeNamespaceLabel()}>
                  {props.activeNamespaceLabel()}
                </span>
                <button
                  type="button"
                  onClick={props.clearNamespaceFilter}
                  class={chip.clearButtonClass}
                >
                  Clear
                </button>
              </div>
            );
          })()}
        </Show>
      </div>
    </Show>

      <div class="rounded-lg border border-border-subtle bg-surface-alt/25 p-2.5">
        <div class={`mb-2.5 flex flex-wrap items-center justify-between gap-3 ${RECOVERY_TIMELINE_RANGE_GROUP_CLASS}`}>
          <div class="text-[11px] text-muted">Range</div>
          <div class={RECOVERY_TIMELINE_RANGE_GROUP_CLASS}>
            <For each={rangeOptions}>
              {(range) => (
                <button
                  type="button"
                  onClick={() => props.setChartRangeDays(range)}
                  class={`px-2 py-1 ${segmentedButtonClass(props.chartRangeDays() === range, false, 'accent')}`}
                >
                  {range}d
                </button>
              )}
            </For>
          </div>
        </div>

        <Show
          when={!props.loading() && props.timeline().points.length > 0}
          fallback={
            <div class="flex h-40 items-center justify-center text-sm text-muted">
              {props.loading()
                ? getRecoveryActivityLoadingState().text
                : getRecoveryActivityEmptyState().text}
            </div>
          }
        >
          <div class="relative">
            <div class="mb-2 flex flex-wrap items-center justify-end gap-3 text-[10px] text-muted">
              <div class={RECOVERY_TIMELINE_LEGEND_ITEM_CLASS}>
                <span class={`h-2.5 w-2.5 rounded ${getRecoveryArtifactModePresentation('snapshot').segmentClassName}`} />
                {getRecoveryArtifactModePresentation('snapshot').aggregateLabel}
              </div>
              <div class={RECOVERY_TIMELINE_LEGEND_ITEM_CLASS}>
                <span class={`h-2.5 w-2.5 rounded ${getRecoveryArtifactModePresentation('local').segmentClassName}`} />
                {getRecoveryArtifactModePresentation('local').aggregateLabel}
              </div>
              <div class={RECOVERY_TIMELINE_LEGEND_ITEM_CLASS}>
                <span class={`h-2.5 w-2.5 rounded ${getRecoveryArtifactModePresentation('remote').segmentClassName}`} />
                {getRecoveryArtifactModePresentation('remote').aggregateLabel}
              </div>
            </div>

            <div class="grid grid-cols-[auto_minmax(0,1fr)] gap-3">
              <div class="flex h-24 flex-col justify-between text-[10px] text-muted">
                <For each={[0, 1, 2, 3, 4]}>
                  {(step) => {
                    const value = Math.round((props.timeline().axisMax * (4 - step)) / 4);
                    return <span>{value}</span>;
                  }}
                </For>
              </div>

              <div class="relative h-28">
                <div
                  data-testid="recovery-activity-bars"
                  class="absolute inset-x-0 bottom-4 top-0 flex items-stretch gap-[3px]"
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
                      const isSelected = props.selectedDateKey() === point.key;

                      return (
                        <div class="flex-1 self-stretch">
                          <button
                            type="button"
                            class={`h-full w-full rounded-sm ${getRecoveryTimelineColumnButtonClass(isSelected)}`}
                            aria-label={`${getRecoveryPrettyDateLabel(point.key)}: ${total} recovery points`}
                            onClick={() => props.toggleSelectedDate(point.key)}
                            onMouseEnter={(event) => {
                              const rect = event.currentTarget.getBoundingClientRect();
                              const breakdown: string[] = [];
                              if (point.snapshot > 0) {
                                breakdown.push(
                                  `${getRecoveryArtifactModePresentation('snapshot').aggregateLabel}: ${point.snapshot}`,
                                );
                              }
                              if (point.local > 0) {
                                breakdown.push(
                                  `${getRecoveryArtifactModePresentation('local').aggregateLabel}: ${point.local}`,
                                );
                              }
                              if (point.remote > 0) {
                                breakdown.push(
                                  `${getRecoveryArtifactModePresentation('remote').aggregateLabel}: ${point.remote}`,
                                );
                              }
                              const tooltipText =
                                point.total > 0
                                  ? `${getRecoveryPrettyDateLabel(point.key)}\nAvailable: ${point.total} recovery point${point.total > 1 ? 's' : ''}\n${breakdown.join(' • ')}`
                                  : `${getRecoveryPrettyDateLabel(point.key)}\nNo recovery points available`;
                              showTooltip(tooltipText, rect.left + rect.width / 2, rect.top, {
                                align: 'center',
                                direction: 'up',
                              });
                            }}
                            onMouseLeave={() => hideTooltip()}
                            onFocus={(event) => {
                              const rect = event.currentTarget.getBoundingClientRect();
                              const tooltipText = `${getRecoveryPrettyDateLabel(point.key)}\nAvailable: ${point.total} recovery point${point.total > 1 ? 's' : ''}`;
                              showTooltip(tooltipText, rect.left + rect.width / 2, rect.top, {
                                align: 'center',
                                direction: 'up',
                              });
                            }}
                            onBlur={() => hideTooltip()}
                          >
                            <div class="relative h-full w-full">
                              <Show when={total > 0}>
                                <div
                                  class="absolute inset-x-0 bottom-0"
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

                <div class="pointer-events-none absolute inset-x-0 bottom-0 h-4 flex items-end gap-[3px]">
                  <For each={props.timeline().points}>
                    {(point, index) => {
                      const showLabel =
                        index() % getRecoveryTimelineLabelEvery(props.timeline().points.length) === 0 ||
                        index() === props.timeline().points.length - 1;
                      const isSelected = props.selectedDateKey() === point.key;
                      const barMinWidth = getRecoveryTimelineBarMinWidthClass(
                        props.isMobile,
                        props.chartRangeDays(),
                      );
                      return (
                        <div
                          class={`relative flex-1 ${props.isMobile ? '' : 'shrink-0'} ${barMinWidth}`}
                        >
                          <Show when={showLabel}>
                            <span
                              class={`absolute bottom-0 left-1/2 -translate-x-1/2 whitespace-nowrap text-[9px] ${
                                isSelected
                                  ? getRecoveryTimelineAxisLabelClass(true)
                                  : getRecoveryTimelineAxisLabelClass(false)
                              }`}
                            >
                              {getRecoveryCompactAxisLabel(point.key, props.chartRangeDays())}
                            </span>
                          </Show>
                        </div>
                      );
                    }}
                  </For>
                </div>
              </div>
            </div>
          </div>
        </Show>
    </div>
  </Card>
);
