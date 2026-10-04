import type { MemoryObservation } from '@/types/api';

// Keep only the existing server-owned wire fields. Missing/future state is not
// replaced by a resource poll time or a platform facet from another metric.
export const readMemoryObservation = (value: unknown): MemoryObservation | undefined => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const record = value as Record<string, unknown>;
  return {
    state: typeof record.state === 'string' ? record.state : '',
    source: typeof record.source === 'string' ? record.source : '',
    ...(typeof record.observedAt === 'string' ? { observedAt: record.observedAt } : {}),
  };
};
