import { Show, splitProps, type Component, type JSX } from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import BarChartIcon from 'lucide-solid/icons/bar-chart';

import { FilterSegmentedControl } from '@/components/shared/FilterToolbar';

import type { WorkloadsMetricDisplayMode } from './workloadsFilterModel';
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

export const MetricDisplayModeSegmentedControl: Component<
  MetricDisplayModeSegmentedControlProps
> = (props) => {
  const [local, divProps] = splitProps(props, ['value', 'onChange', 'range', 'onRangeChange']);

  return (
    <div
      {...divProps}
      class={`flex flex-wrap items-center justify-end gap-2 ${divProps.class ?? ''}`.trim()}
    >
      <FilterSegmentedControl
        aria-label={divProps['aria-label'] ?? 'Metric display'}
        value={local.value}
        onChange={(value) => local.onChange(value as WorkloadsMetricDisplayMode)}
        options={[
          {
            value: 'bars',
            title: 'Show current values as progress bars',
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
        <FilterSegmentedControl
          aria-label="Sparkline range"
          value={local.range!}
          onChange={(value) => local.onRangeChange?.(value as WorkloadTableMetricHistoryRange)}
          options={WORKLOAD_TABLE_HISTORY_RANGES.map((range) => ({
            value: range,
            title: `Show table sparklines for ${WORKLOAD_TABLE_HISTORY_RANGE_LABELS[range]}`,
            label: WORKLOAD_TABLE_HISTORY_RANGE_LABELS[range],
          }))}
        />
      </Show>
    </div>
  );
};
