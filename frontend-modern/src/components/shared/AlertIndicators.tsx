import { Component } from 'solid-js';
import type { Alert } from '@/types/api';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';

const getMetadataUnit = (alert: Alert): string | undefined => {
  const rawUnit = alert.metadata?.['unit'];
  if (typeof rawUnit === 'string') {
    const trimmed = rawUnit.trim();
    if (trimmed.length > 0) {
      return trimmed;
    }
  }
  return undefined;
};

const formatAlertValue = (alert: Alert): string => {
  const metric = alert.type.toLowerCase();
  const unitFromMetadata = getMetadataUnit(alert);

  switch (metric) {
    case 'temperature':
      return `${alert.value.toFixed(1)}°C`;
    case 'diskread':
    case 'diskwrite':
    case 'networkin':
    case 'networkout':
      return `${alert.value.toFixed(1)} MB/s`;
    case 'cpu':
    case 'memory':
    case 'disk':
    case 'usage':
      return `${alert.value.toFixed(1)}%`;
    default:
      if (unitFromMetadata) {
        return `${alert.value.toFixed(1)} ${unitFromMetadata}`;
      }
      return alert.value.toFixed(1);
  }
};

const formatAlertThreshold = (alert: Alert): string => {
  const metric = alert.type.toLowerCase();
  const unitFromMetadata = getMetadataUnit(alert);

  switch (metric) {
    case 'temperature':
      return `${alert.threshold.toFixed(0)}°C`;
    case 'diskread':
    case 'diskwrite':
    case 'networkin':
    case 'networkout':
      return `${alert.threshold.toFixed(0)} MB/s`;
    case 'cpu':
    case 'memory':
    case 'disk':
    case 'usage':
      return `${alert.threshold.toFixed(0)}%`;
    default:
      if (unitFromMetadata) {
        return `${alert.threshold.toFixed(0)} ${unitFromMetadata}`;
      }
      return alert.threshold.toFixed(0);
  }
};

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
      .map(
        (alert) => `${alert.type}: ${formatAlertValue(alert)} (threshold: ${formatAlertThreshold(alert)})`,
      )
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
      .map(
        (alert, index) =>
          `${index + 1}. ${alert.type}: ${formatAlertValue(alert)} (threshold: ${formatAlertThreshold(alert)})`,
      )
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
