import { Accessor, createMemo } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import type { RecoveryPointsFacets, RecoveryPointsFacetsResponse } from '@/types/recovery';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';

const RECOVERY_FACETS_URL = '/api/recovery/facets';
const REFRESH_MS = 30_000;

export type RecoveryFacetsQuery = {
  rollupId?: string | null;
  platform?: string | null;
  kind?: string | null;
  mode?: string | null;
  outcome?: string | null;
  itemType?: string | null;

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
    platform: norm(q.platform) || null,
    kind: norm(q.kind) || null,
    mode: norm(q.mode) || null,
    outcome: norm(q.outcome) || null,
    itemType: norm(q.itemType) || null,

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
  if (q.platform) params.set('platform', q.platform);
  if (q.kind) params.set('kind', q.kind);
  if (q.mode) params.set('mode', q.mode);
  if (q.outcome) params.set('outcome', q.outcome);
  if (q.itemType) params.set('itemType', q.itemType);

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

type RawRecoveryPointsFacets = RecoveryPointsFacets & {
  nodesHosts?: string[];
};

type RawRecoveryPointsFacetsResponse = {
  data: RawRecoveryPointsFacets;
};

const normalizeFacetValues = (values: unknown): string[] => {
  if (!Array.isArray(values)) return [];
  return values
    .map((value) => (typeof value === 'string' ? value.trim() : ''))
    .filter((value) => value.length > 0);
};

const normalizeFacets = (facets: RawRecoveryPointsFacets): RecoveryPointsFacets => ({
  ...facets,
  itemTypes: normalizeFacetValues(facets.itemTypes),
  nodesAgents: normalizeFacetValues(facets.nodesAgents ?? facets.nodesHosts),
});

async function fetchFacets(
  query: RecoveryFacetsQuery | undefined,
): Promise<RecoveryPointsFacetsResponse> {
  const url = buildURL(query);
  const response = await apiFetchJSON<RawRecoveryPointsFacetsResponse>(url);
  return {
    data: normalizeFacets(response.data || {}),
  };
}

export function useRecoveryPointsFacets(query?: Accessor<RecoveryFacetsQuery | null | undefined>) {
  const source = createMemo<string | null>(() => {
    if (!query) return '__all__';
    const q = query();
    if (!q) return null;
    return serializeQuery(q);
  });

  const state = createNonSuspendingQuery<RecoveryPointsFacetsResponse, string>({
    source,
    fetcher: async (key) => fetchFacets(parseSerializedQuery(key)),
    initialValue: { data: {} },
    pollMs: REFRESH_MS,
  });

  const facets = createMemo<RecoveryPointsFacets>(() => state.value().data || {});

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
    facets,
    refetch: state.refetch,
  };
}
