import { createMemo } from 'solid-js';
import { ResourceAPI, type ResourceStatsResponse } from '@/api/resources';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import type { ResourcePolicyPostureSummary } from '@/types/resource';

const EMPTY_POLICY_POSTURE: ResourcePolicyPostureSummary = {
  totalResources: 0,
  sensitivityCounts: {},
  routingCounts: {},
  redactionCounts: {},
};

const EMPTY_RESOURCE_STATS: ResourceStatsResponse = {
  total: 0,
  byType: {},
  byStatus: {},
  bySource: {},
  policyPosture: EMPTY_POLICY_POSTURE,
};

/**
 * Read the compact server-owned resource aggregation without downloading and
 * adapting every resource row. The retained query keeps route remounts fast
 * while the normal organization-switch boundary invalidates stale posture.
 */
export function useResourceStats() {
  const state = createNonSuspendingQuery<ResourceStatsResponse, string>({
    source: () => 'resource-stats',
    fetcher: () => ResourceAPI.getStats(),
    initialValue: EMPTY_RESOURCE_STATS,
    cacheKey: () => 'resource-stats',
  });
  const policyPosture = createMemo(() => state.value().policyPosture ?? EMPTY_POLICY_POSTURE);

  return {
    error: state.error,
    loading: state.loading,
    policyPosture,
    refetch: state.refetch,
    resolvedOnce: state.resolvedOnce,
    stats: state.value,
  };
}
