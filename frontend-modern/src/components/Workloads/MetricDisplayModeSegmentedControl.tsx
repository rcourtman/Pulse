import { splitProps, type Component, type JSX } from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import BarChartIcon from 'lucide-solid/icons/bar-chart';

import { FilterSegmentedControl } from '@/components/shared/FilterToolbar';

import type { WorkloadsMetricDisplayMode } from './workloadsFilterModel';

interface MetricDisplayModeSegmentedControlProps extends Omit<
  JSX.HTMLAttributes<HTMLDivElement>,
  'onChange'
> {
  value: WorkloadsMetricDisplayMode;
  onChange: (value: WorkloadsMetricDisplayMode) => void;
}

export const MetricDisplayModeSegmentedControl: Component<
  MetricDisplayModeSegmentedControlProps
> = (props) => {
  const [local, divProps] = splitProps(props, ['value', 'onChange']);

  return (
    <FilterSegmentedControl
      {...divProps}
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
              Sparklines
            </>
          ),
        },
      ]}
    />
  );
};
