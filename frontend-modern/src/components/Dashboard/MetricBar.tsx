import { Show, createMemo } from 'solid-js';
import { Sparkline } from '@/components/shared/Sparkline';
import { useMetricsViewMode } from '@/stores/metricsViewMode';
import { getMetricHistory } from '@/stores/metricsHistory';

interface MetricBarProps {
  value: number;
  label: string;
  sublabel?: string;
  type?: 'cpu' | 'memory' | 'disk' | 'generic';
  resourceId?: string; // Required for sparkline mode to fetch history
}

export function MetricBar(props: MetricBarProps) {
  const { viewMode } = useMetricsViewMode();
  const width = createMemo(() => Math.min(props.value, 100));

  // Get color based on percentage and metric type (matching original)
  const getColor = createMemo(() => {
    const percentage = props.value;
    const metric = props.type || 'generic';

    if (metric === 'cpu') {
      if (percentage >= 90) return 'red';
      if (percentage >= 80) return 'yellow';
      return 'green';
    } else if (metric === 'memory') {
      if (percentage >= 85) return 'red';
      if (percentage >= 75) return 'yellow';
      return 'green';
    } else if (metric === 'disk') {
      if (percentage >= 90) return 'red';
      if (percentage >= 80) return 'yellow';
      return 'green';
    } else {
      if (percentage >= 90) return 'red';
      if (percentage >= 75) return 'yellow';
      return 'green';
    }
  });

  // Map color to CSS classes
  const progressColorClass = createMemo(() => {
    const colorMap = {
      red: 'bg-red-500/60 dark:bg-red-500/50',
      yellow: 'bg-yellow-500/60 dark:bg-yellow-500/50',
      green: 'bg-green-500/60 dark:bg-green-500/50',
    };
    return colorMap[getColor()] || 'bg-gray-500/60 dark:bg-gray-500/50';
  });

  // Get metric history for sparkline
  const metricHistory = createMemo(() => {
    if (viewMode() !== 'sparklines' || !props.resourceId) return [];
    return getMetricHistory(props.resourceId);
  });

  // Determine which metric type to use for sparkline
  const sparklineMetric = (): 'cpu' | 'memory' | 'disk' => {
    const type = props.type || 'cpu';
    if (type === 'generic') return 'cpu';
    return type;
  };

  return (
    <Show
      when={viewMode() === 'sparklines' && props.resourceId}
      fallback={
        // Original progress bar mode
        <div class="metric-text w-full h-6 flex items-center">
          {/* On very small screens (< lg), show compact percentage with color indicator */}
          <div class="lg:hidden relative w-full h-3.5 flex items-center justify-center rounded overflow-hidden bg-gray-100 dark:bg-gray-700">
            {/* Slim color indicator bar */}
            <div class={`absolute bottom-0 left-0 h-1 w-full ${progressColorClass()}`} />
            <span class={`text-[10px] font-medium z-10 ${
              getColor() === 'red' ? 'text-red-700 dark:text-red-300' :
              getColor() === 'yellow' ? 'text-yellow-700 dark:text-yellow-300' :
              'text-gray-800 dark:text-gray-100'
            }`}>
              {props.label}
            </span>
          </div>
          {/* On larger screens (>= lg), show full progress bar */}
          <div class="hidden lg:block relative w-full h-3.5 rounded overflow-hidden bg-gray-200 dark:bg-gray-600">
            <div class={`absolute top-0 left-0 h-full ${progressColorClass()}`} style={{ width: `${width()}%` }} />
            <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
              <span class="flex items-center gap-1 whitespace-nowrap px-0.5">
                <span>{props.label}</span>
                <Show when={props.sublabel}>
                  {(sublabel) => (
                    <span class="metric-sublabel hidden xl:inline font-normal">
                      ({sublabel()})
                    </span>
                  )}
                </Show>
              </span>
            </span>
          </div>
        </div>
      }
    >
      {/* Sparkline mode */}
      <div class="metric-text w-full h-6 flex items-center gap-1.5">
        <div class="flex-1 min-w-0">
          <Sparkline
            data={metricHistory()}
            metric={sparklineMetric()}
            width={0}
            height={24}
          />
        </div>
        <span class="text-[10px] font-medium text-gray-800 dark:text-gray-100 whitespace-nowrap flex-shrink-0 min-w-[35px]">
          {props.label}
        </span>
      </div>
    </Show>
  );
}
