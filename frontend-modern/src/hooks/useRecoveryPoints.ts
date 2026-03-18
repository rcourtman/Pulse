import { Accessor, createMemo, createResource, onCleanup } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import type { RecoveryPoint, RecoveryPointsResponse } from '@/types/recovery';

const RECOVERY_POINTS_URL = '/api/recovery/points';
const DEFAULT_LIMIT = 200;
const REFRESH_MS = 30_000;

export type RecoveryPointsQuery = {
  // Paging
  page?: number | null;
  limit?: number | null;

  // Primary filters (server-side)
  rollupId?: string | null;
  provider?: string | null;
  kind?: string | null;
  mode?: string | null;
  outcome?: string | null;
  subjectResourceId?: string | null;

  // Normalized filters (server-side)
  q?: string | null;
  cluster?: string | null;
  node?: string | null;
  namespace?: string | null;
  scope?: 'workload' | null;
  verification?: 'verified' | 'unverified' | 'unknown' | null;

  // Time window (server-side)
  from?: string | null; // RFC3339
  to?: string | null; // RFC3339
};

const normalizeQuery = (query: RecoveryPointsQuery | undefined): RecoveryPointsQuery => {
  const q = query || {};
  const norm = (value: string | null | undefined) => (value || '').trim();
  const page =
    typeof q.page === 'number' && Number.isFinite(q.page) ? Math.max(1, Math.floor(q.page)) : 1;
  const limit =
    typeof q.limit === 'number' && Number.isFinite(q.limit)
      ? Math.max(1, Math.floor(q.limit))
      : DEFAULT_LIMIT;

  return {
    page,
    limit,

    rollupId: norm(q.rollupId) || null,
    provider: norm(q.provider) || null,
    kind: norm(q.kind) || null,
    mode: norm(q.mode) || null,
    outcome: norm(q.outcome) || null,
    subjectResourceId: norm(q.subjectResourceId) || null,

    q: norm(q.q) || null,
    cluster: norm(q.cluster) || null,
    node: norm(q.node) || null,
    namespace: norm(q.namespace) || null,
    scope: q.scope || null,
    verification: q.verification || null,

    from: norm(q.from) || null,
    to: norm(q.to) || null,
  };
};

const serializeQuery = (query: RecoveryPointsQuery | undefined): string =>
  JSON.stringify(normalizeQuery(query));

const parseSerializedQuery = (value: string | null): RecoveryPointsQuery | undefined => {
  if (value == null) return undefined;
  if (value === '__all__') return undefined;
  try {
    return JSON.parse(value) as RecoveryPointsQuery;
  } catch {
    return undefined;
  }
};

const buildURL = (query: RecoveryPointsQuery | undefined): string => {
  const q = normalizeQuery(query);
  const params = new URLSearchParams();

  params.set('page', String(q.page || 1));
  params.set('limit', String(q.limit || DEFAULT_LIMIT));

  if (q.rollupId) params.set('rollupId', q.rollupId);
  if (q.provider) params.set('provider', q.provider);
  if (q.kind) params.set('kind', q.kind);
  if (q.mode) params.set('mode', q.mode);
  if (q.outcome) params.set('outcome', q.outcome);
  if (q.subjectResourceId) params.set('subjectResourceId', q.subjectResourceId);

  if (q.q) params.set('q', q.q);
  if (q.cluster) params.set('cluster', q.cluster);
  if (q.node) params.set('node', q.node);
  if (q.namespace) params.set('namespace', q.namespace);
  if (q.scope) params.set('scope', q.scope);
  if (q.verification) params.set('verification', q.verification);

  if (q.from) params.set('from', q.from);
  if (q.to) params.set('to', q.to);

  return `${RECOVERY_POINTS_URL}?${params.toString()}`;
};

async function fetchRecoveryPointsResponse(
  query: RecoveryPointsQuery | undefined,
): Promise<RecoveryPointsResponse> {
  const url = buildURL(query);
  return apiFetchJSON<RecoveryPointsResponse>(url);
}

export function useRecoveryPoints(query?: Accessor<RecoveryPointsQuery | null | undefined>) {
  const [response, { refetch }] = createResource<RecoveryPointsResponse, string | null>(
    () => {
      if (!query) return '__all__';
      const q = query();
      if (!q) return null;
      return serializeQuery(q);
    },
    async (key) => fetchRecoveryPointsResponse(parseSerializedQuery(key)),
  );

  const points = createMemo<RecoveryPoint[]>(() => response()?.data || []);
  const meta = createMemo(
    () => response()?.meta || { page: 1, limit: DEFAULT_LIMIT, total: 0, totalPages: 1 },
  );

  const interval = setInterval(() => void refetch(), REFRESH_MS);
  onCleanup(() => clearInterval(interval));

  return {
    response,
    points,
    meta,
    refetch,
  };
}
