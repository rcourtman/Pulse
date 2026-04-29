import type { ResourceCorrelation } from '@/types/aiIntelligence';
import type { ResourceRelationship } from '@/types/resource';
import { formatDurationMs } from '@/utils/patrolFormat';
import { formatConfidencePercentage } from '@/utils/confidencePresentation';
import { asTrimmedString } from '@/utils/stringUtils';
import { humanizeArrowDelimitedLabel } from '@/utils/textPresentation';

const parseGoDurationMs = (value: string): number | null => {
  const normalized = value.trim();
  if (!normalized) return null;

  let totalNs = 0;
  let matched = false;
  const pattern = /(\d+(?:\.\d+)?)(h|m|s|ms|us|µs|ns)/g;
  for (const match of normalized.matchAll(pattern)) {
    matched = true;
    const amount = Number.parseFloat(match[1]);
    if (!Number.isFinite(amount)) continue;
    const unit = match[2];
    if (unit === 'h') totalNs += amount * 60 * 60 * 1_000_000_000;
    else if (unit === 'm') totalNs += amount * 60 * 1_000_000_000;
    else if (unit === 's') totalNs += amount * 1_000_000_000;
    else if (unit === 'ms') totalNs += amount * 1_000_000;
    else if (unit === 'us' || unit === 'µs') totalNs += amount * 1_000;
    else if (unit === 'ns') totalNs += amount;
  }

  if (!matched || !Number.isFinite(totalNs) || totalNs <= 0) return null;
  return Math.round(totalNs / 1_000_000);
};

const humanizeCorrelationToken = (value?: string): string => {
  return humanizeArrowDelimitedLabel(value, { fallback: 'Correlation' });
};

export function formatResourceCorrelationEndpoint(
  correlation: ResourceCorrelation,
  role: 'source' | 'target',
): string {
  return (
    asTrimmedString(
      role === 'source'
        ? correlation.source_name || correlation.source_id
        : correlation.target_name || correlation.target_id,
    ) || 'Unknown resource'
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

  const avgDelayMs =
    typeof correlation.avg_delay === 'number'
      ? Math.round(correlation.avg_delay / 1_000_000)
      : parseGoDurationMs(correlation.avg_delay);
  const formattedDelay = avgDelayMs ? formatDurationMs(avgDelayMs) : '';
  if (formattedDelay) {
    parts.push(`avg delay ${formattedDelay}`);
  }

  if (typeof correlation.confidence === 'number' && Number.isFinite(correlation.confidence)) {
    parts.push(`${formatConfidencePercentage(correlation.confidence)} confidence`);
  }

  return parts.join(' · ');
}

export function sortResourceCorrelations(
  correlations: readonly ResourceCorrelation[],
): ResourceCorrelation[] {
  return [...correlations].sort((left, right) => {
    const confidenceDiff = (right.confidence || 0) - (left.confidence || 0);
    if (confidenceDiff !== 0) return confidenceDiff;

    const leftTime = Date.parse(left.last_seen || '');
    const rightTime = Date.parse(right.last_seen || '');
    return (
      (Number.isFinite(rightTime) ? rightTime : 0) - (Number.isFinite(leftTime) ? leftTime : 0)
    );
  });
}

export function formatResourceRelationshipType(relationship: ResourceRelationship): string {
  return humanizeCorrelationToken(relationship.type);
}

export function formatResourceRelationshipEndpoint(
  relationship: ResourceRelationship,
  role: 'source' | 'target',
): string {
  return (
    asTrimmedString(role === 'source' ? relationship.sourceId : relationship.targetId) ||
    'Unknown resource'
  );
}

export function formatResourceRelationshipSummary(relationship: ResourceRelationship): string {
  const parts: string[] = [];

  if (typeof relationship.confidence === 'number' && Number.isFinite(relationship.confidence)) {
    parts.push(`${formatConfidencePercentage(relationship.confidence)} confidence`);
  }

  const discoverer = humanizeCorrelationToken(relationship.discoverer);
  if (discoverer && discoverer !== 'Correlation') {
    parts.push(discoverer);
  }

  if (relationship.active === false) {
    parts.push('Historical');
  }

  return parts.join(' · ');
}

export function sortResourceRelationships(
  relationships: readonly ResourceRelationship[],
): ResourceRelationship[] {
  return [...relationships].sort((left, right) => {
    if (left.active !== right.active) return left.active ? -1 : 1;

    const confidenceDiff = (right.confidence || 0) - (left.confidence || 0);
    if (confidenceDiff !== 0) return confidenceDiff;

    const leftTime = Date.parse(left.lastSeenAt || left.observedAt || '');
    const rightTime = Date.parse(right.lastSeenAt || right.observedAt || '');
    return (
      (Number.isFinite(rightTime) ? rightTime : 0) - (Number.isFinite(leftTime) ? leftTime : 0)
    );
  });
}

const formatPluralCount = (count: number, singular: string, plural: string): string =>
  `${count} ${count === 1 ? singular : plural}`;

const formatSummaryParts = (parts: Array<string | null | undefined>): string =>
  parts.filter((part): part is string => Boolean(part && part.trim())).join(' · ');

export function formatResourceCorrelationSummaryText(options: {
  relationshipsCount?: number;
  dependenciesCount: number;
  dependentsCount: number;
  correlationsCount: number;
  summaryText?: string | null;
}): string {
  return (
    options.summaryText?.trim() ||
    formatSummaryParts([
      options.relationshipsCount && options.relationshipsCount > 0
        ? formatPluralCount(
            options.relationshipsCount,
            'canonical relationship',
            'canonical relationships',
          )
        : null,
      options.dependenciesCount > 0
        ? formatPluralCount(options.dependenciesCount, 'dependency', 'dependencies')
        : null,
      options.dependentsCount > 0
        ? formatPluralCount(options.dependentsCount, 'dependent', 'dependents')
        : null,
      options.correlationsCount > 0
        ? formatPluralCount(options.correlationsCount, 'correlation', 'correlations')
        : null,
    ])
  );
}
