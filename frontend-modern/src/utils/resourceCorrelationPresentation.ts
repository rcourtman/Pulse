import type { ResourceCorrelation } from '@/types/aiIntelligence';
import { formatDurationMs } from '@/utils/patrolFormat';

const humanizeCorrelationToken = (value?: string): string => {
  const normalized = (value || '').trim();
  if (!normalized) return 'Correlation';
  return normalized
    .replace(/_/g, ' ')
    .replace(/\s*->\s*/g, ' → ')
    .replace(/\b\w/g, (char) => char.toUpperCase());
};

const formatResourceCorrelationEndpointLabel = (value?: string): string => {
  const normalized = (value || '').trim();
  if (!normalized) return 'Unknown resource';
  return normalized;
};

export function formatResourceCorrelationEndpoint(
  correlation: ResourceCorrelation,
  role: 'source' | 'target',
): string {
  return formatResourceCorrelationEndpointLabel(
    role === 'source'
      ? correlation.source_name || correlation.source_id
      : correlation.target_name || correlation.target_id,
  );
}

export function formatResourceCorrelationHeadline(correlation: ResourceCorrelation): string {
  return `${formatResourceCorrelationEndpoint(correlation, 'source')} → ${formatResourceCorrelationEndpoint(correlation, 'target')}`;
}

export function formatResourceCorrelationPattern(correlation: ResourceCorrelation): string {
  return humanizeCorrelationToken(correlation.event_pattern);
}

export function formatResourceCorrelationSummary(correlation: ResourceCorrelation): string {
  const parts: string[] = [];

  if (correlation.occurrences > 0) {
    parts.push(`${correlation.occurrences} occurrence${correlation.occurrences === 1 ? '' : 's'}`);
  }

  const avgDelayMs = Math.round((correlation.avg_delay || 0) / 1_000_000);
  const formattedDelay = formatDurationMs(avgDelayMs);
  if (formattedDelay) {
    parts.push(`avg delay ${formattedDelay}`);
  }

  if (typeof correlation.confidence === 'number' && Number.isFinite(correlation.confidence)) {
    parts.push(`${Math.round(correlation.confidence * 100)}% confidence`);
  }

  return parts.join(' · ');
}
