import { Accessor, createMemo, createResource, onCleanup } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import type { RecoveryPointsSeriesBucket, RecoveryPointsSeriesResponse } from '@/types/recovery';

const RECOVERY_SERIES_URL = '/api/recovery/series';
const REFRESH_MS = 30_000;

export type RecoverySeriesQuery = {
  rollupId?: string | null;
  provider?: string | null;
  kind?: string | null;
  mode?: string | null;
  outcome?: string | null;

  q?: string | null;
  cluster?: string | null;
  node?: string | null;
  namespace?: string | null;
  scope?: 'workload' | null;
  verification?: 'verified' | 'unverified' | 'unknown' | null;

  from?: string | null; // RFC3339
  to?: string | null; // RFC3339

  tzOffsetMinutes?: number | null; // UTC -> local offset (east positive)
};

const normalizeQuery = (query: RecoverySeriesQuery | undefined): RecoverySeriesQuery => {
  const q = query || {};
  const norm = (value: string | null | undefined) => (value || '').trim();
  const tzOffsetMinutes =
    typeof q.tzOffsetMinutes === 'number' && Number.isFinite(q.tzOffsetMinutes) ? Math.trunc(q.tzOffsetMinutes) : 0;
  return {
    rollupId: norm(q.rollupId) || null,
    provider: norm(q.provider) || null,
    kind: norm(q.kind) || null,
    mode: norm(q.mode) || null,
    outcome: norm(q.outcome) || null,

    q: norm(q.q) || null,
    cluster: norm(q.cluster) || null,
    node: norm(q.node) || null,
    namespace: norm(q.namespace) || null,
    scope: q.scope || null,
    verification: q.verification || null,

    from: norm(q.from) || null,
    to: norm(q.to) || null,

    tzOffsetMinutes,
  };
};

const serializeQuery = (query: RecoverySeriesQuery | undefined): string => JSON.stringify(normalizeQuery(query));

const parseSerializedQuery = (value: string | null): RecoverySeriesQuery | undefined => {
  if (value == null) return undefined;
  if (value === '__all__') return undefined;
  try {
    return JSON.parse(value) as RecoverySeriesQuery;
  } catch {
    return undefined;
  }
};

const buildURL = (query: RecoverySeriesQuery | undefined): string => {
  const q = normalizeQuery(query);
  const params = new URLSearchParams();

  if (q.rollupId) params.set('rollupId', q.rollupId);
  if (q.provider) params.set('provider', q.provider);
  if (q.kind) params.set('kind', q.kind);
  if (q.mode) params.set('mode', q.mode);
  if (q.outcome) params.set('outcome', q.outcome);

  if (q.q) params.set('q', q.q);
  if (q.cluster) params.set('cluster', q.cluster);
  if (q.node) params.set('node', q.node);
  if (q.namespace) params.set('namespace', q.namespace);
  if (q.scope) params.set('scope', q.scope);
  if (q.verification) params.set('verification', q.verification);

  if (q.from) params.set('from', q.from);
  if (q.to) params.set('to', q.to);

  params.set('tzOffsetMinutes', String(q.tzOffsetMinutes || 0));

  return `${RECOVERY_SERIES_URL}?${params.toString()}`;
};

async function fetchSeries(query: RecoverySeriesQuery | undefined): Promise<RecoveryPointsSeriesResponse> {
  const url = buildURL(query);
  return apiFetchJSON<RecoveryPointsSeriesResponse>(url);
}

export function useRecoveryPointsSeries(query?: Accessor<RecoverySeriesQuery | null | undefined>) {
  const [response, { refetch }] = createResource<RecoveryPointsSeriesResponse, string | null>(
    () => {
      if (!query) return '__all__';
      const q = query();
      if (!q) return null;
      return serializeQuery(q);
    },
    async (key) => fetchSeries(parseSerializedQuery(key)),
  );

  const series = createMemo<RecoveryPointsSeriesBucket[]>(() => response()?.data || []);

  const interval = setInterval(() => void refetch(), REFRESH_MS);
  onCleanup(() => clearInterval(interval));

  return {
    response,
    series,
    refetch,
  };
}
