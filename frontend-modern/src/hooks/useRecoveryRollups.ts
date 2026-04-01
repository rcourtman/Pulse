import { createMemo, type Accessor } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import type { ProtectionRollup, RecoveryRollupsTransportResponse } from '@/types/recovery';
import { normalizeRecoveryRollupsResponse } from '@/utils/recoveryPlatformModel';
import { createNonSuspendingQuery } from '@/features/recovery/createNonSuspendingQuery';

const RECOVERY_ROLLUPS_URL = '/api/recovery/rollups';
const PAGE_LIMIT = 500;
const MAX_PAGES = 10; // hard cap to avoid pathological DB sizes causing huge client pulls
const REFRESH_MS = 30_000;

export type RecoveryRollupsQuery = {
  rollupId?: string | null;
  platform?: string | null;
  kind?: string | null;
  mode?: string | null;
  outcome?: string | null;
  itemType?: string | null;
  itemResourceId?: string | null;
  subjectResourceId?: string | null;
  q?: string | null;
  cluster?: string | null;
  node?: string | null;
  namespace?: string | null;
  scope?: 'workload' | null;
  verification?: 'verified' | 'unverified' | 'unknown' | null;
  from?: string | null;
  to?: string | null;
};

const normalizeQuery = (query: RecoveryRollupsQuery | undefined): RecoveryRollupsQuery => {
  const q = query || {};
  const norm = (value: string | null | undefined) => (value || '').trim();
  return {
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

const serializeQuery = (query: RecoveryRollupsQuery | undefined): string =>
  JSON.stringify(normalizeQuery(query));

const parseSerializedQuery = (value: string | null): RecoveryRollupsQuery | undefined => {
  if (value == null) return undefined;
  if (value === '__all__') return undefined;
  try {
    return JSON.parse(value) as RecoveryRollupsQuery;
  } catch {
    return undefined;
  }
};

const buildURL = (page: number, limit: number, query: RecoveryRollupsQuery | undefined): string => {
  const q = normalizeQuery(query);
  const params = new URLSearchParams();
  params.set('page', String(page));
  params.set('limit', String(limit));
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
  return `${RECOVERY_ROLLUPS_URL}?${params.toString()}`;
};

async function fetchRecoveryRollups(
  query: RecoveryRollupsQuery | undefined,
): Promise<ProtectionRollup[]> {
  const rollups: ProtectionRollup[] = [];

  for (let page = 1; page <= MAX_PAGES; page += 1) {
    const url = buildURL(page, PAGE_LIMIT, query);
    const response = await apiFetchJSON<RecoveryRollupsTransportResponse>(url);
    const resp = normalizeRecoveryRollupsResponse(response);
    rollups.push(...resp.data);
    if (!resp?.meta || (resp.data || []).length < PAGE_LIMIT || page >= resp.meta.totalPages) {
      break;
    }
  }

  return rollups;
}

export function useRecoveryRollups(query?: () => RecoveryRollupsQuery | null | undefined) {
  const source = createMemo<string | null>(() => {
    if (!query) return '__all__';
    const q = query();
    if (!q) return null;
    return serializeQuery(q);
  });

  const state = createNonSuspendingQuery<ProtectionRollup[], string>({
    source,
    fetcher: async (key) => fetchRecoveryRollups(parseSerializedQuery(key)),
    initialValue: [],
    pollMs: REFRESH_MS,
  });

  const rollups = (() => state.value()) as Accessor<ProtectionRollup[]> & {
    readonly error: unknown;
    readonly loading: boolean;
  };
  Object.defineProperties(rollups, {
    error: {
      get: () => state.error(),
    },
    loading: {
      get: () => state.loading(),
    },
  });

  return {
    rollups,
    refetch: state.refetch,
  };
}
