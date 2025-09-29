import { Component, Show, createSignal } from 'solid-js';
import type { Alert } from '@/types/api';
import { Portal } from 'solid-js/web';

interface AlertIndicatorProps {
  severity: 'critical' | 'warning' | null;
  alerts?: Alert[];
}

export const AlertIndicator: Component<AlertIndicatorProps> = (props) => {
  if (!props.severity) return null;

  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPosition, setTooltipPosition] = createSignal({ x: 0, y: 0 });

  const dotClass = props.severity === 'critical' ? 'bg-red-500 animate-pulse' : 'bg-orange-500';

  const handleMouseEnter = (e: MouseEvent) => {
    if (!props.alerts || props.alerts.length === 0) return;
    const rect = (e.target as HTMLElement).getBoundingClientRect();
    setTooltipPosition({ x: rect.left + rect.width / 2, y: rect.top - 5 });
    setShowTooltip(true);
  };

  const handleMouseLeave = () => {
    setShowTooltip(false);
  };

  return (
    <>
      <span
        class={`inline-block w-2 h-2 rounded-full ${dotClass}`}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      />
      <Show when={showTooltip() && props.alerts && props.alerts.length > 0}>
        <Portal>
          <div
            class="fixed z-50 bg-gray-900 text-white text-xs rounded px-2 py-1 pointer-events-none transform -translate-x-1/2 -translate-y-full"
            style={{
              left: `${tooltipPosition().x}px`,
              top: `${tooltipPosition().y}px`,
            }}
          >
            {props.alerts!.map((alert, i) => (
              <div class={i > 0 ? 'mt-1' : ''}>
                {alert.type}: {alert.value.toFixed(1)}% (threshold: {alert.threshold}%)
              </div>
            ))}
            <div class="absolute top-full left-1/2 transform -translate-x-1/2 w-0 h-0 border-l-4 border-r-4 border-t-4 border-transparent border-t-gray-900" />
          </div>
        </Portal>
      </Show>
    </>
  );
};

interface AlertCountBadgeProps {
  count: number;
  severity: 'critical' | 'warning';
  alerts?: Alert[];
}

export const AlertCountBadge: Component<AlertCountBadgeProps> = (props) => {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPosition, setTooltipPosition] = createSignal({ x: 0, y: 0 });

  const badgeClass =
    props.severity === 'critical' ? 'bg-red-500 text-white' : 'bg-orange-500 text-white';

  const handleMouseEnter = (e: MouseEvent) => {
    if (!props.alerts || props.alerts.length === 0) return;
    const rect = (e.target as HTMLElement).getBoundingClientRect();
    setTooltipPosition({ x: rect.left + rect.width / 2, y: rect.top - 5 });
    setShowTooltip(true);
  };

  const handleMouseLeave = () => {
    setShowTooltip(false);
  };

  return (
    <>
      <span
        class={`inline-flex items-center justify-center min-w-[20px] h-5 px-1 text-xs font-medium rounded-full ${badgeClass}`}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        {props.count}
      </span>
      <Show when={showTooltip() && props.alerts && props.alerts.length > 0}>
        <Portal>
          <div
            class="fixed z-50 bg-gray-900 text-white text-xs rounded px-2 py-1 pointer-events-none transform -translate-x-1/2 -translate-y-full max-w-xs"
            style={{
              left: `${tooltipPosition().x}px`,
              top: `${tooltipPosition().y}px`,
            }}
          >
            <div class="font-semibold mb-1">{props.count} Active Alerts:</div>
            {props.alerts!.map((alert, i) => (
              <div class={i > 0 ? 'mt-1' : ''}>
                {i + 1}. {alert.type}: {alert.value.toFixed(1)}% (threshold: {alert.threshold}%)
              </div>
            ))}
            <div class="absolute top-full left-1/2 transform -translate-x-1/2 w-0 h-0 border-l-4 border-r-4 border-t-4 border-transparent border-t-gray-900" />
          </div>
        </Portal>
      </Show>
    </>
  );
};
