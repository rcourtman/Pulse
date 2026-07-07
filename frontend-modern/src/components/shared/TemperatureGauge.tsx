import { Component, createMemo } from 'solid-js';
import { formatTemperature, getTemperatureTextClass } from '@/utils/temperature';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';

interface TemperatureGaugeProps {
  value: number;
  min?: number | null;
  max?: number | null;
  critical?: number;
  warning?: number;
  thresholds?: MetricDisplayThresholds | null;
  class?: string;
}

export const TemperatureGauge: Component<TemperatureGaugeProps> = (props) => {
  const explicitThresholds = createMemo<MetricDisplayThresholds | null | undefined>(() => {
    if (props.thresholds !== undefined) return props.thresholds;
    if (props.critical === undefined && props.warning === undefined) return undefined;

    const critical = props.critical ?? 80;
    return {
      critical,
      warning: props.warning ?? Math.max(0, critical - 5),
    };
  });

  const textColorClass = createMemo(() =>
    getTemperatureTextClass(props.value, explicitThresholds()),
  );

  return (
    <span class={`text-xs whitespace-nowrap ${textColorClass()} ${props.class || ''}`}>
      {formatTemperature(props.value)}
    </span>
  );
};
