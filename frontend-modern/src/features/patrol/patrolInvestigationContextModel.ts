import type { CorrelationsResponse, IntelligencePolicyPostureSummary } from '@/types/aiIntelligence';

export interface PatrolInvestigationContextSummaryInput {
  recentChangesCount?: number | null;
  correlations?: CorrelationsResponse | null;
  policyPosture?: IntelligencePolicyPostureSummary | null;
}

export interface PatrolInvestigationContextSummary {
  recentChangeCount: number;
  correlationCount: number;
  governedResourceCount: number;
  hasContext: boolean;
  summaryText: string;
}

export function buildPatrolInvestigationContextSummary(
  input: PatrolInvestigationContextSummaryInput,
): PatrolInvestigationContextSummary {
  const recentChangeCount = normalizeNonNegativeCount(input.recentChangesCount);
  const correlationCount = normalizeCorrelationCount(input.correlations);
  const governedResourceCount = normalizeNonNegativeCount(input.policyPosture?.total_resources);

  const parts: string[] = [];
  if (recentChangeCount > 0) {
    parts.push(`${recentChangeCount} recent change${recentChangeCount === 1 ? '' : 's'}`);
  }
  if (correlationCount > 0) {
    parts.push(`${correlationCount} correlation${correlationCount === 1 ? '' : 's'}`);
  }
  if (governedResourceCount > 0) {
    parts.push(
      `${governedResourceCount} policy-covered resource${governedResourceCount === 1 ? '' : 's'}`,
    );
  }

  return {
    recentChangeCount,
    correlationCount,
    governedResourceCount,
    hasContext: parts.length > 0,
    summaryText: parts.join(' · '),
  };
}

function normalizeCorrelationCount(correlations?: CorrelationsResponse | null): number {
  if (!correlations) return 0;
  if (typeof correlations.count === 'number' && Number.isFinite(correlations.count)) {
    return Math.max(0, Math.trunc(correlations.count));
  }
  if (Array.isArray(correlations.correlations)) {
    return correlations.correlations.length;
  }
  return 0;
}

function normalizeNonNegativeCount(value?: number | null): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.trunc(value));
}
