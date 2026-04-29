import type { AnomalyReport } from '@/types/aiIntelligence';
import { ANOMALY_SEVERITY_CLASS, formatAnomalyRatio, formatPercent } from '@/utils/format';
import { getMetricColorClass, getMetricColorRgba } from '@/utils/metricThresholds';

export interface EnhancedCPUBarProps {
  usage: number;
  loadAverage?: number;
  cores?: number;
  model?: string;
  resourceId?: string;
  anomaly?: AnomalyReport | null;
}

export interface EnhancedCPUBarPresentation {
  anomalyClass: string;
  anomalyDescription?: string;
  anomalyRatio: string;
  barClass: string;
  barFill: string;
  barWidth: string;
  displayLoadAverage?: string;
  displayUsage: string;
  hasAnomaly: boolean;
  tooltipUsageClass: string;
}

export function buildEnhancedCPUBarPresentation(
  props: EnhancedCPUBarProps,
): EnhancedCPUBarPresentation {
  const anomalyRatio = formatAnomalyRatio(props.anomaly) ?? '';

  return {
    anomalyClass: props.anomaly
      ? (ANOMALY_SEVERITY_CLASS[props.anomaly.severity] ?? 'text-yellow-400')
      : 'text-yellow-400',
    anomalyDescription: props.anomaly?.description,
    anomalyRatio,
    barClass: getMetricColorClass(props.usage, 'cpu'),
    barFill: getMetricColorRgba(props.usage, 'cpu'),
    barWidth: `${Math.min(props.usage, 100)}%`,
    displayLoadAverage:
      props.loadAverage !== undefined ? props.loadAverage.toFixed(2) : undefined,
    displayUsage: formatPercent(props.usage),
    hasAnomaly: Boolean(props.anomaly && anomalyRatio),
    tooltipUsageClass: props.usage > 90 ? 'text-red-400' : 'text-base-content',
  };
}
