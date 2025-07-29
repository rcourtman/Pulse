import { Component, onMount, onCleanup, createMemo, createEffect, Show } from 'solid-js';
import Sparkline from '@/components/Dashboard/Sparkline';
import { observeChart, subscribeToChartDimension } from '@/stores/chartDimensions';
import { useIntersectionObserver } from '@/hooks/useIntersectionObserver';
import type { ChartPoint } from '@/types/charts';

interface DynamicChartProps {
  data: ChartPoint[] | undefined;
  metric: string;
  guestId: string;
  chartType?: 'mini' | 'sparkline' | 'storage';
  containerClass?: string;
  paddingAdjustment?: number;
  lazy?: boolean; // Enable lazy loading
  filled?: boolean; // Pass through to Sparkline
  forceGray?: boolean; // Force gray color
}

/**
 * Optimized dynamic chart component that uses a shared ResizeObserver for better performance.
 * This ensures charts fill their available space without stretching or distortion.
 */
export const DynamicChart: Component<DynamicChartProps> = (props) => {
  const chartType = () => props.chartType || 'mini';
  const chartId = () => `${props.guestId}-${props.metric}`;
  
  const defaultWidth = () => {
    switch (chartType()) {
      case 'storage': return 184;
      case 'mini': return 118;
      case 'sparkline': return 66;
      default: return 118;
    }
  };
  
  let containerRef: HTMLDivElement | undefined;
  let cleanupFn: (() => void) | undefined;
  
  // Use intersection observer for lazy loading if enabled
  const isVisible = props.lazy ? useIntersectionObserver(() => containerRef) : () => true;
  
  // Create a memoized signal for the chart dimension
  const containerWidth = createMemo(() => {
    const dimension = subscribeToChartDimension(chartId())();
    if (dimension > 0) {
      return dimension - (props.paddingAdjustment || 0);
    }
    return defaultWidth();
  });

  onMount(() => {
    if (containerRef && isVisible()) {
      // Register with shared ResizeObserver only when visible
      cleanupFn = observeChart(containerRef, chartId());
    }
  });
  
  // Re-register when visibility changes
  createEffect(() => {
    if (containerRef && isVisible() && !cleanupFn) {
      cleanupFn = observeChart(containerRef, chartId());
    }
  });

  onCleanup(() => {
    // Don't clean up if we're just re-rendering the same chart
    // This prevents the dimension from resetting and causing animations
    if (cleanupFn) {
      cleanupFn();
    }
  });

  const getHeight = createMemo(() => {
    switch (chartType()) {
      case 'storage': return 14;
      case 'mini': return 20;
      case 'sparkline': return 16;
      default: return 20;
    }
  });

  // Only render if we have valid data, width, and visibility
  const shouldRender = createMemo(() => {
    return isVisible() && containerWidth() > 0 && props.data && (Array.isArray(props.data) ? props.data.length > 0 : true);
  });
  
  // Show loading skeleton immediately when visible but no data
  const showLoadingSkeleton = createMemo(() => {
    return isVisible() && containerWidth() > 0 && (!props.data || (Array.isArray(props.data) && props.data.length === 0));
  });

  return (
    <div ref={containerRef!} class={props.containerClass || "w-full h-full flex items-center justify-center"}>
      <Show 
        when={shouldRender()}
        fallback={
          <Show when={showLoadingSkeleton()} fallback={null}>
            <div 
              class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded"
              style={{ width: `${containerWidth()}px`, height: `${getHeight()}px` }}
            />
          </Show>
        }
      >
        <Sparkline
          data={props.data!}
          metric={props.metric}
          guestId={props.guestId}
          chartType={chartType()}
          showTooltip={true}
          filled={props.filled !== undefined ? props.filled : (chartType() === 'mini' || chartType() === 'storage')}
          width={containerWidth()}
          height={getHeight()}
          forceGray={props.forceGray}
        />
      </Show>
    </div>
  );
};