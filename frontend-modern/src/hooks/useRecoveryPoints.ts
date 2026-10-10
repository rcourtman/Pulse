import { Accessor, createMemo } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import type {
  RecoveryPoint,
  RecoveryPointsResponse,
  RecoveryPointsTransportResponse,
} from '@/types/recovery';
import { normalizeRecoveryPointsResponse } from '@/utils/recoveryPlatformModel';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';

const RECOVERY_POINTS_URL = '/api/recovery/points';
const DEFAULT_LIMIT = 200;
const REFRESH_MS = 30_000;

// The TrueNAS Protection tab is the only reader and asks for one platform's
// first page. The points endpoint accepts more filters; add one here when a
// surface sends it.
export type RecoveryPointsQuery = {
  // Paging
  page?: number | null;
  limit?: number | null;

  // Filter (server-side)
  platform?: string | null;
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
    platform: norm(q.platform) || null,
  };
};

const serializeQuery = (query: RecoveryPointsQuery | undefined): string =>
  JSON.stringify(normalizeQuery(query));

const parseSerializedQuery = (value: string | null): RecoveryPointsQuery | undefined => {
  if (value == null) return undefined;
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

  if (q.platform) params.set('platform', q.platform);

  return `${RECOVERY_POINTS_URL}?${params.toString()}`;
};

async function fetchRecoveryPointsResponse(
  query: RecoveryPointsQuery | undefined,
): Promise<RecoveryPointsResponse> {
  const url = buildURL(query);
  const response = await apiFetchJSON<RecoveryPointsTransportResponse>(url);
  return normalizeRecoveryPointsResponse(response);
}

export function useRecoveryPoints(query: Accessor<RecoveryPointsQuery | null | undefined>) {
  const source = createMemo<string | null>(() => {
    const q = query();
    if (!q) return null;
    return serializeQuery(q);
  });

  const state = createNonSuspendingQuery<RecoveryPointsResponse, string>({
    source,
    cacheKey: (key) => `recovery-points:${key}`,
    fetcher: async (key) => fetchRecoveryPointsResponse(parseSerializedQuery(key)),
    initialValue: {
      data: [],
      meta: { page: 1, limit: DEFAULT_LIMIT, total: 0, totalPages: 1 },
    },
    pollMs: REFRESH_MS,
  });

  const points = createMemo<RecoveryPoint[]>(() => state.value().data || []);

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
    refetch: state.refetch,
  };
}
