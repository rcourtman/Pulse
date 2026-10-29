import { type Accessor, createMemo } from 'solid-js';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import type {
  ProtectionPosture,
  ProtectionPosturePolicy,
  ProtectionPosturesResponse,
} from '@/types/recovery';
import { apiFetchJSON } from '@/utils/apiClient';
import { parseProtectionPostureBatches } from '@/utils/protectionPostureResponse';

const PROTECTION_POSTURES_URL = '/api/recovery/postures';
export const MAX_PROTECTION_POSTURE_BATCH_SIZE = 200;
// Bound work per consuming surface as well as per request. A fleet-sized read
// must not enqueue every batch at once against the same server.
export const MAX_CONCURRENT_PROTECTION_POSTURE_REQUESTS = 4;
const REFRESH_MS = 30_000;

const EMPTY_POLICY: ProtectionPosturePolicy = {
  freshnessWindowSeconds: 0,
  verificationWindowSeconds: 0,
  requireVerification: false,
};

const EMPTY_RESPONSE: ProtectionPosturesResponse = {
  data: [],
  policy: EMPTY_POLICY,
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

async function fetchProtectionPostures(resourceIDs: readonly string[], signal: AbortSignal) {
  return apiFetchJSON<unknown>(buildProtectionPostureBatchURL(resourceIDs), { signal });
}

/**
 * Fetch server-owned protection posture in bounded batches. A fleet may exceed
 * one API batch, but request count scales by 200-row pages rather than by row.
 */
async function fetchAllProtectionPostures(
  resourceIDs: readonly string[],
  signal: AbortSignal,
): Promise<ProtectionPosturesResponse> {
  signal.throwIfAborted();
  const batches: string[][] = [];
  for (let offset = 0; offset < resourceIDs.length; offset += MAX_PROTECTION_POSTURE_BATCH_SIZE) {
    batches.push(resourceIDs.slice(offset, offset + MAX_PROTECTION_POSTURE_BATCH_SIZE));
  }
  // Every batch belongs to this one query read. Scope changes, replacement
  // reads and disposal cancel the transport too, not just its late result.
  // If one batch fails, the joined assessment is unusable: stop its siblings
  // rather than leaving them running after the query has already settled.
  const controller = new AbortController();
  const cancel = () => controller.abort(signal.reason);
  signal.addEventListener('abort', cancel, { once: true });
  const responses: unknown[] = new Array(batches.length);
  let nextBatch = 0;
  let failure: { error: unknown } | undefined;
  const readBatches = async () => {
    try {
      while (nextBatch < batches.length) {
        // A cancelled transport may still resolve. Never let that completion
        // dispatch a queued request for the retired scope/read.
        controller.signal.throwIfAborted();
        const index = nextBatch++;
        responses[index] = await fetchProtectionPostures(batches[index], controller.signal);
      }
    } catch (error) {
      // Stop queued work before siblings settle their aborts, and retain the
      // initiating failure rather than whichever abort rejection arrives first.
      failure ??= { error };
      controller.abort();
      throw failure.error;
    }
  };
  try {
    await Promise.all(
      Array.from(
        { length: Math.min(MAX_CONCURRENT_PROTECTION_POSTURE_REQUESTS, batches.length) },
        readBatches,
      ),
    );
    // Completion order never changes request order or publishes partial health.
    return parseProtectionPostureBatches(responses);
  } catch (error) {
    controller.abort();
    throw failure ? failure.error : error;
  } finally {
    signal.removeEventListener('abort', cancel);
  }
}

export function useProtectionPostures(resourceIDs: Accessor<readonly string[] | null | undefined>) {
  const source = createMemo<string | null>(() => {
    const normalized = normalizeProtectionPostureResourceIDs(resourceIDs());
    return normalized.length > 0 ? JSON.stringify(normalized) : null;
  });

  const state = createNonSuspendingQuery<ProtectionPosturesResponse, string>({
    source,
    cacheKey: (key) => `protection-postures:${key}`,
    fetcher: (key, signal) => fetchAllProtectionPostures(JSON.parse(key) as string[], signal),
    initialValue: EMPTY_RESPONSE,
    pollMs: REFRESH_MS,
  });

  // A retained assessment is useful while a refresh is pending, not after
  // that read fails. Withdraw the entire joined result (including its policy)
  // so rows, health counts and explanations cannot still claim protection
  // beside the unavailable notice. A successful full retry restores it.
  const acceptedResponse = createMemo(() => (state.error() ? EMPTY_RESPONSE : state.value()));
  const postures = createMemo<ProtectionPosture[]>(() => acceptedResponse().data ?? []);
  const postureByResourceID = createMemo<ReadonlyMap<string, ProtectionPosture>>(
    () => new Map(postures().map((posture) => [posture.subjectResourceId, posture] as const)),
  );
  const policy = createMemo(() => acceptedResponse().policy ?? EMPTY_POLICY);

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
