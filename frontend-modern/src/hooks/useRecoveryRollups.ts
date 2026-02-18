import { createResource, onCleanup } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import type { ProtectionRollup, RecoveryRollupsResponse } from '@/types/recovery';

const RECOVERY_ROLLUPS_URL = '/api/recovery/rollups';
const PAGE_LIMIT = 500;
const MAX_PAGES = 10; // hard cap to avoid pathological DB sizes causing huge client pulls
const REFRESH_MS = 30_000;

export type RecoveryRollupsQuery = {
  rollupId?: string | null;
  provider?: string | null;
  kind?: string | null;
  mode?: string | null;
  outcome?: string | null;
  subjectResourceId?: string | null;
  from?: string | null;
  to?: string | null;
};

const normalizeQuery = (query: RecoveryRollupsQuery | undefined): RecoveryRollupsQuery => {
  const q = query || {};
  const norm = (value: string | null | undefined) => (value || '').trim();
  return {
    rollupId: norm(q.rollupId) || null,
    provider: norm(q.provider) || null,
    kind: norm(q.kind) || null,
    mode: norm(q.mode) || null,
    outcome: norm(q.outcome) || null,
    subjectResourceId: norm(q.subjectResourceId) || null,
    from: norm(q.from) || null,
    to: norm(q.to) || null,
  };
};

const serializeQuery = (query: RecoveryRollupsQuery | undefined): string => JSON.stringify(normalizeQuery(query));

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
  if (q.provider) params.set('provider', q.provider);
  if (q.kind) params.set('kind', q.kind);
  if (q.mode) params.set('mode', q.mode);
  if (q.outcome) params.set('outcome', q.outcome);
  if (q.subjectResourceId) params.set('subjectResourceId', q.subjectResourceId);
  if (q.from) params.set('from', q.from);
  if (q.to) params.set('to', q.to);
  return `${RECOVERY_ROLLUPS_URL}?${params.toString()}`;
};

async function fetchRecoveryRollups(query: RecoveryRollupsQuery | undefined): Promise<ProtectionRollup[]> {
  const rollups: ProtectionRollup[] = [];

  for (let page = 1; page <= MAX_PAGES; page += 1) {
    const url = buildURL(page, PAGE_LIMIT, query);
    const resp = await apiFetchJSON<RecoveryRollupsResponse>(url);
    rollups.push(...(resp?.data || []));
    if (!resp?.meta || (resp.data || []).length < PAGE_LIMIT || page >= resp.meta.totalPages) {
      break;
    }
  }

  return rollups;
}

export function useRecoveryRollups(query?: () => RecoveryRollupsQuery | null | undefined) {
  const [rollups, { refetch }] = createResource<ProtectionRollup[], string | null>(
    () => {
      if (!query) return '__all__';
      const q = query();
      if (!q) return null;
      return serializeQuery(q);
    },
    async (key) => fetchRecoveryRollups(parseSerializedQuery(key)),
  );

  const interval = setInterval(() => void refetch(), REFRESH_MS);
  onCleanup(() => clearInterval(interval));

  return {
    rollups,
    refetch,
  };
}
