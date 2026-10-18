import type { Resource } from '@/types/resource';

export interface ResourceStaleness {
  // Backend-formatted age of the last report, e.g. "9m" or "1h"; null when
  // the backend did not say.
  age: string | null;
  label: string;
}

// The backend owns staleness: a resource whose sources stopped reporting
// carries a `telemetry_stale` health reason with the age, either as the reason
// for a `stale` verdict or behind an alert that outranks it. Tables use this
// to stop presenting the last received metrics as live.
export const getResourceStaleness = (
  resource: Pick<Resource, 'health'> | undefined | null,
): ResourceStaleness | null => {
  const health = resource?.health;
  if (!health) return null;
  const reason = health.reasons?.find((entry) => entry.code === 'telemetry_stale');
  if (!reason && health.verdict !== 'stale') return null;
  const age = reason?.detail?.trim() || null;
  return { age, label: age ? `No report for ${age}` : 'No recent report' };
};
