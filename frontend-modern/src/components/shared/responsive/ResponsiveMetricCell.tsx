import { Component, Show, createMemo, JSX } from 'solid-js';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { formatPercent } from '@/utils/format';

export interface ResponsiveMetricCellProps {
  /** Metric value (0-100 percentage) */
  value: number;

  /** Metric type for theming/thresholds */
  type: 'cpu' | 'memory' | 'disk';

  /** Primary label (defaults to formatted percentage) */
  label?: string;

  /** Secondary label (e.g., "4.2GB / 8GB") */
  sublabel?: string;

  /** Resource ID for sparkline tracking */
  resourceId?: string;

  /** Whether the resource is running/online - if false, shows fallback */
  isRunning?: boolean;

  /** Show mobile (compact text) view */
  showMobile?: boolean;

  /** Custom fallback when not running (defaults to "—") */
  fallback?: JSX.Element;

  /** Additional CSS classes for container */
  class?: string;
}

/**
 * Get the appropriate text color class based on metric value and type
 */
function getMetricColorClass(value: number, type: 'cpu' | 'memory' | 'disk'): string {
  // Thresholds match MetricBar component
  if (type === 'cpu') {
    if (value >= 90) return 'text-red-600 dark:text-red-400 font-bold';
    if (value >= 80) return 'text-orange-600 dark:text-orange-400 font-medium';
    return 'text-gray-600 dark:text-gray-400';
  }

  if (type === 'memory') {
    if (value >= 85) return 'text-red-600 dark:text-red-400 font-bold';
    if (value >= 75) return 'text-orange-600 dark:text-orange-400 font-medium';
    return 'text-gray-600 dark:text-gray-400';
  }

  if (type === 'disk') {
    if (value >= 90) return 'text-red-600 dark:text-red-400 font-bold';
    if (value >= 80) return 'text-orange-600 dark:text-orange-400 font-medium';
    return 'text-gray-600 dark:text-gray-400';
  }

  return 'text-gray-600 dark:text-gray-400';
}

/**
 * A responsive metric cell that shows a simple colored percentage on mobile
 * and a full MetricBar (with progress bar or sparkline) on desktop.
 *
 * @example
 * ```tsx
 * <ResponsiveMetricCell
 *   value={cpuPercent}
 *   type="cpu"
 *   resourceId={metricsKey}
 *   isRunning={isOnline}
 *   showMobile={isMobile()}
 * />
 * ```
 */
export const ResponsiveMetricCell: Component<ResponsiveMetricCellProps> = (props) => {
  const displayLabel = createMemo(() => props.label ?? formatPercent(props.value));
  const colorClass = createMemo(() => getMetricColorClass(props.value, props.type));
  const isRunning = () => props.isRunning !== false; // Default to true if not specified

  const defaultFallback = (
    <div class="h-4 flex items-center justify-center">
      <span class="text-xs text-gray-400 dark:text-gray-500">—</span>
    </div>
  );

  return (
    <Show when={isRunning()} fallback={props.fallback ?? defaultFallback}>
      <div class={props.class}>
        {/* Mobile: Colored percentage text */}
        <Show when={props.showMobile}>
          <div class={`md:hidden text-xs text-center ${colorClass()}`}>
            {displayLabel()}
          </div>
        </Show>

        {/* Desktop: Full MetricBar with sparkline support */}
        <div class={props.showMobile ? 'hidden md:block' : ''}>
          <MetricBar
            value={props.value}
            label={displayLabel()}
            sublabel={props.sublabel}
            type={props.type}
            resourceId={props.resourceId}
          />
        </div>
      </div>
    </Show>
  );
};

/**
 * Simpler metric text component for when you just want colored percentage
 * without the MetricBar complexity
 */
export const MetricText: Component<{
  value: number;
  type: 'cpu' | 'memory' | 'disk';
  label?: string;
  class?: string;
}> = (props) => {
  const displayLabel = createMemo(() => props.label ?? formatPercent(props.value));
  const colorClass = createMemo(() => getMetricColorClass(props.value, props.type));

  return (
    <span class={`text-xs text-center ${colorClass()} ${props.class || ''}`}>
      {displayLabel()}
    </span>
  );
};

/**
 * Metric cell with explicit mobile/desktop rendering
 * Use this when you need full control over what renders in each mode
 */
export const DualMetricCell: Component<{
  value: number;
  type: 'cpu' | 'memory' | 'disk';
  label?: string;
  sublabel?: string;
  resourceId?: string;
  isRunning?: boolean;
  showMobile: boolean;
  mobileContent?: JSX.Element;
  desktopContent?: JSX.Element;
  fallback?: JSX.Element;
  class?: string;
}> = (props) => {
  const displayLabel = createMemo(() => props.label ?? formatPercent(props.value));
  const colorClass = createMemo(() => getMetricColorClass(props.value, props.type));
  const isRunning = () => props.isRunning !== false;

  const defaultFallback = (
    <div class="h-4 flex items-center justify-center">
      <span class="text-xs text-gray-400 dark:text-gray-500">—</span>
    </div>
  );

  const defaultMobileContent = (
    <div class={`text-xs text-center ${colorClass()}`}>
      {displayLabel()}
    </div>
  );

  const defaultDesktopContent = (
    <MetricBar
      value={props.value}
      label={displayLabel()}
      sublabel={props.sublabel}
      type={props.type}
      resourceId={props.resourceId}
    />
  );

  return (
    <Show when={isRunning()} fallback={props.fallback ?? defaultFallback}>
      <div class={props.class}>
        <Show when={props.showMobile} fallback={props.desktopContent ?? defaultDesktopContent}>
          {props.mobileContent ?? defaultMobileContent}
        </Show>
      </div>
    </Show>
  );
};
