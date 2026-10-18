import { Component, createMemo } from 'solid-js';
import {
  formatTemperature,
  getTemperatureTextClass,
  type TemperatureDisplayMetric,
} from '@/utils/temperature';
import type { MetricDisplayThresholds } from '@/utils/metricThresholds';

interface TemperatureGaugeProps {
  value: number;
  min?: number | null;
  max?: number | null;
  critical?: number;
  warning?: number;
  thresholds?: MetricDisplayThresholds | null;
  metric?: TemperatureDisplayMetric;
  /** Severity of an open alert on this reading; keeps its tone while the alert holds. */
  alertSeverity?: 'warning' | 'critical' | null;
  title?: string;
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
    getTemperatureTextClass(props.value, explicitThresholds(), props.metric, props.alertSeverity),
  );

  return (
    <span
      class={`text-xs whitespace-nowrap ${textColorClass()} ${props.class || ''}`}
      title={props.title}
    >
      {formatTemperature(props.value)}
    </span>
  );
};
