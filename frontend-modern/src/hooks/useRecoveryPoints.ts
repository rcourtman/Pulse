import { Accessor, createMemo } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import type {
  RecoveryPoint,
  RecoveryPointsResponse,
  RecoveryPointsTransportResponse,
} from '@/types/recovery';
import { normalizeRecoveryPointsResponse } from '@/utils/recoveryPlatformModel';
import { createNonSuspendingQuery } from '@/features/recovery/createNonSuspendingQuery';

const RECOVERY_POINTS_URL = '/api/recovery/points';
const DEFAULT_LIMIT = 200;
const REFRESH_MS = 30_000;

export type RecoveryPointsQuery = {
  // Paging
  page?: number | null;
  limit?: number | null;

  // Primary filters (server-side)
  rollupId?: string | null;
  platform?: string | null;
  kind?: string | null;
  mode?: string | null;
  outcome?: string | null;
  itemType?: string | null;
  itemResourceId?: string | null;
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
    platform: norm(q.platform) || null,
    kind: norm(q.kind) || null,
    mode: norm(q.mode) || null,
    outcome: norm(q.outcome) || null,
    itemType: norm(q.itemType) || null,
    itemResourceId: norm(q.itemResourceId) || norm(q.subjectResourceId) || null,
    subjectResourceId: null,

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
  if (q.platform) params.set('platform', q.platform);
  if (q.kind) params.set('kind', q.kind);
  if (q.mode) params.set('mode', q.mode);
  if (q.outcome) params.set('outcome', q.outcome);
  if (q.itemType) params.set('itemType', q.itemType);
  if (q.itemResourceId) params.set('itemResourceId', q.itemResourceId);

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
  const response = await apiFetchJSON<RecoveryPointsTransportResponse>(url);
  return normalizeRecoveryPointsResponse(response);
}

export function useRecoveryPoints(query?: Accessor<RecoveryPointsQuery | null | undefined>) {
  const source = createMemo<string | null>(() => {
    if (!query) return '__all__';
    const q = query();
    if (!q) return null;
    return serializeQuery(q);
  });

  const state = createNonSuspendingQuery<RecoveryPointsResponse, string>({
    source,
    fetcher: async (key) => fetchRecoveryPointsResponse(parseSerializedQuery(key)),
    initialValue: {
      data: [],
      meta: { page: 1, limit: DEFAULT_LIMIT, total: 0, totalPages: 1 },
    },
    pollMs: REFRESH_MS,
  });

  const points = createMemo<RecoveryPoint[]>(() => state.value().data || []);
  const meta = createMemo(
    () => state.value().meta || { page: 1, limit: DEFAULT_LIMIT, total: 0, totalPages: 1 },
  );

  const response = {
    get error() {
      return state.error();
    },
    get loading() {
      return state.loading();
    },
  };

  return {
    response,
    points,
    meta,
    refetch: state.refetch,
  };
}
