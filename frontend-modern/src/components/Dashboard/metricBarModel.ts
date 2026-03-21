import { estimateTextWidth } from '@/utils/format';
import { getMetricColorClass } from '@/utils/metricThresholds';
import type { MetricType } from '@/utils/metricThresholds';

export interface MetricBarProps {
  value: number;
  label: string;
  sublabel?: string;
  showLabel?: boolean;
  type?: 'cpu' | 'memory' | 'disk' | 'generic';
  resourceId?: string;
  class?: string;
}

export interface MetricBarPresentation {
  progressColorClass: string;
  showLabel: boolean;
  showSublabel: boolean;
  width: number;
}

export function buildMetricBarPresentation(
  props: MetricBarProps,
  containerWidth: number,
): MetricBarPresentation {
  const width = Math.min(props.value, 100);
  const showLabel = props.showLabel !== false && props.label.trim().length > 0;
  const showSublabel =
    showLabel &&
    Boolean(props.sublabel) &&
    containerWidth >= estimateTextWidth(`${props.label} (${props.sublabel})`);
  const metric = props.type || 'cpu';
  const metricType: MetricType = metric === 'generic' ? 'cpu' : metric;

  return {
    progressColorClass: getMetricColorClass(props.value, metricType),
    showLabel,
    showSublabel,
    width,
  };
}

