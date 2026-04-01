import { For, Show } from 'solid-js';
import type { Accessor, Component, JSX } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { hideTooltip, showTooltip } from '@/components/shared/Tooltip';
import {
  getRecoveryActivityEmptyState,
  getRecoveryActivityLoadingState,
} from '@/utils/recoveryEmptyStatePresentation';
import { getRecoveryFilterChipPresentation } from '@/utils/recoveryFilterChipPresentation';
import { getRecoveryArtifactModePresentation } from '@/utils/recoveryArtifactModePresentation';
import {
  getRecoveryPrettyDateLabel,
  getRecoveryCompactAxisLabel,
} from '@/utils/recoveryDatePresentation';
import {
  getRecoveryTimelineAxisLabelClass,
  getRecoveryTimelineAxisTicks,
  RECOVERY_TIMELINE_LEGEND_ITEM_CLASS,
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
  clearItemTypeFilter: () => void;
  clearNamespaceFilter: () => void;
  clearNodeFilter: () => void;
  clearSelectedDate: () => void;
  isMobile: boolean;
  loading: Accessor<boolean>;
  overallRollupsSummary: Accessor<RecoveryRollupSummary>;
  selectedDateKey: Accessor<string | null>;
  selectedDateLabel: Accessor<string>;
  toggleSelectedDate: (key: string) => void;
  timeline: Accessor<TimelineModel>;
}

function RecoveryActivitySectionContent(props: RecoveryActivitySectionProps): JSX.Element {
  const timelineAxisTicks = () => getRecoveryTimelineAxisTicks(props.timeline().points.length);

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
          <div class="flex flex-wrap items-center gap-2" />
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
        <div class="mb-1 flex flex-wrap items-center gap-1.5">
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

      <div class="rounded-lg border border-border-subtle bg-surface-alt/25 px-2 py-1.5">
        <div class="mb-1 flex flex-wrap items-center justify-end gap-1.5 text-[9px] text-muted">
          <div class="flex flex-wrap items-center gap-1.5 text-[9px] text-muted">
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
              <div class="flex h-16 flex-col justify-between text-[10px] text-muted">
                <For each={[0, 1, 2, 3, 4]}>
                  {(step) => {
                    const value = Math.round((props.timeline().axisMax * (4 - step)) / 4);
                    return <span>{value}</span>;
                  }}
                </For>
              </div>

              <div class="relative h-20">
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

                <div class="pointer-events-none absolute inset-x-0 bottom-0 h-4">
                  <For each={timelineAxisTicks()}>
                    {(tick) => {
                      const point = props.timeline().points[tick.index];
                      const isSelected = props.selectedDateKey() === point.key;
                      const alignmentClass =
                        tick.align === 'start'
                          ? 'left-0 text-left'
                          : tick.align === 'end'
                            ? 'left-full -translate-x-full text-right'
                            : '-translate-x-1/2 text-center';

                      return (
                        <span
                          class={`absolute bottom-0 whitespace-nowrap text-[9px] ${
                            isSelected
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
