import { Show, createMemo, createSignal, onMount, onCleanup } from 'solid-js';
import { Sparkline } from '@/components/shared/Sparkline';
import { useMetricsViewMode } from '@/stores/metricsViewMode';
import { getMetricHistoryForRange, getMetricsVersion } from '@/stores/metricsHistory';

interface MetricBarProps {
  value: number;
  label: string;
  sublabel?: string;
  type?: 'cpu' | 'memory' | 'disk' | 'generic';
  resourceId?: string; // Required for sparkline mode to fetch history
  class?: string;
}

// Estimate text width based on character count (rough approximation for 10px font)
// Average char width ~6px at 10px font size
const estimateTextWidth = (text: string): number => {
  return text.length * 5.5 + 8; // chars * avg width + padding
};

export function MetricBar(props: MetricBarProps) {
  const { viewMode, timeRange } = useMetricsViewMode();
  const width = createMemo(() => Math.min(props.value, 100));

  // Track container width
  const [containerWidth, setContainerWidth] = createSignal(100);
  let containerRef: HTMLDivElement | undefined;

  // Set up ResizeObserver to track container width changes
  onMount(() => {
    if (!containerRef) return;

    setContainerWidth(containerRef.offsetWidth);

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setContainerWidth(entry.contentRect.width);
      }
    });

    observer.observe(containerRef);

    onCleanup(() => observer.disconnect());
  });

  // Determine if sublabel fits based on estimated text width
  const showSublabel = createMemo(() => {
    if (!props.sublabel) return false;
    const fullText = `${props.label} (${props.sublabel})`;
    const estimatedWidth = estimateTextWidth(fullText);
    return containerWidth() >= estimatedWidth;
  });

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
  // Depends on metricsVersion to re-fetch when data is seeded (e.g., on time range change)
  const metricHistory = createMemo(() => {
    // Subscribe to version changes so we re-read when new data is seeded
    getMetricsVersion();
    if (viewMode() !== 'sparklines' || !props.resourceId) return [];
    return getMetricHistoryForRange(props.resourceId, timeRange());
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
        // Progress bar mode - full width, flex centered like stacked bars
        <div ref={containerRef} class="metric-text w-full h-4 flex items-center justify-center">
          <div class={`relative w-full h-full overflow-hidden bg-gray-200 dark:bg-gray-600 rounded ${props.class || ''}`}>
            <div class={`absolute top-0 left-0 h-full ${progressColorClass()}`} style={{ width: `${width()}%` }} />
            <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-gray-700 dark:text-gray-100 leading-none">
              <span class="flex items-center gap-1 whitespace-nowrap px-0.5">
                <span>{props.label}</span>
                <Show when={showSublabel()}>
                  <span class="metric-sublabel font-normal text-gray-500 dark:text-gray-300">
                    ({props.sublabel})
                  </span>
                </Show>
              </span>
            </span>
          </div>
        </div>
      }
    >
      {/* Sparkline mode - full width, flex centered like stacked bars */}
      <div class="metric-text w-full h-4 flex items-center justify-center min-w-0 overflow-hidden">
        <Sparkline
          data={metricHistory()}
          metric={sparklineMetric()}
          width={0}
          height={16}
        />
      </div>
    </Show>
  );
}
