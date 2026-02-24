import { Accessor, createMemo, createResource, onCleanup } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import type { RecoveryPointsFacets, RecoveryPointsFacetsResponse } from '@/types/recovery';

const RECOVERY_FACETS_URL = '/api/recovery/facets';
const REFRESH_MS = 30_000;

export type RecoveryFacetsQuery = {
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
};

const normalizeQuery = (query: RecoveryFacetsQuery | undefined): RecoveryFacetsQuery => {
  const q = query || {};
  const norm = (value: string | null | undefined) => (value || '').trim();
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
  };
};

const serializeQuery = (query: RecoveryFacetsQuery | undefined): string =>
  JSON.stringify(normalizeQuery(query));

const parseSerializedQuery = (value: string | null): RecoveryFacetsQuery | undefined => {
  if (value == null) return undefined;
  if (value === '__all__') return undefined;
  try {
    return JSON.parse(value) as RecoveryFacetsQuery;
  } catch {
    return undefined;
  }
};

const buildURL = (query: RecoveryFacetsQuery | undefined): string => {
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

  return `${RECOVERY_FACETS_URL}?${params.toString()}`;
};

async function fetchFacets(
  query: RecoveryFacetsQuery | undefined,
): Promise<RecoveryPointsFacetsResponse> {
  const url = buildURL(query);
  return apiFetchJSON<RecoveryPointsFacetsResponse>(url);
}

export function useRecoveryPointsFacets(query?: Accessor<RecoveryFacetsQuery | null | undefined>) {
  const [response, { refetch }] = createResource<RecoveryPointsFacetsResponse, string | null>(
    () => {
      if (!query) return '__all__';
      const q = query();
      if (!q) return null;
      return serializeQuery(q);
    },
    async (key) => fetchFacets(parseSerializedQuery(key)),
  );

  const facets = createMemo<RecoveryPointsFacets>(() => response()?.data || {});

  const interval = setInterval(() => void refetch(), REFRESH_MS);
  onCleanup(() => clearInterval(interval));

  return {
    response,
    facets,
    refetch,
  };
}
