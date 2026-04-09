import type {
  IntelligencePolicyPostureSummary,
  IntelligenceSummary,
} from '@/types/aiIntelligence';
import type {
  ResourceRedactionHint,
  ResourceRoutingScope,
  ResourceSensitivity,
} from '@/types/resource';

function normalizeNonNegativeCount(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) {
    return null;
  }

  return Math.trunc(value);
}

function normalizeCountMap<T extends string>(
  counts: Partial<Record<T, number>> | undefined,
): Partial<Record<T, number>> {
  const normalized: Partial<Record<T, number>> = {};

  for (const [key, value] of Object.entries(counts ?? {})) {
    const normalizedValue = normalizeNonNegativeCount(value);
    if (normalizedValue !== null) {
      normalized[key as T] = normalizedValue;
    }
  }

  return normalized;
}

function sumCountMap<T extends string>(counts: Partial<Record<T, number>>): number {
  return (Object.values(counts) as Array<number | undefined>).reduce<number>(
    (total, count) => total + (count ?? 0),
    0,
  );
}

function normalizePolicyPosture(
  posture: IntelligencePolicyPostureSummary | undefined,
): IntelligencePolicyPostureSummary | undefined {
  if (!posture) {
    return undefined;
  }

  const sensitivityCounts = normalizeCountMap<ResourceSensitivity>(posture.sensitivity_counts);
  const routingCounts = normalizeCountMap<ResourceRoutingScope>(posture.routing_counts);
  const redactionCounts = normalizeCountMap<ResourceRedactionHint>(posture.redaction_counts);
  const totalResources = Math.max(
    normalizeNonNegativeCount(posture.total_resources) ?? 0,
    sumCountMap(sensitivityCounts),
    sumCountMap(routingCounts),
  );

  return {
    total_resources: totalResources,
    sensitivity_counts: sensitivityCounts,
    routing_counts: routingCounts,
    ...(posture.redaction_counts !== undefined ? { redaction_counts: redactionCounts } : {}),
  };
}

export function normalizeIntelligenceSummary(summary: IntelligenceSummary): IntelligenceSummary {
  const recentChanges = summary.recent_changes ?? [];

  return {
    ...summary,
    recent_changes: recentChanges,
    recent_changes_count:
      normalizeNonNegativeCount(summary.recent_changes_count) ?? recentChanges.length,
    policy_posture: normalizePolicyPosture(summary.policy_posture),
  };
}
