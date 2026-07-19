import { type Accessor, createMemo } from 'solid-js';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import type {
  ProtectionPosture,
  ProtectionPosturePolicy,
  ProtectionPosturesResponse,
} from '@/types/recovery';
import { apiFetchJSON } from '@/utils/apiClient';

const PROTECTION_POSTURES_URL = '/api/recovery/postures';
export const MAX_PROTECTION_POSTURE_BATCH_SIZE = 200;
const REFRESH_MS = 30_000;

const EMPTY_POLICY: ProtectionPosturePolicy = {
  freshnessWindowSeconds: 0,
  verificationWindowSeconds: 0,
  requireVerification: false,
};

const EMPTY_RESPONSE: ProtectionPosturesResponse = {
  data: [],
  policy: EMPTY_POLICY,
  meta: { page: 1, limit: MAX_PROTECTION_POSTURE_BATCH_SIZE, total: 0, totalPages: 0 },
};

export function normalizeProtectionPostureResourceIDs(
  resourceIDs: readonly string[] | null | undefined,
): string[] {
  return [
    ...new Set(
      (resourceIDs ?? []).map((resourceID) => resourceID.trim()).filter((resourceID) => resourceID),
    ),
  ].sort((left, right) => left.localeCompare(right));
}

export function buildProtectionPostureBatchURL(resourceIDs: readonly string[]): string {
  const normalized = normalizeProtectionPostureResourceIDs(resourceIDs);
  if (normalized.length > MAX_PROTECTION_POSTURE_BATCH_SIZE) {
    throw new Error(
      `Protection posture batches are limited to ${MAX_PROTECTION_POSTURE_BATCH_SIZE} resource IDs.`,
    );
  }
  const params = new URLSearchParams();
  for (const resourceID of normalized) {
    params.append('resourceId', resourceID);
  }
  params.set('limit', String(MAX_PROTECTION_POSTURE_BATCH_SIZE));
  return `${PROTECTION_POSTURES_URL}?${params.toString()}`;
}

async function fetchProtectionPostures(resourceIDs: readonly string[]) {
  return apiFetchJSON<ProtectionPosturesResponse>(buildProtectionPostureBatchURL(resourceIDs));
}

/**
 * Fetch server-owned protection posture in bounded batches. A fleet may exceed
 * one API batch, but request count scales by 200-row pages rather than by row.
 */
async function fetchAllProtectionPostures(
  resourceIDs: readonly string[],
): Promise<ProtectionPosturesResponse> {
  const batches: string[][] = [];
  for (let offset = 0; offset < resourceIDs.length; offset += MAX_PROTECTION_POSTURE_BATCH_SIZE) {
    batches.push(resourceIDs.slice(offset, offset + MAX_PROTECTION_POSTURE_BATCH_SIZE));
  }
  const responses = await Promise.all(batches.map(fetchProtectionPostures));
  const data = responses.flatMap((response) => response.data ?? []);
  return {
    data,
    policy: responses[0]?.policy ?? EMPTY_POLICY,
    meta: {
      page: 1,
      limit: MAX_PROTECTION_POSTURE_BATCH_SIZE,
      total: data.length,
      totalPages: responses.length,
    },
  };
}

export function useProtectionPostures(resourceIDs: Accessor<readonly string[] | null | undefined>) {
  const source = createMemo<string | null>(() => {
    const normalized = normalizeProtectionPostureResourceIDs(resourceIDs());
    return normalized.length > 0 ? JSON.stringify(normalized) : null;
  });

  const state = createNonSuspendingQuery<ProtectionPosturesResponse, string>({
    source,
    cacheKey: (key) => `protection-postures:${key}`,
    fetcher: (key) => fetchAllProtectionPostures(JSON.parse(key) as string[]),
    initialValue: EMPTY_RESPONSE,
    pollMs: REFRESH_MS,
  });

  const postures = createMemo<ProtectionPosture[]>(() => state.value().data ?? []);
  const postureByResourceID = createMemo<ReadonlyMap<string, ProtectionPosture>>(
    () => new Map(postures().map((posture) => [posture.subjectResourceId, posture] as const)),
  );
  const policy = createMemo(() => state.value().policy ?? EMPTY_POLICY);

  return {
    response: {
      get error() {
        return state.error();
      },
      get loading() {
        return state.loading();
      },
    },
    postures,
    postureByResourceID,
    policy,
    refetch: state.refetch,
    resolvedOnce: state.resolvedOnce,
  };
}
