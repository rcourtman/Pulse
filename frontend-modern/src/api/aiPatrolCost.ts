import { apiFetchJSON } from '@/utils/apiClient';
import type { PatrolCostProjection, PatrolModelGuidanceResponse } from '@/types/ai';

export interface PatrolCostPreviewQuery {
  /** provider:model route to price; omit for the configured Patrol model. */
  model?: string;
  /** Schedule to price in minutes; 0 means manual only; omit for the configured one. */
  intervalMinutes?: number;
}

/**
 * Project what Patrol will cost per 30 days for a model and schedule. The
 * server owns the price table and the install's run history, so the
 * frontend never re-derives prices.
 */
export async function getPatrolCostPreview(
  query: PatrolCostPreviewQuery = {},
  signal?: AbortSignal,
): Promise<PatrolCostProjection> {
  const search = new URLSearchParams();
  const model = query.model?.trim();
  if (model) {
    search.set('model', model);
  }
  if (typeof query.intervalMinutes === 'number' && Number.isFinite(query.intervalMinutes)) {
    search.set('interval_minutes', String(Math.max(0, Math.round(query.intervalMinutes))));
  }
  const suffix = search.toString();
  return apiFetchJSON<PatrolCostProjection>(
    `/api/ai/patrol/cost-preview${suffix ? `?${suffix}` : ''}`,
    signal ? { signal } : undefined,
  );
}

/** Recommended, suggested, and caution markers for the model pickers. */
export async function getPatrolModelGuidance(): Promise<PatrolModelGuidanceResponse> {
  return apiFetchJSON<PatrolModelGuidanceResponse>('/api/ai/patrol/model-guidance');
}
