import { Show, createMemo, createSignal, onMount, onCleanup } from 'solid-js';
import { Sparkline, SparklineLockBadge } from '@/components/shared/Sparkline';
import { useMetricsViewMode } from '@/stores/metricsViewMode';
import { isRangeLocked } from '@/stores/license';
import { getSparklineData, getSparklineStore, isSparklineLoading } from '@/stores/sparklineData';
import { estimateTextWidth } from '@/utils/format';
import { getMetricColorClass } from '@/utils/metricThresholds';
import type { MetricType } from '@/utils/metricThresholds';

interface MetricBarProps {
  value: number;
  label: string;
  sublabel?: string;
  showLabel?: boolean;
  type?: 'cpu' | 'memory' | 'disk' | 'generic';
  resourceId?: string; // Required for sparkline mode to fetch history
  class?: string;
}


export function MetricBar(props: MetricBarProps) {
  const { viewMode, timeRange } = useMetricsViewMode();

  const locked = () => isRangeLocked(timeRange());
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
    if (props.showLabel === false) return false;
    if (!props.sublabel) return false;
    const fullText = `${props.label} (${props.sublabel})`;
    const estimatedWidth = estimateTextWidth(fullText);
    return containerWidth() >= estimatedWidth;
  });

  const showLabel = createMemo(() => props.showLabel !== false && props.label.trim().length > 0);

  // Get color class from centralized thresholds
  const progressColorClass = createMemo(() => {
    const metric = props.type || 'cpu';
    // 'generic' falls back to cpu thresholds
    const metricType: MetricType = metric === 'generic' ? 'cpu' : metric;
    return getMetricColorClass(props.value, metricType);
  });

  // Determine which metric type to use for sparkline
  const sparklineMetric = (): 'cpu' | 'memory' | 'disk' => {
    const type = props.type || 'cpu';
    if (type === 'generic') return 'cpu';
    return type;
  };

  // Get sparkline points from the new sparkline data store
  const sparklinePoints = createMemo(() => {
    getSparklineStore(); // reactive dependency
    if (viewMode() !== 'sparklines' || !props.resourceId) return [];
    return getSparklineData(props.resourceId, sparklineMetric());
  });

  return (
    <Show
      when={viewMode() === 'sparklines' && props.resourceId}
      fallback={
        // Progress bar mode - full width, flex centered like stacked bars
        <div ref={containerRef} class="metric-text w-full h-4 flex items-center justify-center min-w-0">
          <div class={`relative w-full h-full overflow-hidden bg-gray-200 dark:bg-gray-600 rounded ${props.class || ''}`}>
            <div class={`absolute top-0 left-0 h-full ${progressColorClass()}`} style={{ width: `${width()}%` }} />
            <Show when={showLabel()}>
              <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-gray-700 dark:text-gray-100 leading-none min-w-0 overflow-hidden">
                <span class="max-w-full min-w-0 whitespace-nowrap overflow-hidden text-ellipsis px-0.5 text-center">
                  <span>{props.label}</span>
                  <Show when={showSublabel()}>
                    <span class="metric-sublabel font-normal text-gray-500 dark:text-gray-300">
                      {' '}({props.sublabel})
                    </span>
                  </Show>
                </span>
              </span>
            </Show>
          </div>
        </div>
      }
    >
      {/* Sparkline mode - full width, flex centered like stacked bars */}
      <div class="metric-text w-full h-4 flex items-center justify-center min-w-0 overflow-hidden">
        <Show when={!locked()} fallback={<SparklineLockBadge />}>
          <Show
            when={!isSparklineLoading() || sparklinePoints().length > 0}
            fallback={
              <span class="text-[9px] text-gray-400 dark:text-gray-500 animate-pulse">loading...</span>
            }
          >
            <Sparkline
              data={sparklinePoints()}
              metric={sparklineMetric()}
              width={0}
              height={16}
            />
          </Show>
        </Show>
      </div>
    </Show>
  );
}
