import { Component } from 'solid-js';
import type { Alert } from '@/types/api';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';

interface AlertIndicatorProps {
  severity: 'critical' | 'warning' | null;
  alerts?: Alert[];
}

export const AlertIndicator: Component<AlertIndicatorProps> = (props) => {
  if (!props.severity) return null;

  const dotClass = props.severity === 'critical' ? 'bg-red-500 animate-pulse' : 'bg-orange-500';

  const handleMouseEnter = (e: MouseEvent) => {
    if (!props.alerts || props.alerts.length === 0) return;
    const rect = (e.target as HTMLElement).getBoundingClientRect();
    const content = props.alerts
      .map((alert) => `${alert.type}: ${alert.value.toFixed(1)}% (threshold: ${alert.threshold}%)`)
      .join('\n');
    showTooltip(content, rect.left + rect.width / 2, rect.top, {
      align: 'center',
      direction: 'up',
      maxWidth: 260,
    });
  };

  const handleMouseLeave = () => {
    hideTooltip();
  };

  return (
    <>
      <span
        class={`inline-block w-2 h-2 rounded-full ${dotClass}`}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      />
    </>
  );
};

interface AlertCountBadgeProps {
  count: number;
  severity: 'critical' | 'warning';
  alerts?: Alert[];
}

export const AlertCountBadge: Component<AlertCountBadgeProps> = (props) => {
  const badgeClass =
    props.severity === 'critical' ? 'bg-red-500 text-white' : 'bg-orange-500 text-white';

  const handleMouseEnter = (e: MouseEvent) => {
    if (!props.alerts || props.alerts.length === 0) return;
    const rect = (e.target as HTMLElement).getBoundingClientRect();
    const header = `${props.count} Active Alert${props.count === 1 ? '' : 's'}:`;
    const details = props.alerts
      .map((alert, index) => `${index + 1}. ${alert.type}: ${alert.value.toFixed(1)}% (threshold: ${alert.threshold}%)`)
      .join('\n');
    const content = [header, details].filter(Boolean).join('\n');
    showTooltip(content, rect.left + rect.width / 2, rect.top, {
      align: 'center',
      direction: 'up',
      maxWidth: 300,
    });
  };

  const handleMouseLeave = () => {
    hideTooltip();
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
    </>
  );
};
