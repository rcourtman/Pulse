import { Component, Show, createMemo, JSX } from 'solid-js';
import { MetricBar } from '@/components/Dashboard/MetricBar';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { formatPercent } from '@/utils/format';
import { getMetricSeverity } from '@/utils/metricThresholds';
import type { MetricSeverity } from '@/utils/metricThresholds';

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

/** Map metric severity to text color + weight for compact metric display */
const METRIC_TEXT_STYLES: Record<MetricSeverity, string> = {
  critical: 'text-red-600 dark:text-red-400 font-bold',
  warning:  'text-orange-600 dark:text-orange-400 font-medium',
  normal:   'text-slate-600 dark:text-slate-400',
};

function metricTextClass(value: number, type: 'cpu' | 'memory' | 'disk'): string {
  return METRIC_TEXT_STYLES[getMetricSeverity(value, type)];
}

function compactCapacityLabel(sublabel?: string): string | undefined {
  if (!sublabel) return undefined;

  const raw = sublabel.trim();
  const parts = raw.split('/');
  if (parts.length < 2) return raw;

  const leftRaw = parts[0]?.trim();
  const rightRaw = parts.slice(1).join('/').trim();
  if (!leftRaw || !rightRaw) return raw;

  const rightUnitMatch = rightRaw.match(/[A-Za-z]+$/);
  const leftUnitMatch = leftRaw.match(/[A-Za-z]+$/);
  const rightUnit = rightUnitMatch?.[0];
  const leftUnit = leftUnitMatch?.[0];

  let normalizedLeft = leftRaw;
  if (rightUnit && leftUnit && rightUnit === leftUnit) {
    normalizedLeft = leftRaw.slice(0, Math.max(0, leftRaw.length - rightUnit.length)).trim();
  }

  const compactLeft = normalizedLeft.replace(/\s+/g, '');
  const compactRight = rightRaw.replace(/\s+/g, '');

  if (!compactLeft || !compactRight) return raw;
  return `${compactLeft}/${compactRight}`;
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
  const { isAtLeast, isBelow } = useBreakpoint();
  const displayLabel = createMemo(() => props.label ?? formatPercent(props.value));
  const colorClass = createMemo(() => metricTextClass(props.value, props.type));
  const isRunning = () => props.isRunning !== false; // Default to true if not specified

  const isVeryNarrow = createMemo(() => isBelow('xs'));
  const isMedium = createMemo(() => isAtLeast('md') && isBelow('lg'));
  const isWide = createMemo(() => isAtLeast('lg'));

  const compactSublabel = createMemo(() => compactCapacityLabel(props.sublabel));
  const resolvedSublabel = createMemo(() => {
    if (isWide()) return props.sublabel;
    if (isMedium()) return compactSublabel();
    return undefined;
  });
  const showLabel = createMemo(() => true);
  const showMobileText = createMemo(() => Boolean(props.showMobile) && !isVeryNarrow());
  const showMetricBar = createMemo(() => !props.showMobile || isVeryNarrow());

  const defaultFallback = (
    <div class="h-4 flex items-center justify-center">
      <span class="text-xs text-slate-400 dark:text-slate-500">—</span>
    </div>
  );

  return (
    <Show when={isRunning()} fallback={props.fallback ?? defaultFallback}>
      <div class={props.class}>
        {/* Mobile: Colored percentage text */}
        <Show when={showMobileText()}>
          <div class={`md:hidden text-xs text-center ${colorClass()} whitespace-nowrap overflow-hidden text-ellipsis`}>
            {displayLabel()}
          </div>
        </Show>

        {/* Desktop: Full MetricBar with sparkline support */}
        <div class={showMetricBar() ? '' : 'hidden md:block'}>
          <MetricBar
            value={props.value}
            label={displayLabel()}
            sublabel={resolvedSublabel()}
            showLabel={showLabel()}
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
  const colorClass = createMemo(() => metricTextClass(props.value, props.type));

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
  const colorClass = createMemo(() => metricTextClass(props.value, props.type));
  const isRunning = () => props.isRunning !== false;

  const defaultFallback = (
    <div class="h-4 flex items-center justify-center">
      <span class="text-xs text-slate-400 dark:text-slate-500">—</span>
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
