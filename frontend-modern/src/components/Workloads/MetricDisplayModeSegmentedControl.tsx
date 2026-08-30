import { Show, splitProps, type Component, type JSX } from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import BarChartIcon from 'lucide-solid/icons/bar-chart';

import { FilterSegmentedControl } from '@/components/shared/FilterToolbar';

import type { WorkloadsMetricDisplayMode, WorkloadsMetricHoverMode } from './workloadsFilterModel';
import {
  WORKLOAD_TABLE_HISTORY_RANGE_LABELS,
  WORKLOAD_TABLE_HISTORY_RANGES,
  type WorkloadTableMetricHistoryRange,
} from './workloadMetricHistoryModel';

interface MetricDisplayModeSegmentedControlProps extends Omit<
  JSX.HTMLAttributes<HTMLDivElement>,
  'onChange'
> {
  value: WorkloadsMetricDisplayMode;
  onChange: (value: WorkloadsMetricDisplayMode) => void;
  range?: WorkloadTableMetricHistoryRange;
  onRangeChange?: (range: WorkloadTableMetricHistoryRange) => void;
}

interface MetricHistoryRangeSegmentedControlProps {
  range: WorkloadTableMetricHistoryRange;
  onRangeChange: (range: WorkloadTableMetricHistoryRange) => void;
  label?: JSX.Element;
}

interface MetricHoverModeSegmentedControlProps {
  value: WorkloadsMetricHoverMode;
  onChange: (value: WorkloadsMetricHoverMode) => void;
}

export const MetricHoverModeSegmentedControl: Component<MetricHoverModeSegmentedControlProps> = (
  props,
) => (
  <FilterSegmentedControl
    aria-label="Row hover behavior"
    value={props.value}
    onChange={(value) => props.onChange(value as WorkloadsMetricHoverMode)}
    options={[
      {
        value: 'details',
        title: 'Keep current-value bars and show their detailed tooltips',
        label: 'Details',
      },
      {
        value: 'history',
        title: 'Preview synchronized CPU, memory, and disk history across the row',
        label: 'History',
      },
    ]}
  />
);

export const MetricHistoryRangeSegmentedControl: Component<
  MetricHistoryRangeSegmentedControlProps
> = (props) => (
  <FilterSegmentedControl
    aria-label="Sparkline range"
    label={props.label}
    value={props.range}
    onChange={(value) => props.onRangeChange(value as WorkloadTableMetricHistoryRange)}
    options={WORKLOAD_TABLE_HISTORY_RANGES.map((range) => ({
      value: range,
      title: `Show table sparklines for ${WORKLOAD_TABLE_HISTORY_RANGE_LABELS[range]}`,
      label: WORKLOAD_TABLE_HISTORY_RANGE_LABELS[range],
    }))}
  />
);

export const MetricDisplayModeSegmentedControl: Component<
  MetricDisplayModeSegmentedControlProps
> = (props) => {
  const [local, divProps] = splitProps(props, ['value', 'onChange', 'range', 'onRangeChange']);

  return (
    <div
      {...divProps}
      class={`flex flex-wrap items-center justify-start gap-2 ${divProps.class ?? ''}`.trim()}
    >
      <FilterSegmentedControl
        aria-label={divProps['aria-label'] ?? 'Metric display'}
        value={local.value}
        onChange={(value) => local.onChange(value as WorkloadsMetricDisplayMode)}
        options={[
          {
            value: 'bars',
            title: 'Show current-value bars and preview row history on hover',
            label: (
              <>
                <BarChartIcon class="h-3 w-3" />
                Bars
              </>
            ),
          },
          {
            value: 'sparklines',
            title: 'Show recent metric history as mini sparklines',
            label: (
              <>
                <ActivityIcon class="h-3 w-3" />
                Trends
              </>
            ),
          },
        ]}
      />
      <Show when={local.value === 'sparklines' && local.range && local.onRangeChange}>
        <MetricHistoryRangeSegmentedControl
          range={local.range!}
          onRangeChange={local.onRangeChange!}
        />
      </Show>
    </div>
  );
};
