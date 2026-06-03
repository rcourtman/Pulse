import { Component, Show, createMemo, JSX } from 'solid-js';
import { AnimatedNumber } from '@/components/shared/AnimatedNumber';
import { MetricBar } from '@/components/Workloads/MetricBar';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { formatPercent } from '@/utils/format';
import { getMetricSeverity } from '@/utils/metricThresholds';
import type { MetricDisplayThresholds, MetricSeverity } from '@/utils/metricThresholds';

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

  /** Resolved warning/critical thresholds for alert-aligned coloring. */
  thresholds?: MetricDisplayThresholds | null;
}

/** Map metric severity to text color + weight for compact metric display */
const METRIC_TEXT_STYLES: Record<MetricSeverity, string> = {
  critical: 'text-red-600 dark:text-red-400 font-bold',
  warning: 'text-orange-600 dark:text-orange-400 font-medium',
  normal: 'text-muted',
};

function metricTextClass(
  value: number,
  type: 'cpu' | 'memory' | 'disk',
  thresholds?: MetricDisplayThresholds | null,
): string {
  return METRIC_TEXT_STYLES[getMetricSeverity(value, type, thresholds)];
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
  const colorClass = createMemo(() => metricTextClass(props.value, props.type, props.thresholds));
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
      <span class="text-xs text-muted" aria-hidden="true">—</span>
    </div>
  );

  return (
    <Show when={isRunning()} fallback={props.fallback ?? defaultFallback}>
      <div class={props.class}>
        {/* Mobile: Colored percentage text */}
        <Show when={showMobileText()}>
          <div
            class={`md:hidden text-xs text-center ${colorClass()} whitespace-nowrap overflow-hidden text-ellipsis`}
          >
            <Show when={!props.label} fallback={displayLabel()}>
              <AnimatedNumber value={props.value} format={formatPercent} />
            </Show>
          </div>
        </Show>

        {/* Desktop: Full MetricBar with sparkline support */}
        <div class={showMetricBar() ? '' : 'hidden md:block'}>
          <MetricBar
            value={props.value}
            label={displayLabel()}
            animatedLabelValue={props.label ? undefined : props.value}
            sublabel={resolvedSublabel()}
            showLabel={showLabel()}
            type={props.type}
            resourceId={props.resourceId}
            thresholds={props.thresholds}
          />
        </div>
      </div>
    </Show>
  );
};
